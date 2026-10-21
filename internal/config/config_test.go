package config

import (
	"strings"
	"testing"
)

func TestPublicURLIsCleanedAndExplained(t *testing.T) {
	t.Setenv("HAMAZON_SHOP_PASSWORD", "x")
	t.Setenv("HAMAZON_SESSION_SECRET", strings.Repeat("a", 32))

	t.Setenv("HAMAZON_PUBLIC_URL", ` "https://shop.example.com/" `)
	c, err := Load()
	if err != nil || c.PublicURL != "https://shop.example.com" {
		t.Fatalf("quoted value should be accepted, got %q, %v", c.PublicURL, err)
	}

	t.Setenv("HAMAZON_PUBLIC_URL", "shop.example.com")
	_, err = Load()
	if err == nil || !strings.Contains(err.Error(), `"shop.example.com"`) {
		t.Fatalf("error should name the received value, got %v", err)
	}
}
