package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cleanAuthConfig(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	for _, key := range []string{"AUTH_CONFIG_FILE", "APP_ORIGIN", "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "AUTH_ALLOWED_EMAILS", "AUTH_ALLOWED_DOMAINS"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
}

func writeAuthConfig(t *testing.T, value string) {
	t.Helper()
	if err := os.WriteFile("auth.config.json", []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestAuthJSONConfig(t *testing.T) {
	cleanAuthConfig(t)
	writeAuthConfig(t, `{"app_origin":"https://studio.example","google_client_id":"file-id","google_client_secret":"file-secret","allowed_emails":["owner@example.com"],"allowed_domains":[]}`)
	a, err := NewAuthFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !a.configured() || a.origin != "https://studio.example" || a.clientID != "file-id" || a.clientSecret != "file-secret" || !a.emails["owner@example.com"] || a.emails["ritugandhi@selectq.in"] {
		t.Fatal("JSON configuration not applied")
	}
	t.Setenv("GOOGLE_CLIENT_SECRET", "environment-secret")
	t.Setenv("AUTH_ALLOWED_EMAILS", "")
	a, err = NewAuthFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if a.clientSecret != "environment-secret" || a.configured() {
		t.Fatal("environment overrides, including empty allowlist, must take precedence")
	}
}

func TestAuthJSONDefaultsAndPaths(t *testing.T) {
	cleanAuthConfig(t)
	cfg, err := loadAuthConfig()
	if err != nil || cfg.Origin != "http://localhost:8080" || len(cfg.AllowedEmails) != 1 {
		t.Fatal("missing optional file should use defaults")
	}
	writeAuthConfig(t, "\xef\xbb\xbf"+`{"allowed_emails":[]}`)
	cfg, err = loadAuthConfig()
	if err != nil || len(cfg.AllowedEmails) != 0 {
		t.Fatal("BOM and explicit empty allowlist must be supported")
	}
	custom := filepath.Join(t.TempDir(), "custom.json")
	if err := os.WriteFile(custom, []byte(`{"google_client_id":"custom-id"}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AUTH_CONFIG_FILE", custom)
	cfg, err = loadAuthConfig()
	if err != nil || cfg.ClientID != "custom-id" {
		t.Fatal("explicit config path not loaded")
	}
	t.Setenv("AUTH_CONFIG_FILE", custom+".missing")
	if _, err := loadAuthConfig(); err == nil {
		t.Fatal("missing explicit file must stop startup")
	}
}

func TestAuthJSONRejectsInvalidInput(t *testing.T) {
	for _, value := range []string{"", "null", "[]", "{", `{"unknown-secret-key":"secret"}`, `{"allowed_emails":"secret"}`, `{} {}`, `{} trailing-secret`} {
		t.Run(value, func(t *testing.T) {
			cleanAuthConfig(t)
			writeAuthConfig(t, value)
			_, err := loadAuthConfig()
			if err == nil {
				t.Fatal("invalid config accepted")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatal("configuration content leaked in error")
			}
		})
	}
}
