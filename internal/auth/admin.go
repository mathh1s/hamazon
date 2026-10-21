package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/mathh1s/hamazon/internal/db"
)

const (
	AdminCookie     = "hz_admin"
	sessionLifetime = 7 * 24 * time.Hour
	bcryptCost      = 12
	minPasswordLen  = 10
)

var (
	ErrBadLogin      = errors.New("wrong name or password")
	ErrAdminExists   = errors.New("an admin with this name already exists")
	ErrAdminNotFound = errors.New("admin not found")
	ErrWeakPassword  = errors.New("password must be at least 10 characters")
	ErrBadName       = errors.New("name must be 3 to 32 letters, numbers, dots, dashes or underscores")
)

var adminName = regexp.MustCompile(`^[A-Za-z0-9_.-]{3,32}$`)

type Admin struct {
	ID        int64
	Name      string
	CSRFToken string
}

type Admins struct {
	db *sql.DB
	// dummy is compared against for unknown names so a login attempt takes
	// the same time whether or not the admin exists.
	dummy []byte
}

func NewAdmins(conn *sql.DB) *Admins {
	dummy, _ := bcrypt.GenerateFromPassword([]byte("hamazon-dummy-password"), bcryptCost)
	return &Admins{db: conn, dummy: dummy}
}

func (a *Admins) Create(ctx context.Context, name, password string) error {
	if !adminName.MatchString(name) {
		return ErrBadName
	}
	if len(password) < minPasswordLen {
		return ErrWeakPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return err
	}
	_, err = a.db.ExecContext(ctx, `INSERT INTO admins (name, password_hash, created_at) VALUES (?, ?, ?)`, name, string(hash), db.Now())
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return ErrAdminExists
	}
	return err
}

// SetPassword also ends every session of that admin.
func (a *Admins) SetPassword(ctx context.Context, name, password string) error {
	if len(password) < minPasswordLen {
		return ErrWeakPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return err
	}
	return db.InTx(ctx, a.db, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE admins SET password_hash = ? WHERE name = ?`, string(hash), name)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrAdminNotFound
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM admin_sessions WHERE admin_id = (SELECT id FROM admins WHERE name = ?)`, name)
		return err
	})
}

func (a *Admins) Remove(ctx context.Context, name string) error {
	res, err := a.db.ExecContext(ctx, `DELETE FROM admins WHERE name = ?`, name)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrAdminNotFound
	}
	return nil
}

func (a *Admins) List(ctx context.Context) ([]string, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT name FROM admins ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// Login checks the credentials and opens a session. Only a hash of the
// session token is stored, the token itself lives in the admin's cookie.
func (a *Admins) Login(ctx context.Context, name, password string) (string, error) {
	var id int64
	var hash string
	err := a.db.QueryRowContext(ctx, `SELECT id, password_hash FROM admins WHERE name = ?`, name).Scan(&id, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword(a.dummy, []byte(password))
		return "", ErrBadLogin
	}
	if err != nil {
		return "", err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return "", ErrBadLogin
	}

	token := randomToken()
	now := time.Now()
	_, err = a.db.ExecContext(ctx, `INSERT INTO admin_sessions (token_hash, admin_id, csrf_token, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
		hashToken(token), id, randomToken(), db.FormatTime(now), db.FormatTime(now.Add(sessionLifetime)))
	if err != nil {
		return "", err
	}
	_, _ = a.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE expires_at < ?`, db.FormatTime(now))
	return token, nil
}

func (a *Admins) Session(ctx context.Context, token string) (Admin, bool) {
	if token == "" || len(token) > 128 {
		return Admin{}, false
	}
	var adm Admin
	err := a.db.QueryRowContext(ctx, `SELECT a.id, a.name, s.csrf_token FROM admin_sessions s JOIN admins a ON a.id = s.admin_id
		WHERE s.token_hash = ? AND s.expires_at > ?`, hashToken(token), db.Now()).Scan(&adm.ID, &adm.Name, &adm.CSRFToken)
	return adm, err == nil
}

func (a *Admins) Logout(ctx context.Context, token string) error {
	_, err := a.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE token_hash = ?`, hashToken(token))
	return err
}

func (adm Admin) CheckCSRF(header string) bool {
	return header != "" && subtle.ConstantTimeCompare([]byte(header), []byte(adm.CSRFToken)) == 1
}

func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
