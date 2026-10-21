package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/mathh1s/hamazon/internal/db"
)

func TestAccessCookie(t *testing.T) {
	a := NewAccess("creeper", []byte("0123456789abcdef0123456789abcdef"))
	if a.CheckPassword("Creeper") || !a.CheckPassword("creeper") {
		t.Fatal("password check is wrong")
	}
	rec := httptest.NewRecorder()
	a.Grant(rec, false)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	if !a.Valid(req) {
		t.Fatal("fresh cookie must be valid")
	}
	rotated := NewAccess("new-password", []byte("0123456789abcdef0123456789abcdef"))
	if rotated.Valid(req) {
		t.Fatal("changing the password must invalidate old cookies")
	}
}

func TestAdminLoginAndSession(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatal(err)
	}
	admins := NewAdmins(conn)
	if err := admins.Create(ctx, "mathis", "short"); err != ErrWeakPassword {
		t.Fatalf("expected weak password error, got %v", err)
	}
	if err := admins.Create(ctx, "mathis", "a-long-password"); err != nil {
		t.Fatal(err)
	}
	if _, err := admins.Login(ctx, "mathis", "wrong-password"); err != ErrBadLogin {
		t.Fatalf("expected bad login, got %v", err)
	}
	token, err := admins.Login(ctx, "MATHIS", "a-long-password")
	if err != nil {
		t.Fatal(err)
	}
	adm, ok := admins.Session(ctx, token)
	if !ok || adm.Name != "mathis" || !adm.CheckCSRF(adm.CSRFToken) || adm.CheckCSRF("") {
		t.Fatalf("session lookup failed: %+v", adm)
	}
	if err := admins.SetPassword(ctx, "mathis", "another-long-password"); err != nil {
		t.Fatal(err)
	}
	if _, ok := admins.Session(ctx, token); ok {
		t.Fatal("changing the password must end existing sessions")
	}
}
