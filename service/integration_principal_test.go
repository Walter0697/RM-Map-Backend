package service

import (
	"context"
	"mapmarker/backend/database/dbmodel"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testOAuthPrincipal() *IntegrationPrincipal {
	u := dbmodel.User{IsActivated: true, Role: "user"}
	u.ID = 10
	r := dbmodel.UserRelation{UserOneUID: 10, UserTwoUID: 20}
	r.ID = 30
	return NewOAuthIntegrationPrincipal(u, r, []string{"markers:read"})
}
func TestOAuthPrincipalRejectsInvalidIdentity(t *testing.T) {
	p := testOAuthPrincipal()
	for _, change := range []func(*dbmodel.User, *dbmodel.UserRelation){
		func(u *dbmodel.User, r *dbmodel.UserRelation) { u.ID = 0 },
		func(u *dbmodel.User, r *dbmodel.UserRelation) { u.IsActivated = false },
		func(u *dbmodel.User, r *dbmodel.UserRelation) { r.UserOneUID = 99 },
		func(u *dbmodel.User, r *dbmodel.UserRelation) { r.ID = 0 },
	} {
		u, r := p.ActorUser, p.Relation
		change(&u, &r)
		if NewOAuthIntegrationPrincipal(u, r, p.Scopes) != nil {
			t.Fatal("accepted invalid identity/relation")
		}
	}
}
func TestOAuthPrincipalScopeAndRoleBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, scope string
		mutate      func(*IntegrationPrincipal)
		allowed     bool
	}{
		{"read", "markers:read", func(*IntegrationPrincipal) {}, true},
		{"write", "markers:write", func(*IntegrationPrincipal) {}, false},
		{"wrong relation", "markers:read", func(p *IntegrationPrincipal) { p.Relation.UserOneUID = 999 }, false},
		{"inactive", "markers:read", func(p *IntegrationPrincipal) { p.ActorUser.IsActivated = false }, false},
		{"role elevation", "markers:read", func(p *IntegrationPrincipal) { p.Role = "admin" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testOAuthPrincipal()
			tc.mutate(p)
			r := httptest.NewRequest(http.MethodGet, "/integration/markers", nil).WithContext(ContextWithMCPPrincipal(context.Background(), p))
			w := httptest.NewRecorder()
			legacy := func(http.ResponseWriter, *http.Request, string, string, string) (*dbmodel.APIKey, bool) {
				t.Fatal("OAuth attempted API-key authentication")
				return nil, false
			}
			got, ok := authenticateIntegrationPrincipalWith(w, r, "test", tc.scope, "", legacy, false)
			if ok != tc.allowed {
				t.Fatalf("allowed=%v want %v", ok, tc.allowed)
			}
			if ok && (got.APIKey != nil || got.Actor().ID != 10) {
				t.Fatal("OAuth borrowed a key/actor")
			}
			if !ok && w.Code != http.StatusForbidden {
				t.Fatalf("status %d", w.Code)
			}
		})
	}
}
func TestLegacyIntegrationPrincipalPreservesKeySemantics(t *testing.T) {
	key := &dbmodel.APIKey{Testing: true, Scopes: "markers:read", RelationID: 30}
	key.ID = 7
	key.ActorUser.ID = 10
	key.ActorUser.Role = "user"
	key.ActorUserID = &key.ActorUser.ID
	key.ServiceAccount = &dbmodel.ServiceAccount{Role: "admin"}
	sid := uint(8)
	key.ServiceAccountID = &sid
	r := httptest.NewRequest(http.MethodGet, "/integration/markers", nil)
	w := httptest.NewRecorder()
	legacy := func(http.ResponseWriter, *http.Request, string, string, string) (*dbmodel.APIKey, bool) {
		return key, true
	}
	p, ok := authenticateIntegrationPrincipalWith(w, r, "test", "markers:read", "", legacy, false)
	if !ok || p.APIKey != key || p.ActorRole() != "admin" || !p.Testing || !p.HasScope("markers:read") {
		t.Fatal("legacy semantics changed")
	}
}
