package service

import (
	"gorm.io/gorm/schema"
	"mapmarker/backend/config"
	"mapmarker/backend/database/dbmodel"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestOIDCStateIsRandomSingleUseAndExpires(t *testing.T) {
	a, b := GenerateOIDCState(), GenerateOIDCState()
	if len(a) < 43 || a == b {
		t.Fatal("weak state")
	}
	for _, expired := range []bool{false, true} {
		expiry := time.Now().Add(time.Minute)
		if expired {
			expiry = time.Now().Add(-time.Second)
		}
		state := GenerateOIDCState()
		oidcStateStore.Store(state, oidcStateValue{ExpireAt: expiry, Nonce: "nonce", Verifier: "verifier", BrowserSecret: "browser", MCPRequestID: "request"})
		result, ok := consumeOIDCState(state)
		if ok == expired {
			t.Fatal("incorrect state validity")
		}
		if ok && result.MCPRequestID != "request" {
			t.Fatal("lost OAuth request")
		}
		if _, ok := consumeOIDCState(state); ok {
			t.Fatal("state reused")
		}
	}
}

func TestOIDCProductionTransportRequiresHTTPS(t *testing.T) {
	original := config.Data
	defer func() { config.Data = original }()
	config.Data.App.Environment = "production"
	for _, raw := range []string{"http://provider.example/token", "file:///secret", "https://user:pass@provider.example/keys", "https://provider.example/keys#fragment"} {
		if validateOIDCTransportURL(raw) == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if err := validateOIDCTransportURL("https://provider.example/token"); err != nil {
		t.Fatal(err)
	}
}

func TestOIDCEnrollmentRejectsUnverifiedParametersBeforeDatabaseAccess(t *testing.T) {
	original := config.Data
	defer func() { config.Data = original }()
	config.Data.OIDC.Issuer = "https://provider.example/"
	for _, input := range []struct {
		id              uint
		issuer, subject string
	}{{0, "https://provider.example/", "sub"}, {1, "https://other.example/", "sub"}, {1, "https://provider.example/", ""}, {1, "https://provider.example/", " sub"}} {
		if BindOIDCIdentity(input.id, input.issuer, input.subject) == nil {
			t.Fatal("invalid binding accepted")
		}
	}
	if _, err := resolveVerifiedOIDCUser("", "subject", "existing-user"); err == nil {
		t.Fatal("missing issuer accepted")
	}
}

func TestOIDCCodeExchangeDoesNotFollowRedirects(t *testing.T) {
	original := config.Data
	oldClient := oidcHTTPClient
	defer func() { config.Data = original; oidcHTTPClient = oldClient }()
	config.Data.App.Environment = "test"
	config.Data.OIDC.ClientSecret = "secret-not-for-redirect"
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		forwarded := false
		target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded = true; w.Write([]byte(`{"id_token":"bad"}`)) }))
		source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, status) }))
		oidcHTTPClient = source.Client() // deliberately has the ordinary follow-redirect policy
		if _, err := exchangeOIDCCode(source.URL, "private-code", "private-verifier"); err == nil {
			t.Fatal("accepted redirect")
		}
		if forwarded {
			t.Fatalf("forwarded credential-bearing request through %d", status)
		}
		source.Close()
		target.Close()
	}
}

func TestOIDCIdentityBootstrapTableName(t *testing.T) {
	parsed, err := schema.Parse(&dbmodel.OIDCIdentity{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Table != "oidc_identities" {
		t.Fatalf("bootstrap table mismatch: %s", parsed.Table)
	}
	indexes := parsed.ParseIndexes()
	for _, name := range []string{"oidc_issuer_subject", "idx_oidc_identities_user_id"} {
		if _, ok := indexes[name]; !ok {
			t.Fatalf("missing bootstrap index %s", name)
		}
	}
}
