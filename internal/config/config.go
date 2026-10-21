package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Addr            string
	DataDir         string
	ShopPassword    string
	SessionSecret   []byte
	WebhookURL      string
	PublicURL       string
	TrustProxy      bool
	FloodgatePrefix string
	SeedFile        string
}

func (c Config) DatabasePath() string { return filepath.Join(c.DataDir, "hamazon.db") }
func (c Config) AssetsDir() string    { return filepath.Join(c.DataDir, "assets") }

// Load reads the environment. Only the server needs the secrets, so CLI
// subcommands call LoadBase and skip that validation.
func Load() (Config, error) {
	c := LoadBase()
	var errs []error

	c.ShopPassword = os.Getenv("HAMAZON_SHOP_PASSWORD")
	if c.ShopPassword == "" {
		errs = append(errs, errors.New("HAMAZON_SHOP_PASSWORD is required"))
	}

	secret := os.Getenv("HAMAZON_SESSION_SECRET")
	if len(secret) < 32 {
		errs = append(errs, errors.New("HAMAZON_SESSION_SECRET must be at least 32 characters"))
	}
	c.SessionSecret = []byte(secret)

	c.WebhookURL = strings.TrimSpace(os.Getenv("HAMAZON_DISCORD_WEBHOOK_URL"))
	if c.WebhookURL != "" {
		if err := validateWebhookURL(c.WebhookURL); err != nil {
			errs = append(errs, err)
		}
	}

	c.PublicURL = strings.TrimRight(os.Getenv("HAMAZON_PUBLIC_URL"), "/")
	if c.PublicURL != "" {
		if u, err := url.Parse(c.PublicURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			errs = append(errs, errors.New("HAMAZON_PUBLIC_URL must be an absolute http(s) URL"))
		}
	}

	if v := os.Getenv("HAMAZON_TRUST_PROXY"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			errs = append(errs, fmt.Errorf("HAMAZON_TRUST_PROXY: %w", err))
		}
		c.TrustProxy = b
	}

	return c, errors.Join(errs...)
}

func LoadBase() Config {
	c := Config{
		Addr:            envOr("HAMAZON_ADDR", ":8080"),
		DataDir:         envOr("HAMAZON_DATA_DIR", "/data"),
		FloodgatePrefix: envOr("HAMAZON_FLOODGATE_PREFIX", "."),
		SeedFile:        os.Getenv("HAMAZON_SEED_FILE"),
	}
	return c
}

// Only Discord hosts are accepted so a typo or a tampered .env cannot turn
// the notifier into a request forwarder to arbitrary hosts.
func validateWebhookURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("HAMAZON_DISCORD_WEBHOOK_URL: %w", err)
	}
	host := strings.ToLower(u.Hostname())
	okHost := host == "discord.com" || host == "discordapp.com" || host == "ptb.discord.com" || host == "canary.discord.com"
	if u.Scheme != "https" || !okHost || !strings.HasPrefix(u.Path, "/api/webhooks/") {
		return errors.New("HAMAZON_DISCORD_WEBHOOK_URL must be a https://discord.com/api/webhooks/... URL")
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
