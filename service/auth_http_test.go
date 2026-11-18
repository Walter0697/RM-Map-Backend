package service

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"github.com/golang-jwt/jwt"
	"mapmarker/backend/config"
	"mapmarker/backend/database/dbmodel"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOIDCVerifiedCallback(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	original := config.Data
	oldClient := oidcHTTPClient
	oldResolve := oidcResolveUserFn
	oldIssue := oidcIssueTokenFn
	defer func() {
		config.Data = original
		oidcHTTPClient = oldClient
		oidcResolveUserFn = oldResolve
		oidcIssueTokenFn = oldIssue
	}()
	var issuer, rawToken, receivedVerifier string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			json.NewEncoder(w).Encode(map[string]interface{}{"issuer": issuer, "authorization_endpoint": issuer + "/auth", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/keys":
			json.NewEncoder(w).Encode(map[string]interface{}{"keys": []interface{}{map[string]interface{}{"kty": "RSA", "kid": "test", "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}})
		case "/token":
			r.ParseForm()
			receivedVerifier = r.Form.Get("code_verifier")
			json.NewEncoder(w).Encode(map[string]string{"id_token": rawToken})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	issuer = srv.URL
	oidcHTTPClient = srv.Client()
	config.Data.App.Environment = "test"
	config.Data.App.AuthMode = config.AuthModeOIDC
	config.Data.OIDC.Enable = true
	config.Data.OIDC.Issuer = issuer
	config.Data.OIDC.ClientID = "client"
	config.Data.OIDC.RedirectURL = "http://localhost/auth/oidc/callback"
	config.Data.OIDC.FrontendRedirectURL = "http://localhost/login"
	config.Data.OIDC.AuthEndpoint = issuer + "/auth"
	config.Data.OIDC.TokenEndpoint = issuer + "/token"
	config.Data.OIDC.Scopes = []string{"openid"}
	called := false
	oidcResolveUserFn = func(iss, sub, name string) (*dbmodel.User, error) {
		called = true
		if iss != issuer || sub != "subject" {
			t.Fatal("bad verified identity")
		}
		return &dbmodel.User{Username: "existing-user", IsActivated: true}, nil
	}
	oidcIssueTokenFn = func(*dbmodel.User) (string, error) { return "session", nil }
	for _, environment := range []string{"production", "test"} {
		config.Data.App.Environment = environment
		for _, tc := range []string{"valid", "bad-signature", "wrong-audience", "wrong-azp", "missing-azp", "wrong-issuer", "expired", "wrong-nonce", "no-cookie"} {
			t.Run(environment+"/"+tc, func(t *testing.T) {
				called = false
				start := httptest.NewRecorder()
				OIDCStartHandler(start, httptest.NewRequest("GET", "/auth/oidc/start", nil))
				if start.Code != 302 {
					t.Fatalf("start: %d", start.Code)
				}
				cookies := start.Result().Cookies()
				if len(cookies) != 1 {
					t.Fatalf("expected login cookie, got %d", len(cookies))
				}
				loginCookie := cookies[0]
				wantSecure := environment == "production"
				if loginCookie.Name != "oidc_login_state" || loginCookie.Secure != wantSecure || !loginCookie.HttpOnly || loginCookie.SameSite != http.SameSiteLaxMode || loginCookie.Path != "/auth/oidc" || loginCookie.MaxAge != 300 || loginCookie.Value == "" {
					t.Fatalf("incorrect %s login cookie flags: %#v", environment, loginCookie)
				}
				location, _ := url.Parse(start.Header().Get("Location"))
				q := location.Query()
				if q.Get("code_challenge_method") != "S256" || q.Get("nonce") == "" {
					t.Fatal("missing nonce/PKCE")
				}
				claims := jwt.MapClaims{"iss": issuer, "sub": "subject", "aud": "client", "exp": time.Now().Add(time.Minute).Unix(), "iat": time.Now().Unix(), "nonce": q.Get("nonce")}
				switch tc {
				case "wrong-audience":
					claims["aud"] = "other"
				case "wrong-azp":
					claims["azp"] = "other"
				case "missing-azp":
					claims["aud"] = []string{"client", "other"}
				case "wrong-issuer":
					claims["iss"] = "https://other.example"
				case "expired":
					claims["exp"] = time.Now().Add(-time.Minute).Unix()
				case "wrong-nonce":
					claims["nonce"] = "other"
				}
				token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
				token.Header["kid"] = "test"
				signingKey := key
				if tc == "bad-signature" {
					signingKey, _ = rsa.GenerateKey(rand.Reader, 2048)
				}
				rawToken, _ = token.SignedString(signingKey)
				req := httptest.NewRequest("GET", "/auth/oidc/callback?code=code&state="+q.Get("state"), nil)
				if tc != "no-cookie" {
					req.AddCookie(start.Result().Cookies()[0])
				}
				result := httptest.NewRecorder()
				OIDCCallbackHandler(result, req)
				if tc != "no-cookie" {
					cleared := result.Result().Cookies()
					foundCleared := false
					for _, cookie := range cleared {
						if cookie.Name == loginCookie.Name {
							if cookie.Secure != wantSecure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != loginCookie.Path || cookie.MaxAge != -1 || cookie.Value != "" {
								t.Fatalf("incorrect %s clearing cookie flags: %#v", environment, cookie)
							}
							foundCleared = true
						}
					}
					if !foundCleared {
						t.Fatalf("expected clearing cookie, got %d", len(cleared))
					}
				}
				if tc == "valid" {
					if result.Code != 302 || !called || receivedVerifier == "" || !strings.Contains(result.Header().Get("Location"), "username=existing-user") {
						t.Fatalf("valid callback failed %d", result.Code)
					}
					foundSession := false
					for _, cookie := range result.Result().Cookies() {
						if cookie.Name == mcpBrowserSessionCookie && cookie.Value == "session" && cookie.HttpOnly && cookie.SameSite == http.SameSiteLaxMode && cookie.Path == "/" {
							foundSession = true
						}
					}
					if !foundSession {
						t.Fatal("OIDC login did not establish the reusable browser session")
					}
				} else if result.Code == 302 || called {
					t.Fatalf("accepted invalid identity %s", tc)
				}
				replay := httptest.NewRecorder()
				OIDCCallbackHandler(replay, req)
				if replay.Code != 400 {
					t.Fatal("state replay accepted")
				}
			})
		}
	}
}
