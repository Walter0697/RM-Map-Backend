package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	ExpireAt time.Time
}

var oidcStateStore sync.Map
var oidcUpsertUserFn = upsertUserAndGenerateToken
var oidcHTTPClient = &http.Client{Timeout: 10 * time.Second}

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

func OIDCStartHandler(w http.ResponseWriter, r *http.Request) {
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
	oidcStateStore.Store(state, oidcStateValue{ExpireAt: time.Now().Add(5 * time.Minute)})

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", config.Data.OIDC.ClientID)
	params.Set("redirect_uri", config.Data.OIDC.RedirectURL)
	params.Set("scope", strings.Join(config.Data.OIDC.Scopes, " "))
	params.Set("state", state)

	target := discovery.AuthorizationEndpoint + "?" + params.Encode()
	http.Redirect(w, r, target, http.StatusFound)
}

func OIDCCallbackHandler(w http.ResponseWriter, r *http.Request) {
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

	if !validateOIDCState(state) {
		http.Error(w, "invalid oidc state", http.StatusBadRequest)
		return
	}

	discovery, err := getOIDCDiscovery()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tokenResponse, err := exchangeOIDCCode(discovery.TokenEndpoint, code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	claims := map[string]interface{}{}
	if tokenResponse.IDToken != "" {
		if parsedClaims, parseErr := parseIDTokenClaims(tokenResponse.IDToken); parseErr == nil {
			claims = mergeClaims(claims, parsedClaims)
		}
	}
	if tokenResponse.AccessToken != "" && discovery.UserInfoEndpoint != "" {
		if parsedClaims, userInfoErr := fetchOIDCUserInfo(discovery.UserInfoEndpoint, tokenResponse.AccessToken); userInfoErr == nil {
			claims = mergeClaims(claims, parsedClaims)
		}
	}

	username := pickOIDCUsername(claims)
	if strings.TrimSpace(username) == "" {
		http.Error(w, "unable to resolve username from oidc claims", http.StatusBadGateway)
		return
	}

	jwtToken, err := oidcUpsertUserFn(username, config.Data.OIDC.DefaultRole)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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

func GenerateOIDCState() string {
	return fmt.Sprintf("%s%d", strings.ToLower(config.Data.OIDC.ClientID), time.Now().UnixNano())
}

func validateOIDCState(state string) bool {
	raw, ok := oidcStateStore.Load(state)
	if !ok {
		return false
	}
	oidcStateStore.Delete(state)

	stateValue := raw.(oidcStateValue)
	return stateValue.ExpireAt.After(time.Now())
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
	body, err := GetRequest(discoveryURL)
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

func exchangeOIDCCode(tokenEndpoint string, code string) (*oidcTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", config.Data.OIDC.ClientID)
	form.Set("client_secret", config.Data.OIDC.ClientSecret)
	form.Set("redirect_uri", config.Data.OIDC.RedirectURL)

	request, err := http.NewRequest(http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := oidcHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
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

	response, err := oidcHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
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

func parseIDTokenClaims(idToken string) (map[string]interface{}, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid id token format")
	}

	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	claims := map[string]interface{}{}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil, err
	}

	return claims, nil
}

func mergeClaims(base map[string]interface{}, update map[string]interface{}) map[string]interface{} {
	for key, value := range update {
		base[key] = value
	}
	return base
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
