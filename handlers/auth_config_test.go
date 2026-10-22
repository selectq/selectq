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
	for _, key := range []string{"AUTH_CONFIG_FILE", "APP_ORIGIN", "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "AUTH_ALLOWED_EMAILS", "AUTH_ALLOWED_DOMAINS", "TLS_CERT_FILE", "TLS_KEY_FILE"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
}

func TestServerConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name, config, address string
		tls, wantError        bool
	}{
		{"default", `{}`, ":8080", false, false},
		{"http", `{"tls_enabled":false}`, ":8080", false, false},
		{"tls", `{"tls_enabled":true,"tls_cert_file":"cert.pem","tls_key_file":"key.pem"}`, ":443", true, false},
		{"custom port", `{"tls_enabled":true,"tls_cert_file":"cert.pem","tls_key_file":"key.pem","listen_address":":8443"}`, ":8443", true, false},
		{"missing certificate", `{"tls_enabled":true,"tls_key_file":"key.pem"}`, "", true, true},
		{"missing key", `{"tls_enabled":true,"tls_cert_file":"cert.pem"}`, "", true, true},
		{"wrong flag type", `{"tls_enabled":"true"}`, "", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cleanAuthConfig(t)
			writeAuthConfig(t, tc.config)
			cfg, err := LoadServerConfig()
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected configuration error: %v", err)
			}
			if err == nil && (cfg.ListenAddress != tc.address || cfg.TLSEnabled != tc.tls) {
				t.Fatalf("unexpected listener configuration: %+v", cfg)
			}
		})
	}
}

func TestServerTLSCredentialEnvironment(t *testing.T) {
	cleanAuthConfig(t)
	writeAuthConfig(t, `{"tls_enabled":true,"tls_cert_file":"file-cert","tls_key_file":"file-key"}`)
	t.Setenv("TLS_CERT_FILE", "service-cert")
	t.Setenv("TLS_KEY_FILE", "service-key")
	cfg, err := LoadServerConfig()
	if err != nil || cfg.TLSCertFile != "service-cert" || cfg.TLSKeyFile != "service-key" {
		t.Fatalf("TLS environment overrides not applied: %v", err)
	}
	if _, err := NewAuthFromEnv(); err != nil {
		t.Fatalf("authentication must accept TLS configuration: %v", err)
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
