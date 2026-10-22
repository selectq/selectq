package handlers

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func testAuth(t *testing.T) *Auth {
	t.Helper()
	cleanAuthConfig(t)
	t.Setenv("APP_ORIGIN", "https://studio.example")
	t.Setenv("GOOGLE_CLIENT_ID", "test-client")
	t.Setenv("GOOGLE_CLIENT_SECRET", "test-secret")
	t.Setenv("AUTH_ALLOWED_EMAILS", "ritugandhi@selectq.in")
	t.Setenv("AUTH_ALLOWED_DOMAINS", "")
	a, err := NewAuthFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func approvedUser() authUser {
	return authUser{Subject: "google-id", Email: "ritugandhi@selectq.in", Name: "Ritu", Verified: true}
}

func TestAuthAPIBoundary(t *testing.T) {
	a := testAuth(t)
	a.sessions["valid"] = authSession{approvedUser(), time.Now().Add(time.Hour)}
	a.sessions["expired"] = authSession{approvedUser(), time.Now().Add(-time.Hour)}
	r := mux.NewRouter()
	a.Register(r)
	api := r.PathPrefix("/api").Subrouter()
	api.Use(a.Require)
	api.HandleFunc("/private", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }).Methods("GET", "POST")
	for _, tc := range []struct {
		name, method, session, origin string
		status                        int
	}{
		{"anonymous download", "GET", "", "", 401}, {"forged session", "GET", "fake", "", 401},
		{"expired session", "GET", "expired", "", 401}, {"authorized read", "GET", "valid", "", 204},
		{"cross origin write", "POST", "valid", "https://evil.example", 403},
		{"missing origin write", "POST", "valid", "", 403}, {"authorized write", "POST", "valid", a.origin, 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/api/private", nil)
			req.AddCookie(&http.Cookie{Name: "selectq_session", Value: tc.session})
			req.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("got %d want %d", w.Code, tc.status)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("private responses must not be cached")
			}
		})
	}
	a.clientSecret = ""
	req := httptest.NewRequest("GET", "/api/private", nil)
	req.AddCookie(&http.Cookie{Name: "selectq_session", Value: "valid"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatal("unconfigured auth must fail closed")
	}
}

func TestAuthAllowlist(t *testing.T) {
	a := testAuth(t)
	for _, tc := range []struct {
		name    string
		user    authUser
		allowed bool
	}{
		{"approved", approvedUser(), true},
		{"unverified", authUser{Subject: "id", Email: "ritugandhi@selectq.in"}, false},
		{"other account", authUser{Subject: "id", Email: "other@selectq.in", Verified: true}, false},
		{"missing subject", authUser{Email: "ritugandhi@selectq.in", Verified: true}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if a.allowed(tc.user) != tc.allowed {
				t.Fatal("incorrect account authorization")
			}
		})
	}
	a.domains = authSet("selectq.in")
	if a.allowed(authUser{Subject: "id", Email: "other@selectq.in", Verified: true}) {
		t.Fatal("email suffix is not a verified Workspace domain")
	}
	if !a.allowed(authUser{Subject: "id", Email: "other@selectq.in", Verified: true, Domain: "selectq.in"}) {
		t.Fatal("verified Workspace domain should work when explicitly allowed")
	}
}

type authTransport func(*http.Request) (*http.Response, error)

func (f authTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAuthGoogleFlow(t *testing.T) {
	for _, scenario := range []string{"success", "denied", "unverified", "provider failure"} {
		t.Run(scenario, func(t *testing.T) {
			a := testAuth(t)
			start := httptest.NewRecorder()
			a.login(start, httptest.NewRequest("GET", "/auth/google", nil))
			location, _ := url.Parse(start.Header().Get("Location"))
			q := location.Query()
			if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" || q.Get("redirect_uri") != a.origin+"/auth/google/callback" {
				t.Fatal("missing PKCE or callback URI")
			}
			cookie := start.Result().Cookies()[0]
			if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
				t.Fatal("insecure OAuth cookie")
			}
			state := q.Get("state")
			verifier := a.pending[state].Verifier
			calls := 0
			a.client = &http.Client{Transport: authTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				body := ""
				status := 200
				switch r.URL.String() {
				case "https://oauth2.googleapis.com/token":
					r.ParseForm()
					if r.Form.Get("code_verifier") != verifier || r.Form.Get("client_secret") != "test-secret" || r.Form.Get("code") != "test-code" {
						t.Fatal("bad token exchange")
					}
					body = `{"access_token":"server-only-token"}`
					if scenario == "provider failure" {
						status = 500
					}
				case "https://openidconnect.googleapis.com/v1/userinfo":
					if r.Header.Get("Authorization") != "Bearer server-only-token" {
						t.Fatal("missing bearer token")
					}
					u := approvedUser()
					if scenario == "denied" {
						u.Email = "stranger@example.com"
					}
					if scenario == "unverified" {
						u.Verified = false
					}
					b, _ := json.Marshal(u)
					body = string(b)
				default:
					t.Fatalf("unexpected endpoint: %s", r.URL)
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			req := httptest.NewRequest("GET", "/auth/google/callback?state="+state+"&code=test-code", nil)
			req.AddCookie(cookie)
			result := httptest.NewRecorder()
			a.callback(result, req)
			if scenario == "success" {
				if result.Header().Get("Location") != "/upload" {
					t.Fatal("login failed")
				}
				var session *http.Cookie
				for _, c := range result.Result().Cookies() {
					if c.Name == "selectq_session" {
						session = c
					}
				}
				if session == nil || !session.HttpOnly || !session.Secure || session.MaxAge != 28800 {
					t.Fatal("missing secure session")
				}
				check := httptest.NewRequest("GET", "/api/private", nil)
				check.AddCookie(session)
				if _, ok := a.user(check); !ok {
					t.Fatal("session invalid")
				}
				router := mux.NewRouter()
				a.Register(router)
				logout := httptest.NewRequest("POST", "/auth/logout", nil)
				logout.Header.Set("Origin", a.origin)
				logout.AddCookie(session)
				out := httptest.NewRecorder()
				router.ServeHTTP(out, logout)
				if out.Code != 303 {
					t.Fatal("logout failed")
				}
				if _, ok := a.user(check); ok {
					t.Fatal("logout did not revoke session")
				}
			} else if len(a.sessions) != 0 || !strings.HasPrefix(result.Header().Get("Location"), "/login?error=") {
				t.Fatal("failed login created access")
			}
			before := calls
			replay := httptest.NewRecorder()
			a.callback(replay, req)
			if replay.Header().Get("Location") != "/login?error=state" || calls != before {
				t.Fatal("OAuth state replay accepted")
			}
		})
	}
}

func TestAuthRejectsInvalidState(t *testing.T) {
	a := testAuth(t)
	a.pending["expired"] = loginAttempt{"verifier", time.Now().Add(-time.Minute)}
	for _, state := range []string{"missing", "expired"} {
		req := httptest.NewRequest("GET", "/auth/google/callback?state="+state+"&code=unused", nil)
		req.AddCookie(&http.Cookie{Name: "selectq_oauth_state", Value: state})
		w := httptest.NewRecorder()
		a.callback(w, req)
		if w.Header().Get("Location") != "/login?error=state" {
			t.Fatal("invalid state accepted")
		}
	}
	req := httptest.NewRequest("GET", "/auth/google/callback?state=forged&code=unused", nil)
	req.AddCookie(&http.Cookie{Name: "selectq_oauth_state", Value: "different"})
	w := httptest.NewRecorder()
	a.callback(w, req)
	if w.Header().Get("Location") != "/login?error=state" {
		t.Fatal("mismatched cookie accepted")
	}
}

func TestAuthConfiguration(t *testing.T) {
	a := testAuth(t)
	for _, origin := range []string{"ftp://public.example", "https://studio.example/path", "https://user:secret@studio.example", "https://studio.example?x=1"} {
		t.Setenv("APP_ORIGIN", origin)
		if _, err := NewAuthFromEnv(); err == nil {
			t.Fatalf("accepted invalid origin %s", origin)
		}
	}
	a.clientID = ""
	r := mux.NewRouter()
	a.Register(r)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/auth/session", nil))
	if strings.Contains(w.Body.String(), "test-secret") || !strings.Contains(w.Body.String(), `"configured":false`) {
		t.Fatal("unsafe session response")
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/auth/google", nil))
	if w.Header().Get("Location") != "/login?error=setup" {
		t.Fatal("unconfigured login should show setup state")
	}
}

func TestAuthPublicHTTPOrigin(t *testing.T) {
	testAuth(t)
	t.Setenv("APP_ORIGIN", "http://completedesign.in:8080")
	a, err := NewAuthFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	a.login(w, httptest.NewRequest("GET", "/auth/google", nil))
	location, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if location.Query().Get("redirect_uri") != "http://completedesign.in:8080/auth/google/callback" {
		t.Fatal("incorrect HTTP callback URL")
	}
	cookies := w.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Secure || !cookies[0].HttpOnly {
		t.Fatal("HTTP login requires an HttpOnly cookie without Secure")
	}
}
