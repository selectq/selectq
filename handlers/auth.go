package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

// Auth keeps credentials and Google tokens on the server. Sessions are intentionally
// process-local: restarting the server signs everybody out.
type Auth struct {
	clientID, clientSecret, origin string
	emails, domains                map[string]bool
	client                         *http.Client
	mu                             sync.Mutex
	sessions                       map[string]authSession
	pending                        map[string]loginAttempt
}

type authUser struct {
	Subject  string `json:"sub"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Verified bool   `json:"email_verified"`
	Domain   string `json:"hd"`
}
type authSession struct {
	User    authUser
	Expires time.Time
}
type loginAttempt struct {
	Verifier string
	Expires  time.Time
}

func NewAuthFromEnv() (*Auth, error) {
	allowedEmails := os.Getenv("AUTH_ALLOWED_EMAILS")
	if _, set := os.LookupEnv("AUTH_ALLOWED_EMAILS"); !set {
		allowedEmails = "ritugandhi@selectq.in"
	}
	origin := strings.TrimRight(os.Getenv("APP_ORIGIN"), "/")
	if origin == "" {
		origin = "http://localhost:8080"
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" ||
		(u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))) {
		return nil, fmt.Errorf("APP_ORIGIN must be an HTTPS origin (HTTP is allowed only for localhost)")
	}
	return &Auth{clientID: os.Getenv("GOOGLE_CLIENT_ID"), clientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"), origin: origin,
		emails: authSet(allowedEmails), domains: authSet(os.Getenv("AUTH_ALLOWED_DOMAINS")),
		client: &http.Client{Timeout: 15 * time.Second}, sessions: make(map[string]authSession), pending: make(map[string]loginAttempt)}, nil
}

func authSet(value string) map[string]bool {
	result := make(map[string]bool)
	for _, part := range strings.Split(value, ",") {
		if s := strings.ToLower(strings.TrimSpace(part)); s != "" {
			result[s] = true
		}
	}
	return result
}
func (a *Auth) configured() bool {
	return a.clientID != "" && a.clientSecret != "" && len(a.emails)+len(a.domains) > 0
}
func authRandom() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func (a *Auth) cookie(w http.ResponseWriter, name, value string, age int, sameSite http.SameSite) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, Secure: strings.HasPrefix(a.origin, "https://"), SameSite: sameSite, MaxAge: age})
}
func (a *Auth) allowed(u authUser) bool {
	return u.Subject != "" && u.Email != "" && u.Verified && (a.emails[strings.ToLower(u.Email)] || (u.Domain != "" && a.domains[strings.ToLower(u.Domain)]))
}
func (a *Auth) user(r *http.Request) (authUser, bool) {
	c, err := r.Cookie("selectq_session")
	if err != nil || !a.configured() {
		return authUser{}, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	s, ok := a.sessions[c.Value]
	if !ok || time.Now().After(s.Expires) || !a.allowed(s.User) {
		delete(a.sessions, c.Value)
		return authUser{}, false
	}
	return s.User, true
}
func (a *Auth) Register(r *mux.Router) {
	r.HandleFunc("/auth/session", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		u, ok := a.user(r)
		json.NewEncoder(w).Encode(map[string]any{"configured": a.configured(), "authenticated": ok, "name": u.Name, "email": u.Email})
	}).Methods("GET")
	r.HandleFunc("/auth/google", a.login).Methods("GET")
	r.HandleFunc("/auth/google/callback", a.callback).Methods("GET")
	r.HandleFunc("/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != a.origin {
			http.Error(w, "Invalid origin", http.StatusForbidden)
			return
		}
		if c, err := r.Cookie("selectq_session"); err == nil {
			a.mu.Lock()
			delete(a.sessions, c.Value)
			a.mu.Unlock()
		}
		a.cookie(w, "selectq_session", "", -1, http.SameSiteLaxMode)
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}).Methods("POST")
}

// Require protects every accounting API, including direct downloads and writes.
func (a *Auth) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if _, ok := a.user(r); !ok {
			http.Error(w, "Sign in required", http.StatusUnauthorized)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && r.Header.Get("Origin") != a.origin {
			http.Error(w, "Invalid origin", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (a *Auth) login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !a.configured() {
		http.Redirect(w, r, "/login?error=setup", http.StatusSeeOther)
		return
	}
	state, verifier := authRandom(), authRandom()
	a.mu.Lock()
	for k, v := range a.pending {
		if time.Now().After(v.Expires) {
			delete(a.pending, k)
		}
	}
	for k, v := range a.sessions {
		if time.Now().After(v.Expires) {
			delete(a.sessions, k)
		}
	}
	if len(a.pending) >= 10000 {
		a.mu.Unlock()
		http.Error(w, "Please try again later", http.StatusTooManyRequests)
		return
	}
	a.pending[state] = loginAttempt{verifier, time.Now().Add(10 * time.Minute)}
	a.mu.Unlock()
	a.cookie(w, "selectq_oauth_state", state, 600, http.SameSiteLaxMode)
	challenge := sha256.Sum256([]byte(verifier))
	q := url.Values{"client_id": {a.clientID}, "redirect_uri": {a.origin + "/auth/google/callback"}, "response_type": {"code"},
		"scope": {"openid email profile"}, "state": {state}, "prompt": {"select_account"},
		"code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])}, "code_challenge_method": {"S256"}}
	http.Redirect(w, r, "https://accounts.google.com/o/oauth2/v2/auth?"+q.Encode(), http.StatusSeeOther)
}
func (a *Auth) callback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	fail := func(reason string) { http.Redirect(w, r, "/login?error="+reason, http.StatusSeeOther) }
	c, err := r.Cookie("selectq_oauth_state")
	state := r.URL.Query().Get("state")
	if !a.configured() || err != nil || state == "" || subtle.ConstantTimeCompare([]byte(c.Value), []byte(state)) != 1 {
		fail("state")
		return
	}
	a.cookie(w, "selectq_oauth_state", "", -1, http.SameSiteLaxMode)
	a.mu.Lock()
	attempt, ok := a.pending[state]
	delete(a.pending, state)
	a.mu.Unlock()
	if !ok || time.Now().After(attempt.Expires) {
		fail("state")
		return
	}
	if r.URL.Query().Get("error") != "" || r.URL.Query().Get("code") == "" {
		fail("cancelled")
		return
	}
	form := url.Values{"client_id": {a.clientID}, "client_secret": {a.clientSecret}, "code": {r.URL.Query().Get("code")},
		"code_verifier": {attempt.Verifier}, "redirect_uri": {a.origin + "/auth/google/callback"}, "grant_type": {"authorization_code"}}
	req, _ := http.NewRequestWithContext(r.Context(), "POST", "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := a.client.Do(req)
	if err != nil {
		fail("unavailable")
		return
	}
	var token struct {
		AccessToken string `json:"access_token"`
	}
	err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&token)
	res.Body.Close()
	if err != nil || res.StatusCode != http.StatusOK || token.AccessToken == "" {
		fail("unavailable")
		return
	}
	// Fetch identity directly from Google's TLS-protected UserInfo endpoint using
	// the access token obtained by this server's code exchange. No browser claims
	// or unverified JWT payloads are trusted.
	req, _ = http.NewRequestWithContext(r.Context(), "GET", "https://openidconnect.googleapis.com/v1/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	res, err = a.client.Do(req)
	if err != nil {
		fail("unavailable")
		return
	}
	var u authUser
	err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&u)
	res.Body.Close()
	if err != nil || res.StatusCode != http.StatusOK {
		fail("unavailable")
		return
	}
	if !a.allowed(u) {
		fail("denied")
		return
	}
	id := authRandom()
	a.mu.Lock()
	if old, err := r.Cookie("selectq_session"); err == nil {
		delete(a.sessions, old.Value)
	}
	a.sessions[id] = authSession{u, time.Now().Add(8 * time.Hour)}
	a.mu.Unlock()
	a.cookie(w, "selectq_session", id, 8*60*60, http.SameSiteLaxMode)
	http.Redirect(w, r, "/upload", http.StatusSeeOther)
}
