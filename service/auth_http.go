package service

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/coreos/go-oidc/v3/oidc"
	"io"
	"io/ioutil"
	"log"
	"mapmarker/backend/config"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type oidcDiscoveryDocument struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserInfoEndpoint      string `json:"userinfo_endpoint"`
}

type oidcTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
}

type oidcStateValue struct {
	ExpireAt      time.Time
	Nonce         string
	Verifier      string
	BrowserSecret string
	MCPRequestID  string
}

var oidcStateStore sync.Map
var oidcResolveUserFn = resolveVerifiedOIDCUser
var oidcIssueTokenFn = issueTokenForUser
var oidcHTTPClient = &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}

func AuthModeHandler(w http.ResponseWriter, _ *http.Request) {
	mode, err := config.ResolveAuthMode()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"mode":  "",
			"error": err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"mode":                 mode,
		"environment":          config.Data.App.Environment,
		"localPasswordAllowed": mode == config.AuthModeLocalPassword,
	})
}

func AuthHealthHandler(w http.ResponseWriter, _ *http.Request) {
	mode, err := config.ResolveAuthMode()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	alignment := config.AuthLifetimeAlignmentStatus()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":               "ok",
		"mode":                 mode,
		"environment":          config.Data.App.Environment,
		"localPasswordAllowed": mode == config.AuthModeLocalPassword,
		"oidcEnabled":          config.Data.OIDC.Enable,
		"sessionLifetime": map[string]interface{}{
			"configured": alignment.ConfiguredSessionTTLSeconds,
			"token":      config.ResolveAuthTokenLifetimeSeconds(),
			"oidc": map[string]interface{}{
				"sessionTTL":      alignment.OIDCSessionTTLSeconds,
				"accessTokenTTL":  alignment.OIDCAccessTokenTTLSeconds,
				"refreshTokenTTL": alignment.OIDCRefreshTokenTTLSeconds,
			},
			"alignment": map[string]interface{}{
				"authStateAligned": alignment.AuthStateAligned,
				"oidcAligned":      alignment.OIDCAligned,
			},
		},
		"authState": map[string]interface{}{
			"migrationMode": config.Data.AuthState.MigrationMode,
			"redisEnabled":  config.Data.Redis.Enable,
			"sessionTTL":    config.Data.AuthState.SessionTTLSeconds,
			"metrics":       AuthStateMetrics(),
		},
	})
}

func OIDCStartHandler(w http.ResponseWriter, r *http.Request) { startOIDCLogin(w, r, "") }

// StartMCPOIDCLogin starts a fresh provider authentication for an OAuth consent request.
func StartMCPOIDCLogin(w http.ResponseWriter, r *http.Request, requestID string) {
	startOIDCLogin(w, r, requestID)
}

func startOIDCLogin(w http.ResponseWriter, r *http.Request, requestID string) {
	w.Header().Set("Cache-Control", "no-store")
	if err := validateOIDCIssuer(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	oidcStateStore.Range(func(key, value interface{}) bool {
		if state, ok := value.(oidcStateValue); !ok || !state.ExpireAt.After(time.Now()) {
			oidcStateStore.Delete(key)
		}
		return true
	})
	mode, err := config.ResolveAuthMode()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if mode != config.AuthModeOIDC {
		http.Error(w, "oidc login is disabled in current auth mode", http.StatusConflict)
		return
	}

	discovery, err := getOIDCDiscovery()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	state := GenerateOIDCState()
	nonce, verifier, browserSecret := GenerateOIDCState(), GenerateOIDCState(), GenerateOIDCState()
	oidcStateStore.Store(state, oidcStateValue{ExpireAt: time.Now().Add(5 * time.Minute), Nonce: nonce, Verifier: verifier, BrowserSecret: browserSecret, MCPRequestID: requestID})
	http.SetCookie(w, &http.Cookie{Name: "oidc_login_state", Value: browserSecret, Path: "/auth/oidc", HttpOnly: true, Secure: !config.IsLocalEnvironment(config.Data.App.Environment), SameSite: http.SameSiteLaxMode, MaxAge: 300})

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", config.Data.OIDC.ClientID)
	params.Set("redirect_uri", config.Data.OIDC.RedirectURL)
	params.Set("scope", strings.Join(config.Data.OIDC.Scopes, " "))
	params.Set("state", state)
	params.Set("nonce", nonce)
	challenge := sha256.Sum256([]byte(verifier))
	params.Set("code_challenge", base64.RawURLEncoding.EncodeToString(challenge[:]))
	params.Set("code_challenge_method", "S256")

	if err := validateOIDCTransportURL(discovery.AuthorizationEndpoint); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	target := discovery.AuthorizationEndpoint + "?" + params.Encode()
	http.Redirect(w, r, target, http.StatusFound)
}

func OIDCCallbackHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if err := validateOIDCIssuer(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	mode, err := config.ResolveAuthMode()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if mode != config.AuthModeOIDC {
		http.Error(w, "oidc login is disabled in current auth mode", http.StatusConflict)
		return
	}

	query := r.URL.Query()
	code := query.Get("code")
	state := query.Get("state")
	if code == "" || state == "" {
		http.Error(w, "missing code or state", http.StatusBadRequest)
		return
	}

	loginState, valid := consumeOIDCState(state)
	cookie, cookieErr := r.Cookie("oidc_login_state")
	if !valid || cookieErr != nil || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(loginState.BrowserSecret)) != 1 {
		http.Error(w, "invalid oidc state", http.StatusBadRequest)
		return
	}

	discovery, err := getOIDCDiscovery()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{Name: "oidc_login_state", Path: "/auth/oidc", MaxAge: -1, HttpOnly: true, Secure: !config.IsLocalEnvironment(config.Data.App.Environment), SameSite: http.SameSiteLaxMode})
	tokenResponse, err := exchangeOIDCCode(discovery.TokenEndpoint, code, loginState.Verifier)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	ctx := oidc.ClientContext(r.Context(), safeOIDCHTTPClient())
	claims, err := verifiedOIDCClaims(ctx, tokenResponse.IDToken, loginState.Nonce)
	if err != nil {
		http.Error(w, "oidc identity verification failed", http.StatusUnauthorized)
		return
	}
	subject, _ := claims["sub"].(string)
	user, err := oidcResolveUserFn(config.Data.OIDC.Issuer, subject, pickOIDCUsername(claims))
	if err != nil {
		http.Error(w, "OIDC identity is not linked to an active account; contact your administrator", http.StatusForbidden)
		return
	}
	if loginState.MCPRequestID != "" {
		jwtToken, err := oidcIssueTokenFn(user)
		if err != nil {
			http.Error(w, "unable to create session", http.StatusInternalServerError)
			return
		}
		setMCPBrowserSessionCookie(w, jwtToken)
		completeMCPOIDCLogin(w, r, user, loginState.MCPRequestID)
		return
	}
	jwtToken, err := oidcIssueTokenFn(user)
	if err != nil {
		http.Error(w, "unable to create session", http.StatusInternalServerError)
		return
	}
	setMCPBrowserSessionCookie(w, jwtToken)
	username := user.Username

	redirectURL := config.Data.OIDC.FrontendRedirectURL
	if strings.TrimSpace(redirectURL) == "" {
		redirectURL = strings.TrimSuffix(config.Data.App.AllowedOrigin, "/") + "/login"
	}

	nextURL, err := url.Parse(redirectURL)
	if err != nil {
		http.Error(w, "invalid oidc frontend redirect url", http.StatusInternalServerError)
		return
	}
	values := nextURL.Query()
	values.Set("token", jwtToken)
	values.Set("username", username)
	nextURL.RawQuery = values.Encode()

	http.Redirect(w, r, nextURL.String(), http.StatusFound)
}

func setMCPBrowserSessionCookie(w http.ResponseWriter, jwtToken string) {
	http.SetCookie(w, &http.Cookie{
		Name:     mcpBrowserSessionCookie,
		Value:    jwtToken,
		Path:     "/",
		Secure:   !config.IsLocalEnvironment(config.Data.App.Environment),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   config.Data.AuthState.SessionTTLSeconds,
	})
}

func GenerateOIDCState() string { return newOIDCSecureToken() }

func consumeOIDCState(state string) (oidcStateValue, bool) {
	raw, ok := oidcStateStore.LoadAndDelete(state)
	if !ok {
		return oidcStateValue{}, false
	}
	value, ok := raw.(oidcStateValue)
	return value, ok && value.ExpireAt.After(time.Now()) && value.BrowserSecret != "" && value.Nonce != "" && value.Verifier != ""
}

func verifiedOIDCClaims(ctx context.Context, raw, nonce string) (map[string]interface{}, error) {
	if raw == "" || nonce == "" {
		return nil, fmt.Errorf("missing ID token or nonce")
	}
	provider, err := oidc.NewProvider(ctx, config.Data.OIDC.Issuer)
	if err != nil {
		return nil, err
	}
	token, err := provider.Verifier(&oidc.Config{ClientID: config.Data.OIDC.ClientID}).Verify(ctx, raw)
	if err != nil {
		return nil, err
	}
	if token.Subject == "" || subtle.ConstantTimeCompare([]byte(token.Nonce), []byte(nonce)) != 1 {
		return nil, fmt.Errorf("invalid subject or nonce")
	}
	claims := map[string]interface{}{}
	if err := token.Claims(&claims); err != nil {
		return nil, err
	}
	azp, _ := claims["azp"].(string)
	if (len(token.Audience) > 1 && azp != config.Data.OIDC.ClientID) || (azp != "" && azp != config.Data.OIDC.ClientID) {
		return nil, fmt.Errorf("invalid authorized party")
	}
	return claims, nil
}

func validateOIDCIssuer() error {
	u, err := url.Parse(config.Data.OIDC.Issuer)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("invalid OIDC issuer")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && config.IsLocalEnvironment(config.Data.App.Environment)) {
		return fmt.Errorf("OIDC issuer requires HTTPS")
	}
	return nil
}

func getOIDCDiscovery() (*oidcDiscoveryDocument, error) {
	discovery := &oidcDiscoveryDocument{
		AuthorizationEndpoint: config.Data.OIDC.AuthEndpoint,
		TokenEndpoint:         config.Data.OIDC.TokenEndpoint,
		UserInfoEndpoint:      config.Data.OIDC.UserInfoEndpoint,
	}

	if discovery.AuthorizationEndpoint != "" && discovery.TokenEndpoint != "" {
		return discovery, nil
	}

	issuer := strings.TrimSuffix(config.Data.OIDC.Issuer, "/")
	discoveryURL := issuer + "/.well-known/openid-configuration"
	request, err := http.NewRequest(http.MethodGet, discoveryURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := safeOIDCHTTPClient().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OIDC discovery failed")
	}
	body, err := ioutil.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(body, discovery); err != nil {
		return nil, err
	}
	if discovery.AuthorizationEndpoint == "" || discovery.TokenEndpoint == "" {
		return nil, fmt.Errorf("oidc discovery is missing required endpoints")
	}

	return discovery, nil
}

func exchangeOIDCCode(tokenEndpoint string, code string, verifier string) (*oidcTokenResponse, error) {
	if err := validateOIDCTransportURL(tokenEndpoint); err != nil {
		return nil, err
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("client_id", config.Data.OIDC.ClientID)
	form.Set("client_secret", config.Data.OIDC.ClientSecret)
	form.Set("redirect_uri", config.Data.OIDC.RedirectURL)

	request, err := http.NewRequest(http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := safeOIDCHTTPClient().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("oidc token exchange failed with status %d", response.StatusCode)
	}

	tokenResponse := &oidcTokenResponse{}
	if err := json.Unmarshal(body, tokenResponse); err != nil {
		return nil, err
	}
	if tokenResponse.AccessToken == "" && tokenResponse.IDToken == "" {
		return nil, fmt.Errorf("oidc token response is missing access token/id token")
	}

	return tokenResponse, nil
}

func fetchOIDCUserInfo(userInfoEndpoint string, accessToken string) (map[string]interface{}, error) {
	request, err := http.NewRequest(http.MethodGet, userInfoEndpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)

	response, err := safeOIDCHTTPClient().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("oidc userinfo failed with status %d", response.StatusCode)
	}

	claims := map[string]interface{}{}
	if err := json.Unmarshal(body, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func pickOIDCUsername(claims map[string]interface{}) string {
	keys := []string{
		config.Data.OIDC.UsernameClaim,
		"preferred_username",
		"email",
		"sub",
	}

	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		raw, exists := claims[key]
		if !exists {
			continue
		}
		value := fmt.Sprintf("%v", raw)
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}

	return ""
}

func respondJSON(w http.ResponseWriter, statusCode int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("failed to write json response: %v", err)
	}
}

// Protect discovery/JWKS and exchanges against transport downgrade. Endpoint
// overrides may change locations, but must not send codes or secrets over HTTP.
type oidcSecureTransport struct{ base http.RoundTripper }

func (t oidcSecureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if err := validateOIDCTransportURL(r.URL.String()); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(r)
}
func validateOIDCTransportURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" {
		return fmt.Errorf("invalid OIDC endpoint")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && config.IsLocalEnvironment(config.Data.App.Environment)) {
		return fmt.Errorf("OIDC endpoints require HTTPS")
	}
	return nil
}
func safeOIDCHTTPClient() *http.Client {
	client := *oidcHTTPClient
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	client.Transport = oidcSecureTransport{base: transport}
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	return &client
}
