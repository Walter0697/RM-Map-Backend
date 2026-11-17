package service

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const mcpBrowserCookie = "__Host-rm-mcp-consent"

var mcpScopes = []string{"markers:read", "schedules:read", "stations:read", "settings:read", "travel-plans:read", "schedules:write", "travel-plans:write", "static-preview:generate"}

func MCPEnabled() bool { return strings.EqualFold(os.Getenv("MCP_ENABLED"), "true") }
func mcpOrigin() (string, error) {
	raw := strings.TrimSuffix(strings.TrimSpace(os.Getenv("MCP_PUBLIC_ORIGIN")), "/")
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return "", errors.New("MCP_PUBLIC_ORIGIN must be an HTTPS origin")
	}
	return raw, nil
}
func mcpRandom() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("secure random unavailable")
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func mcpHash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func mcpEqualHash(raw, hash string) bool {
	return subtle.ConstantTimeCompare([]byte(mcpHash(raw)), []byte(hash)) == 1
}
func mcpJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	respondJSON(w, status, v)
}
func mcpError(w http.ResponseWriter, status int, code string) {
	mcpJSON(w, status, map[string]string{"error": code})
}
func mcpReady(w http.ResponseWriter) (string, bool) {
	o, e := mcpOrigin()
	if !MCPEnabled() || e != nil {
		mcpError(w, 503, "temporarily_unavailable")
		return "", false
	}
	return o, true
}
func validMCPRedirect(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Host == "" || u.User != nil || u.Fragment != "" || len(raw) > 2048 {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	ip := net.ParseIP(u.Hostname())
	return u.Scheme == "http" && ip != nil && ip.IsLoopback()
}
func normalizeMCPScope(raw string) (string, error) {
	if raw == "" {
		raw = "markers:read schedules:read stations:read settings:read travel-plans:read"
	}
	allowed := map[string]bool{}
	for _, s := range mcpScopes {
		allowed[s] = true
	}
	seen := map[string]bool{}
	for _, s := range strings.Fields(raw) {
		if !allowed[s] {
			return "", errors.New("invalid scope")
		}
		seen[s] = true
	}
	out := []string{}
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return strings.Join(out, " "), nil
}
func MCPResourceMetadata(w http.ResponseWriter, r *http.Request) {
	o, ok := mcpReady(w)
	if !ok {
		return
	}
	mcpJSON(w, 200, map[string]interface{}{"resource": o + "/mcp", "authorization_servers": []string{o}, "scopes_supported": mcpScopes, "bearer_methods_supported": []string{"header"}})
}
func MCPAuthorizationMetadata(w http.ResponseWriter, r *http.Request) {
	o, ok := mcpReady(w)
	if !ok {
		return
	}
	mcpJSON(w, 200, map[string]interface{}{"issuer": o, "authorization_endpoint": o + "/mcp/oauth/authorize", "token_endpoint": o + "/mcp/oauth/token", "registration_endpoint": o + "/mcp/oauth/register", "revocation_endpoint": o + "/mcp/oauth/revoke", "response_types_supported": []string{"code"}, "response_modes_supported": []string{"query"}, "grant_types_supported": []string{"authorization_code", "refresh_token"}, "token_endpoint_auth_methods_supported": []string{"none"}, "revocation_endpoint_auth_methods_supported": []string{"none"}, "code_challenge_methods_supported": []string{"S256"}, "scopes_supported": mcpScopes, "authorization_response_iss_parameter_supported": true})
}
func MCPRegister(w http.ResponseWriter, r *http.Request) {
	if !mcpRateLimit(w, r, "register", 10) {
		return
	}
	if _, ok := mcpReady(w); !ok {
		return
	}
	var in struct {
		Name      string   `json:"client_name"`
		Redirects []string `json:"redirect_uris"`
		Method    string   `json:"token_endpoint_auth_method"`
		Grants    []string `json:"grant_types"`
		Responses []string `json:"response_types"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	d := json.NewDecoder(r.Body)
	if d.Decode(&in) != nil || len(in.Redirects) == 0 || len(in.Redirects) > 10 || len(in.Name) > 120 || (in.Method != "" && in.Method != "none") {
		mcpError(w, 400, "invalid_client_metadata")
		return
	}
	for _, v := range in.Redirects {
		if !validMCPRedirect(v) {
			mcpError(w, 400, "invalid_redirect_uri")
			return
		}
	}
	for _, v := range in.Grants {
		if v != "authorization_code" && v != "refresh_token" {
			mcpError(w, 400, "invalid_client_metadata")
			return
		}
	}
	for _, v := range in.Responses {
		if v != "code" {
			mcpError(w, 400, "invalid_client_metadata")
			return
		}
	}
	var count int64
	if database.Connection.Model(&dbmodel.MCPOAuthClient{}).Count(&count).Error != nil || count >= 1000 {
		mcpError(w, 503, "temporarily_unavailable")
		return
	}
	b, _ := json.Marshal(in.Redirects)
	c := dbmodel.MCPOAuthClient{ID: mcpRandom(), Name: in.Name, RedirectURIs: string(b), CreatedAt: time.Now()}
	if database.Connection.Create(&c).Error != nil {
		mcpError(w, 500, "server_error")
		return
	}
	mcpJSON(w, 201, map[string]interface{}{"client_id": c.ID, "client_name": c.Name, "redirect_uris": in.Redirects, "token_endpoint_auth_method": "none", "grant_types": []string{"authorization_code", "refresh_token"}, "response_types": []string{"code"}})
}
func MCPAuthorize(w http.ResponseWriter, r *http.Request) {
	if !mcpRateLimit(w, r, "authorize", 60) {
		return
	}
	o, ok := mcpReady(w)
	if !ok {
		return
	}
	q := r.URL.Query()
	if len(r.URL.RawQuery) > 8192 || q.Get("response_type") != "code" || q.Get("resource") != o+"/mcp" || len(q.Get("state")) > 1024 || q.Get("code_challenge_method") != "S256" {
		mcpError(w, 400, "invalid_request")
		return
	}
	challenge := q.Get("code_challenge")
	decoded, e := base64.RawURLEncoding.DecodeString(challenge)
	if e != nil || len(decoded) != 32 {
		mcpError(w, 400, "invalid_request")
		return
	}
	var c dbmodel.MCPOAuthClient
	if database.Connection.First(&c, "id = ?", q.Get("client_id")).Error != nil {
		mcpError(w, 400, "invalid_client")
		return
	}
	var redirects []string
	_ = json.Unmarshal([]byte(c.RedirectURIs), &redirects)
	matched := false
	for _, v := range redirects {
		if v == q.Get("redirect_uri") {
			matched = true
		}
	}
	if !matched {
		mcpError(w, 400, "invalid_redirect_uri")
		return
	}
	scope, e := normalizeMCPScope(q.Get("scope"))
	if e != nil {
		mcpError(w, 400, "invalid_scope")
		return
	}
	// Request records contain no long-lived credentials and are no longer useful after expiry.
	if database.Connection.Where("expires_at < ?", time.Now().Add(-24*time.Hour)).Delete(&dbmodel.MCPOAuthRequest{}).Error != nil {
		mcpError(w, 503, "temporarily_unavailable")
		return
	}
	browser := mcpRandom()
	request := dbmodel.MCPOAuthRequest{ID: mcpRandom(), ClientID: c.ID, RedirectURI: q.Get("redirect_uri"), Resource: o + "/mcp", State: q.Get("state"), Scope: scope, Challenge: challenge, BrowserHash: mcpHash(browser), CodeHash: mcpHash(mcpRandom()), ExpiresAt: time.Now().Add(10 * time.Minute)}
	if database.Connection.Create(&request).Error != nil {
		mcpError(w, 500, "server_error")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: mcpBrowserCookie, Value: browser, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 600})
	StartMCPOIDCLogin(w, r, request.ID)
}
func mcpBoundRequest(r *http.Request, id string) (dbmodel.MCPOAuthRequest, error) {
	var req dbmodel.MCPOAuthRequest
	c, e := r.Cookie(mcpBrowserCookie)
	if e != nil {
		return req, e
	}
	e = database.Connection.First(&req, "id = ? AND used = ? AND expires_at > ?", id, false, time.Now()).Error
	if e != nil || !mcpEqualHash(c.Value, req.BrowserHash) {
		return req, errors.New("invalid authorization request")
	}
	return req, nil
}

var mcpConsentTemplate = template.Must(template.New("consent").Parse(`<!doctype html><html><head><meta charset="utf-8"><title>Connect Marker Map</title></head><body><h1>Connect {{.Client}}</h1><p>Signed in as {{.Username}}. This application requests the following access:</p><ul>{{range .Scopes}}<li>{{.}}</li>{{end}}</ul><form method="post" action="/mcp/oauth/consent"><input type="hidden" name="request_id" value="{{.ID}}"><input type="hidden" name="csrf" value="{{.CSRF}}"><label>Shared map <select name="relation_id" required>{{range .Relations}}<option value="{{.ID}}">{{.UserOne.Username}} / {{.UserTwo.Username}} ({{.ID}})</option>{{end}}</select></label><p>Only maps you currently belong to are available. Existing role and testing-data permissions still apply.</p><button name="decision" value="allow">Allow access</button><button name="decision" value="deny">Cancel</button></form></body></html>`))

func completeMCPOIDCLogin(w http.ResponseWriter, r *http.Request, user *dbmodel.User, requestID string) {
	if _, ok := mcpReady(w); !ok {
		return
	}
	req, e := mcpBoundRequest(r, requestID)
	if e != nil || user == nil || !user.IsActivated {
		mcpError(w, 403, "access_denied")
		return
	}
	var relations []dbmodel.UserRelation
	if database.Connection.Where("user_one_uid = ? OR user_two_uid = ?", user.ID, user.ID).Preload("UserOne").Preload("UserTwo").Find(&relations).Error != nil || len(relations) == 0 {
		mcpError(w, 403, "access_denied")
		return
	}
	csrf := mcpRandom()
	if database.Connection.Model(&req).Updates(map[string]interface{}{"user_id": user.ID, "csrf_hash": mcpHash(csrf)}).Error != nil {
		mcpError(w, 500, "server_error")
		return
	}
	var c dbmodel.MCPOAuthClient
	if database.Connection.First(&c, "id = ?", req.ClientID).Error != nil {
		mcpError(w, 400, "invalid_client")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	_ = mcpConsentTemplate.Execute(w, map[string]interface{}{"Client": c.Name, "Username": user.Username, "Scopes": strings.Fields(req.Scope), "ID": req.ID, "CSRF": csrf, "Relations": relations})
}
func MCPConsent(w http.ResponseWriter, r *http.Request) {
	o, ok := mcpReady(w)
	if !ok {
		return
	}
	if r.Header.Get("Origin") != "" && r.Header.Get("Origin") != o {
		mcpError(w, 403, "access_denied")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if r.ParseForm() != nil {
		mcpError(w, 400, "invalid_request")
		return
	}
	req, e := mcpBoundRequest(r, r.PostForm.Get("request_id"))
	if e != nil || req.UserID == 0 || req.CSRFHash == "" || !mcpEqualHash(r.PostForm.Get("csrf"), req.CSRFHash) {
		mcpError(w, 403, "access_denied")
		return
	}
	target, _ := url.Parse(req.RedirectURI)
	q := target.Query()
	q.Set("state", req.State)
	q.Set("iss", o)
	if r.PostForm.Get("decision") != "allow" {
		database.Connection.Model(&req).Update("used", true)
		q.Set("error", "access_denied")
		target.RawQuery = q.Encode()
		http.Redirect(w, r, target.String(), 303)
		return
	}
	relationID, e := strconv.ParseUint(r.PostForm.Get("relation_id"), 10, 32)
	if e != nil {
		mcpError(w, 400, "invalid_request")
		return
	}
	if _, _, e = mcpCurrentActor(database.Connection, req.UserID, uint(relationID)); e != nil {
		mcpError(w, 403, "access_denied")
		return
	}
	code := mcpRandom()
	result := database.Connection.Model(&dbmodel.MCPOAuthRequest{}).Where("id = ? AND used = ? AND csrf_hash = ? AND expires_at > ?", req.ID, false, req.CSRFHash, time.Now()).Updates(map[string]interface{}{"code_hash": mcpHash(code), "csrf_hash": "", "relation_id": uint(relationID), "expires_at": time.Now().Add(time.Minute)})
	if result.Error != nil || result.RowsAffected != 1 {
		mcpError(w, 400, "invalid_request")
		return
	}
	q.Set("code", code)
	target.RawQuery = q.Encode()
	http.Redirect(w, r, target.String(), 303)
}
func mcpCurrentActor(db *gorm.DB, userID, relationID uint) (dbmodel.User, dbmodel.UserRelation, error) {
	var u dbmodel.User
	var rel dbmodel.UserRelation
	if e := db.First(&u, "id = ? AND is_activated = ?", userID, true).Error; e != nil {
		return u, rel, e
	}
	e := db.First(&rel, "id = ? AND (user_one_uid = ? OR user_two_uid = ?)", relationID, userID, userID).Error
	return u, rel, e
}
func mcpPKCE(verifier, challenge string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	for _, c := range verifier {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-._~", c)) {
			return false
		}
	}
	h := sha256.Sum256([]byte(verifier))
	return subtle.ConstantTimeCompare([]byte(base64.RawURLEncoding.EncodeToString(h[:])), []byte(challenge)) == 1
}
func mcpIssueTokens(db *gorm.DB, g dbmodel.MCPOAuthGrant) (map[string]interface{}, error) {
	access, refresh := mcpRandom(), mcpRandom()
	now := time.Now()
	for _, t := range []dbmodel.MCPOAuthToken{{Hash: mcpHash(access), GrantID: g.ID, Kind: "access", ExpiresAt: now.Add(10 * time.Minute)}, {Hash: mcpHash(refresh), GrantID: g.ID, Kind: "refresh", ExpiresAt: g.ExpiresAt}} {
		if e := db.Create(&t).Error; e != nil {
			return nil, e
		}
	}
	return map[string]interface{}{"access_token": access, "token_type": "Bearer", "expires_in": 600, "refresh_token": refresh, "scope": g.Scope}, nil
}
func MCPToken(w http.ResponseWriter, r *http.Request) {
	o, ok := mcpReady(w)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if r.ParseForm() != nil {
		mcpError(w, 400, "invalid_request")
		return
	}
	f := r.PostForm
	if f.Get("resource") != o+"/mcp" || f.Get("client_id") == "" {
		mcpError(w, 400, "invalid_target")
		return
	}
	var result map[string]interface{}
	replayed := false
	err := database.Connection.Transaction(func(tx *gorm.DB) error {
		switch f.Get("grant_type") {
		case "authorization_code":
			var req dbmodel.MCPOAuthRequest
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&req, "code_hash = ?", mcpHash(f.Get("code"))).Error; e != nil {
				return e
			}
			if req.Used || !req.ExpiresAt.After(time.Now()) || req.ClientID != f.Get("client_id") || req.RedirectURI != f.Get("redirect_uri") || req.Resource != f.Get("resource") || req.RelationID == 0 || req.CSRFHash != "" || !mcpPKCE(f.Get("code_verifier"), req.Challenge) {
				return errors.New("invalid grant")
			}
			if _, _, e := mcpCurrentActor(tx, req.UserID, req.RelationID); e != nil {
				return e
			}
			if e := tx.Model(&req).Update("used", true).Error; e != nil {
				return e
			}
			g := dbmodel.MCPOAuthGrant{ID: mcpRandom(), ClientID: req.ClientID, UserID: req.UserID, RelationID: req.RelationID, Scope: req.Scope, Resource: req.Resource, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(30 * 24 * time.Hour)}
			if e := tx.Create(&g).Error; e != nil {
				return e
			}
			var e error
			result, e = mcpIssueTokens(tx, g)
			return e
		case "refresh_token":
			var token dbmodel.MCPOAuthToken
			if e := tx.First(&token, "hash = ? AND kind = ?", mcpHash(f.Get("refresh_token")), "refresh").Error; e != nil {
				return e
			}
			var g dbmodel.MCPOAuthGrant
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&g, "id = ?", token.GrantID).Error; e != nil {
				return e
			}
			// Re-read after taking the family lock: concurrent use must be replay, not two rotations.
			if e := tx.First(&token, "hash = ?", token.Hash).Error; e != nil {
				return e
			}
			if g.ClientID != f.Get("client_id") || g.Resource != f.Get("resource") {
				return errors.New("invalid grant")
			}
			if token.Used {
				replayed = true
				return tx.Model(&g).Update("revoked", true).Error
			}
			if g.Revoked || !g.ExpiresAt.After(time.Now()) || !token.ExpiresAt.After(time.Now()) {
				return errors.New("invalid grant")
			}
			if _, _, e := mcpCurrentActor(tx, g.UserID, g.RelationID); e != nil {
				return e
			}
			if e := tx.Model(&token).Update("used", true).Error; e != nil {
				return e
			}
			var e error
			result, e = mcpIssueTokens(tx, g)
			return e
		default:
			return errors.New("unsupported grant")
		}
	})
	if err != nil || replayed {
		mcpError(w, 400, "invalid_grant")
		return
	}
	mcpJSON(w, 200, result)
}
func mcpAccess(r *http.Request) (dbmodel.MCPOAuthGrant, dbmodel.User, dbmodel.UserRelation, error) {
	var g dbmodel.MCPOAuthGrant
	var u dbmodel.User
	var rel dbmodel.UserRelation
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || len(parts[1]) > 256 || !strings.EqualFold(parts[0], "Bearer") {
		return g, u, rel, errors.New("missing bearer")
	}
	var t dbmodel.MCPOAuthToken
	if e := database.Connection.First(&t, "hash = ? AND kind = ? AND expires_at > ?", mcpHash(parts[1]), "access", time.Now()).Error; e != nil {
		return g, u, rel, e
	}
	o, e := mcpOrigin()
	if e != nil {
		return g, u, rel, e
	}
	if e = database.Connection.First(&g, "id = ? AND revoked = ? AND expires_at > ? AND resource = ?", t.GrantID, false, time.Now(), o+"/mcp").Error; e != nil {
		return g, u, rel, e
	}
	u, rel, e = mcpCurrentActor(database.Connection, g.UserID, g.RelationID)
	return g, u, rel, e
}
func MCPAuthenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		o, ok := mcpReady(w)
		if !ok {
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != o {
			mcpError(w, 403, "access_denied")
			return
		}
		g, u, rel, e := mcpAccess(r)
		if e != nil {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer resource_metadata="%s/.well-known/oauth-protected-resource/mcp"`, o))
			mcpError(w, 401, "invalid_token")
			return
		}
		next.ServeHTTP(w, r.WithContext(ContextWithMCPPrincipal(r.Context(), NewOAuthIntegrationPrincipal(u, rel, strings.Fields(g.Scope)))))
	})
}
func MCPRevoke(w http.ResponseWriter, r *http.Request) {
	if _, ok := mcpReady(w); !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if r.ParseForm() != nil {
		mcpError(w, 400, "invalid_request")
		return
	}
	var t dbmodel.MCPOAuthToken
	if database.Connection.First(&t, "hash = ?", mcpHash(r.PostForm.Get("token"))).Error == nil {
		if err := database.Connection.Model(&dbmodel.MCPOAuthGrant{}).Where("id = ? AND client_id = ?", t.GrantID, r.PostForm.Get("client_id")).Update("revoked", true).Error; err != nil {
			mcpError(w, 500, "server_error")
			return
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(200)
}

// Connections are authenticated with the caller's MCP token, never a shared admin key.
func MCPConnections(w http.ResponseWriter, r *http.Request) {
	if _, ok := mcpReady(w); !ok {
		return
	}
	g, _, _, e := mcpAccess(r)
	if e != nil {
		mcpError(w, 401, "invalid_token")
		return
	}
	if r.Method == http.MethodDelete {
		if database.Connection.Model(&dbmodel.MCPOAuthGrant{}).Where("id = ? AND user_id = ?", g.ID, g.UserID).Update("revoked", true).Error != nil {
			mcpError(w, 500, "server_error")
			return
		}
		w.WriteHeader(204)
		return
	}
	// Return only this client's connection, avoiding exposure of the user's other apps.
	mcpJSON(w, 200, map[string]interface{}{"id": g.ID, "client_id": g.ClientID, "relation_id": g.RelationID, "scope": g.Scope, "expires_at": g.ExpiresAt})
}
