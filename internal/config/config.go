package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr          string
	DBPath            string
	MasterKey         string
	DiscordWebhookURL string
	SecureCookies     bool
	BrowserURL        string
	LogLevel          string

	// OIDC single-sign-on (all optional; feature enabled only when issuer,
	// client id, client secret, and redirect URL are all set).
	OIDCIssuer        string
	OIDCClientID      string
	OIDCClientSecret  string
	OIDCRedirectURL   string
	OIDCProviderName  string
	OIDCAllowedEmails []string
	OIDCAllowedGroups []string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:          getenv("GRUB_HTTP_ADDR", "0.0.0.0:8080"),
		DBPath:            getenv("GRUB_DB_PATH", "/data/miner.db"),
		MasterKey:         os.Getenv("GRUB_MASTER_KEY"),
		DiscordWebhookURL: os.Getenv("GRUB_DISCORD_WEBHOOK"),
		SecureCookies:     parseBool(os.Getenv("GRUB_SECURE_COOKIES")),
		BrowserURL:        os.Getenv("GRUB_BROWSER_URL"),
		LogLevel:          strings.ToLower(getenv("GRUB_LOG_LEVEL", "info")),
	}
	cfg.OIDCIssuer = os.Getenv("GRUB_OIDC_ISSUER")
	cfg.OIDCClientID = os.Getenv("GRUB_OIDC_CLIENT_ID")
	cfg.OIDCClientSecret = os.Getenv("GRUB_OIDC_CLIENT_SECRET")
	cfg.OIDCRedirectURL = os.Getenv("GRUB_OIDC_REDIRECT_URL")
	cfg.OIDCProviderName = getenv("GRUB_OIDC_PROVIDER_NAME", "SSO")
	cfg.OIDCAllowedEmails = splitList(os.Getenv("GRUB_OIDC_ALLOWED_EMAILS"))
	cfg.OIDCAllowedGroups = splitList(os.Getenv("GRUB_OIDC_ALLOWED_GROUPS"))
	if strings.TrimSpace(cfg.MasterKey) == "" {
		return Config{}, fmt.Errorf("GRUB_MASTER_KEY is required")
	}
	return cfg, nil
}

func parseBool(s string) bool {
	if s == "" {
		return false
	}
	b, _ := strconv.ParseBool(s)
	return b
}

func getenv(k, d string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return d
}

// OIDCEnabled reports whether all mandatory OIDC settings are present.
func (c Config) OIDCEnabled() bool {
	return c.OIDCIssuer != "" && c.OIDCClientID != "" &&
		c.OIDCClientSecret != "" && c.OIDCRedirectURL != ""
}

// splitList parses a comma-separated env value into a trimmed, non-empty slice.
func splitList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
