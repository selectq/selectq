package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type authConfig struct {
	ServerConfig
	Origin         string   `json:"app_origin"`
	ClientID       string   `json:"google_client_id"`
	ClientSecret   string   `json:"google_client_secret"`
	AllowedEmails  []string `json:"allowed_emails"`
	AllowedDomains []string `json:"allowed_domains"`
}

// ServerConfig controls the application's own listener, independently of a proxy.
type ServerConfig struct {
	TLSEnabled    bool   `json:"tls_enabled"`
	TLSCertFile   string `json:"tls_cert_file"`
	TLSKeyFile    string `json:"tls_key_file"`
	ListenAddress string `json:"listen_address"`
}

// LoadServerConfig reads listener settings from the same file as authentication.
func LoadServerConfig() (ServerConfig, error) {
	cfg, err := loadAuthConfig()
	if err != nil {
		return ServerConfig{}, err
	}
	server := cfg.ServerConfig
	if server.ListenAddress == "" {
		server.ListenAddress = ":8080"
		if server.TLSEnabled {
			server.ListenAddress = ":443"
		}
	}
	if server.TLSEnabled && (strings.TrimSpace(server.TLSCertFile) == "" || strings.TrimSpace(server.TLSKeyFile) == "") {
		return ServerConfig{}, fmt.Errorf("tls_cert_file and tls_key_file are required when tls_enabled is true")
	}
	return server, nil
}

// Configuration is read at server startup, never embedded in browser assets or
// the executable. Explicit environment variables override individual JSON values.
func loadAuthConfig() (authConfig, error) {
	cfg := authConfig{Origin: "http://localhost:8080", AllowedEmails: []string{"ritugandhi@selectq.in"}}
	path := os.Getenv("AUTH_CONFIG_FILE")
	explicit := path != ""
	if !explicit {
		path = "auth.config.json"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if explicit || !os.IsNotExist(err) {
			return cfg, fmt.Errorf("cannot read auth configuration file: %w", err)
		}
	} else {
		// PowerShell editors may write UTF-8 with a BOM.
		data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
		if len(bytes.TrimSpace(data)) == 0 || bytes.TrimSpace(data)[0] != '{' {
			return cfg, fmt.Errorf("auth configuration must be a JSON object")
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&cfg); err != nil {
			// Do not include decoder errors: unknown keys can contain secret text.
			return cfg, fmt.Errorf("invalid auth configuration JSON: check field names and value types")
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return cfg, fmt.Errorf("auth configuration must contain exactly one JSON object")
		}
	}
	for key, target := range map[string]*string{
		"APP_ORIGIN": &cfg.Origin, "GOOGLE_CLIENT_ID": &cfg.ClientID, "GOOGLE_CLIENT_SECRET": &cfg.ClientSecret,
		"TLS_CERT_FILE": &cfg.TLSCertFile, "TLS_KEY_FILE": &cfg.TLSKeyFile,
	} {
		if value, set := os.LookupEnv(key); set {
			*target = value
		}
	}
	if value, set := os.LookupEnv("AUTH_ALLOWED_EMAILS"); set {
		cfg.AllowedEmails = strings.Split(value, ",")
	}
	if value, set := os.LookupEnv("AUTH_ALLOWED_DOMAINS"); set {
		cfg.AllowedDomains = strings.Split(value, ",")
	}
	return cfg, nil
}
