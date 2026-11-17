package service

import (
	"context"
	"log"
	"mapmarker/backend/database/dbmodel"
	"net/http"
	"strings"
)

// IntegrationPrincipal is the authorization context shared by REST integrations
// and native MCP. APIKey is present only for an actual authenticated legacy key.
// OAuth does not manufacture credentials or borrow a service-account identity.
type IntegrationPrincipal struct {
	ActorUser   dbmodel.User
	ActorUserID *uint
	Relation    dbmodel.UserRelation
	RelationID  uint
	Scopes      []string
	Role        string
	Testing     bool
	APIKey      *dbmodel.APIKey
}

type mcpPrincipalContextKey struct{}

func NewOAuthIntegrationPrincipal(user dbmodel.User, relation dbmodel.UserRelation, scopes []string) *IntegrationPrincipal {
	if user.ID == 0 || !user.IsActivated || relation.ID == 0 || (relation.UserOneUID != user.ID && relation.UserTwoUID != user.ID) {
		return nil
	}
	id := user.ID
	return &IntegrationPrincipal{ActorUser: user, ActorUserID: &id, Relation: relation, RelationID: relation.ID, Scopes: append([]string(nil), scopes...), Role: user.Role}
}

func principalFromAPIKey(key *dbmodel.APIKey) *IntegrationPrincipal {
	if key == nil {
		return nil
	}
	return &IntegrationPrincipal{ActorUser: key.ActorUser, ActorUserID: key.ActorUserID, Relation: key.Relation, RelationID: key.RelationID, Scopes: key.ScopeList(), Role: APIKeyActorRole(key), Testing: key.Testing, APIKey: key}
}

func ContextWithMCPPrincipal(ctx context.Context, principal *IntegrationPrincipal) context.Context {
	return context.WithValue(ctx, mcpPrincipalContextKey{}, principal)
}
func MCPPrincipalForContext(ctx context.Context) *IntegrationPrincipal {
	principal, _ := ctx.Value(mcpPrincipalContextKey{}).(*IntegrationPrincipal)
	return principal
}
func (p *IntegrationPrincipal) HasScope(scope string) bool {
	if p == nil || strings.TrimSpace(scope) == "" {
		return false
	}
	for _, granted := range p.Scopes {
		if granted == scope {
			return true
		}
	}
	return false
}
func (p *IntegrationPrincipal) ActorRole() string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(p.Role)
}
func (p *IntegrationPrincipal) Actor() *dbmodel.User {
	if p == nil || p.ActorUserID == nil || p.ActorUser.ID == 0 {
		return nil
	}
	return &p.ActorUser
}
func (p *IntegrationPrincipal) AuditID() uint {
	if p != nil && p.APIKey != nil {
		return p.APIKey.ID
	}
	return 0
}
func (p *IntegrationPrincipal) AuditName() string {
	if p != nil && p.APIKey != nil {
		return p.APIKey.Name
	}
	return "mcp-oauth"
}
func (p *IntegrationPrincipal) RecordAudit(event APIKeyAuditEvent) error {
	if p.APIKey != nil {
		return integrationCreateAuditLogFn(&p.APIKey.ID, p.APIKey.Name, event)
	}
	// Keep OAuth records distinct: API-key foreign keys must never reference an OAuth grant.
	log.Printf("mcp integration actor_id=%d relation_id=%d operation=%q success=%t", p.ActorUser.ID, p.Relation.ID, event.Operation, event.Success)
	return nil
}

type integrationKeyAuthenticator func(http.ResponseWriter, *http.Request, string, string, string) (*dbmodel.APIKey, bool)

func authenticateIntegrationPrincipalWith(w http.ResponseWriter, r *http.Request, operation, scope, query string, legacy integrationKeyAuthenticator, jsonError bool) (*IntegrationPrincipal, bool) {
	if p := MCPPrincipalForContext(r.Context()); p != nil {
		// Defense in depth for in-process dispatch. The OAuth middleware additionally
		// reloads the current user and relation from the database on every request.
		valid := p.APIKey == nil && p.ActorUser.ID != 0 && p.ActorUser.IsActivated && p.Relation.ID != 0 && (p.Relation.UserOneUID == p.ActorUser.ID || p.Relation.UserTwoUID == p.ActorUser.ID) && p.Role == p.ActorUser.Role
		if !valid || !p.HasScope(scope) {
			if jsonError {
				writeIntegrationError(w, http.StatusForbidden, "scope_denied", "permission denied")
			} else {
				http.Error(w, "permission denied", http.StatusForbidden)
			}
			return nil, false
		}
		_ = p.RecordAudit(APIKeyAuditEvent{Operation: operation, Success: true})
		return p, true
	}
	key, ok := legacy(w, r, operation, scope, query)
	if !ok {
		return nil, false
	}
	return principalFromAPIKey(key), true
}
func authenticateIntegrationPrincipal(w http.ResponseWriter, r *http.Request, operation, scope, query string) (*IntegrationPrincipal, bool) {
	return authenticateIntegrationPrincipalWith(w, r, operation, scope, query, authenticateIntegrationRequest, false)
}
func authenticateIntegrationPrincipalHook(w http.ResponseWriter, r *http.Request, operation, scope, query string) (*IntegrationPrincipal, bool) {
	return authenticateIntegrationPrincipalWith(w, r, operation, scope, query, integrationAuthenticateRequestFn, false)
}
func authenticateIntegrationNearbyPrincipal(w http.ResponseWriter, r *http.Request, operation, scope, query string) (*IntegrationPrincipal, bool) {
	return authenticateIntegrationPrincipalWith(w, r, operation, scope, query, integrationAuthenticateNearbyRequestFn, true)
}
func authenticateTravelPlanPrincipal(w http.ResponseWriter, r *http.Request, operation, scope, query string) (*IntegrationPrincipal, bool) {
	return authenticateIntegrationPrincipalWith(w, r, operation, scope, query, integrationAuthenticateTravelPlanRequestFn, false)
}
func authenticateTravelPlanPrincipalJSON(w http.ResponseWriter, r *http.Request, operation, scope, query string) (*IntegrationPrincipal, bool) {
	return authenticateIntegrationPrincipalWith(w, r, operation, scope, query, integrationAuthenticateTravelPlanRequestJSONFn, true)
}
func resolveCreateTestingFlagForPrincipal(p *IntegrationPrincipal, requested *bool, canAccess bool) (bool, error) {
	if p != nil && p.Testing {
		return true, nil
	}
	return resolveRequestedTestingFlag(requested, canAccess)
}
