package service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestMCPRedirectPolicy(t *testing.T) {
	for _, tc := range []struct {
		url string
		ok  bool
	}{{"https://client.example/callback", true}, {"http://127.0.0.1:4567/cb", true}, {"http://[::1]:4567/cb", true}, {"http://localhost/cb", false}, {"https://u:p@client.example/cb", false}, {"https://client.example/cb#fragment", false}, {"javascript:alert(1)", false}, {"http://client.example/cb", false}, {"/relative", false}} {
		if got := validMCPRedirect(tc.url); got != tc.ok {
			t.Errorf("%q got %v", tc.url, got)
		}
	}
}
func TestMCPPKCE(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challenge := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if !mcpPKCE(verifier, challenge) {
		t.Fatal("RFC 7636 S256 vector rejected")
	}
	for _, v := range []string{"", verifier + "!", strings.Repeat("a", 129), "short", strings.Replace(verifier, "d", "x", 1)} {
		if mcpPKCE(v, challenge) {
			t.Fatalf("accepted invalid verifier %q", v)
		}
	}
}
func TestMCPScopeAndOriginPolicy(t *testing.T) {
	scope, e := normalizeMCPScope("")
	if e != nil || strings.Contains(scope, ":write") {
		t.Fatal("default not read-only")
	}
	if _, e = normalizeMCPScope("markers:read admin:all"); e == nil {
		t.Fatal("unknown privilege accepted")
	}
	if _, e = normalizeMCPScope("settings:write"); e == nil {
		t.Fatal("non-exposed tool write scope accepted")
	}
	for _, o := range []string{"http://rm.example", "https://rm.example/path", "https://u:p@rm.example", "https://rm.example?x=1"} {
		t.Setenv("MCP_PUBLIC_ORIGIN", o)
		if _, e = mcpOrigin(); e == nil {
			t.Errorf("accepted origin %s", o)
		}
	}
	t.Setenv("MCP_PUBLIC_ORIGIN", "https://rm.example")
	if got, e := mcpOrigin(); e != nil || got != "https://rm.example" {
		t.Fatal(got, e)
	}
}
func TestMCPMetadataAndOriginRejection(t *testing.T) {
	t.Setenv("MCP_ENABLED", "true")
	t.Setenv("MCP_PUBLIC_ORIGIN", "https://rm.example")
	w := httptest.NewRecorder()
	MCPResourceMetadata(w, httptest.NewRequest("GET", "https://rm.example/.well-known/oauth-protected-resource/mcp", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"resource":"https://rm.example/mcp"`) {
		t.Fatal(w.Body.String())
	}
	called := false
	h := MCPAuthenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	r := httptest.NewRequest("POST", "https://rm.example/mcp", nil)
	r.Header.Set("Origin", "https://attacker.example")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 || called {
		t.Fatal("cross-origin request reached application")
	}
	r = httptest.NewRequest("POST", "https://rm.example/mcp", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 || !strings.Contains(w.Header().Get("WWW-Authenticate"), "resource_metadata") {
		t.Fatal("missing OAuth challenge")
	}
}

func TestMCPAuthorizationMetadataAdvertisesAutomaticClientRegistration(t *testing.T) {
	t.Setenv("MCP_ENABLED", "true")
	t.Setenv("MCP_PUBLIC_ORIGIN", "https://rm.example")

	w := httptest.NewRecorder()
	MCPAuthorizationMetadata(w, httptest.NewRequest("GET", "https://rm.example/.well-known/oauth-authorization-server", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("metadata status = %d, body=%s", w.Code, w.Body.String())
	}

	var metadata map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &metadata); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if metadata["registration_endpoint"] != "https://rm.example/mcp/oauth/register" {
		t.Fatalf("unexpected registration endpoint: %v", metadata["registration_endpoint"])
	}
	if metadata["authorization_response_iss_parameter_supported"] != true {
		t.Fatalf("automatic OAuth client registration metadata must advertise issuer responses: %v", metadata)
	}
	if metadata["token_endpoint_auth_methods_supported"].([]any)[0] != "none" {
		t.Fatalf("expected public OAuth clients: %v", metadata["token_endpoint_auth_methods_supported"])
	}
}
func TestMCPOpaqueHash(t *testing.T) {
	a, b := mcpRandom(), mcpRandom()
	if a == b || len(a) != 43 || !mcpEqualHash(a, mcpHash(a)) || mcpEqualHash(b, mcpHash(a)) {
		t.Fatal("invalid credential generation/hash")
	}
}

// Run explicitly against disposable PostgreSQL using MCP_TEST_DATABASE_URL.
// Each run creates a unique schema and never reads or modifies application data.
func TestMCPOAuthDatabaseSecurity(t *testing.T) {
	dsn := os.Getenv("MCP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MCP_TEST_DATABASE_URL not set; PostgreSQL token lifecycle test not run")
	}
	db, e := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	sqlDB, e := db.DB()
	if e != nil {
		t.Fatal(e)
	}
	sqlDB.SetMaxOpenConns(1)
	schema := "mcp_test_" + mcpHash(mcpRandom())[:16]
	if e = db.Exec("CREATE SCHEMA " + schema).Error; e != nil {
		t.Fatal(e)
	}
	defer func() {
		db.Exec("SET search_path TO public")
		db.Exec("DROP SCHEMA " + schema + " CASCADE")
		sqlDB.Close()
	}()
	if e = db.Exec("SET search_path TO " + schema).Error; e != nil {
		t.Fatal(e)
	}
	if e = db.AutoMigrate(&dbmodel.User{}, &dbmodel.UserRelation{}, &dbmodel.MCPOAuthClient{}, &dbmodel.MCPOAuthRequest{}, &dbmodel.MCPOAuthGrant{}, &dbmodel.MCPOAuthToken{}); e != nil {
		t.Fatal(e)
	}
	old := database.Connection
	database.Connection = db
	defer func() { database.Connection = old }()
	t.Setenv("MCP_ENABLED", "true")
	t.Setenv("MCP_PUBLIC_ORIGIN", "https://rm.example")
	u := dbmodel.User{Username: "owner", IsActivated: true, Role: "admin"}
	other := dbmodel.User{Username: "other", IsActivated: true, Role: "user"}
	db.Create(&u)
	db.Create(&other)
	rel := dbmodel.UserRelation{UserOneUID: u.ID, UserTwoUID: u.ID}
	db.Create(&rel)
	if _, _, e = mcpCurrentActor(db, other.ID, rel.ID); e == nil {
		t.Fatal("cross-relation user accepted")
	}

	// Consent cannot be driven by an unrelated browser or forged form.
	browser := mcpRandom()
	pending := dbmodel.MCPOAuthRequest{ID: mcpRandom(), ClientID: "consent-client", BrowserHash: mcpHash(browser), CodeHash: mcpHash(mcpRandom()), Resource: "https://rm.example/mcp", RedirectURI: "https://client.example/cb", Scope: "markers:read", ExpiresAt: time.Now().Add(time.Minute)}
	db.Create(&dbmodel.MCPOAuthClient{ID: "consent-client", Name: "Test client", RedirectURIs: `["https://client.example/cb"]`})
	db.Create(&pending)
	callback := httptest.NewRequest("GET", "https://rm.example/auth/oidc/callback", nil)
	w := httptest.NewRecorder()
	completeMCPOIDCLogin(w, callback, &u, pending.ID)
	if w.Code != 403 {
		t.Fatal("callback accepted absent bound browser")
	}
	callback.AddCookie(&http.Cookie{Name: mcpBrowserCookie, Value: browser})
	w = httptest.NewRecorder()
	completeMCPOIDCLogin(w, callback, &u, pending.ID)
	if w.Code != 200 {
		t.Fatal("consent failed", w.Code, w.Body.String())
	}
	csrfMatch := regexp.MustCompile(`name="csrf" value="([^"]+)"`).FindStringSubmatch(w.Body.String())
	if len(csrfMatch) != 2 {
		t.Fatal("consent CSRF missing")
	}
	consentCall := func(csrf, relation string) *httptest.ResponseRecorder {
		f := url.Values{"request_id": {pending.ID}, "csrf": {csrf}, "relation_id": {relation}, "decision": {"allow"}}
		r := httptest.NewRequest("POST", "https://rm.example/mcp/oauth/consent", strings.NewReader(f.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", "https://rm.example")
		r.AddCookie(&http.Cookie{Name: mcpBrowserCookie, Value: browser})
		w := httptest.NewRecorder()
		MCPConsent(w, r)
		return w
	}
	if w = consentCall("forged", "1"); w.Code != 403 {
		t.Fatal("forged consent accepted")
	}
	if w = consentCall(csrfMatch[1], "99999"); w.Code != 403 {
		t.Fatal("cross-relation consent accepted")
	}
	verifier := strings.Repeat("a", 43)
	digest := sha256.Sum256([]byte(verifier))
	code := mcpRandom()
	req := dbmodel.MCPOAuthRequest{ID: mcpRandom(), ClientID: "test-client", UserID: u.ID, RelationID: rel.ID, Resource: "https://rm.example/mcp", RedirectURI: "https://client.example/cb", Scope: "markers:read", Challenge: base64.RawURLEncoding.EncodeToString(digest[:]), CodeHash: mcpHash(code), ExpiresAt: time.Now().Add(time.Minute)}
	db.Create(&req)
	tokenCall := func(f url.Values) (int, map[string]interface{}) {
		r := httptest.NewRequest("POST", "https://rm.example/mcp/oauth/token", strings.NewReader(f.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		MCPToken(w, r)
		var result map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &result)
		return w.Code, result
	}
	f := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {"test-client"}, "resource": {"https://rm.example/mcp"}, "redirect_uri": {"https://client.example/cb"}, "code_verifier": {verifier}}
	f.Set("resource", "https://wrong.example/mcp")
	if status, _ := tokenCall(f); status == 200 {
		t.Fatal("wrong audience accepted")
	}
	f.Set("resource", "https://rm.example/mcp")

	f.Set("redirect_uri", "https://client.example/other")
	if status, _ := tokenCall(f); status == 200 {
		t.Fatal("redirect mismatch accepted")
	}
	f.Set("redirect_uri", "https://client.example/cb")
	f.Set("client_id", "other-client")
	if status, _ := tokenCall(f); status == 200 {
		t.Fatal("wrong client accepted")
	}
	f.Set("client_id", "test-client")
	f.Set("code_verifier", strings.Repeat("b", 43))
	if status, _ := tokenCall(f); status == 200 {
		t.Fatal("wrong PKCE accepted")
	}
	f.Set("code_verifier", verifier)
	status, result := tokenCall(f)
	if status != 200 {
		t.Fatal(status, result)
	}
	if status, _ = tokenCall(f); status == 200 {
		t.Fatal("authorization code reused")
	}
	access := result["access_token"].(string)
	refresh := result["refresh_token"].(string)
	accessRequest := httptest.NewRequest("POST", "https://rm.example/mcp", nil)
	accessRequest.Header.Set("Authorization", "Bearer "+access)
	if _, _, _, e = mcpAccess(accessRequest); e != nil {
		t.Fatal(e)
	}
	db.Model(&u).Update("is_activated", false)
	if _, _, _, e = mcpAccess(accessRequest); e == nil {
		t.Fatal("disabled user accepted")
	}
	db.Model(&u).Update("is_activated", true)
	rf := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {"test-client"}, "resource": {"https://rm.example/mcp"}}
	status, rotated := tokenCall(rf)
	if status != 200 || rotated["refresh_token"] == refresh {
		t.Fatal("refresh did not rotate", status, rotated)
	}
	if status, _ = tokenCall(rf); status == 200 {
		t.Fatal("refresh replay accepted")
	}
	if _, _, _, e = mcpAccess(accessRequest); e == nil {
		t.Fatal("refresh replay did not revoke family")
	}
	accessRequest.Header.Set("Authorization", "Bearer "+rotated["access_token"].(string))
	if _, _, _, e = mcpAccess(accessRequest); e == nil {
		t.Fatal("new access survived family revocation")
	}
}

func TestMCPOAuthRateLimit(t *testing.T) {
	r := httptest.NewRequest("POST", "https://rm.example/mcp/oauth/register", nil)
	r.RemoteAddr = "192.0.2.101:1234"
	operation := "test-" + mcpRandom()
	for i := 0; i < 2; i++ {
		if !mcpRateLimit(httptest.NewRecorder(), r, operation, 2) {
			t.Fatal("early limit")
		}
	}
	w := httptest.NewRecorder()
	if mcpRateLimit(w, r, operation, 2) || w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatal("registration not bounded")
	}
	r.Header.Set("X-Forwarded-For", "203.0.113.1")
	if mcpRateLimit(httptest.NewRecorder(), r, operation, 2) {
		t.Fatal("untrusted forwarded header bypassed limit")
	}
}
