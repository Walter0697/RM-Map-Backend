package dbmodel

import "time"

// OAuth credentials are stored only as SHA-256 hashes. No browser session token
// or OIDC provider access token is an MCP credential.
type MCPOAuthClient struct {
	ID           string `gorm:"primaryKey;size:64"`
	Name         string `gorm:"size:120"`
	RedirectURIs string `gorm:"type:text"`
	CreatedAt    time.Time
}
type MCPOAuthRequest struct {
	ID          string `gorm:"primaryKey;size:64"`
	ClientID    string `gorm:"index;size:64"`
	RedirectURI string `gorm:"type:text"`
	Resource    string `gorm:"type:text"`
	State       string `gorm:"type:text"`
	Scope       string `gorm:"type:text"`
	Challenge   string `gorm:"size:64"`
	BrowserHash string `gorm:"size:64"`
	CSRFHash    string `gorm:"size:64"`
	UserID      uint
	CodeHash    string `gorm:"uniqueIndex;size:64"`
	RelationID  uint
	Used        bool
	ExpiresAt   time.Time `gorm:"index"`
}
type MCPOAuthGrant struct {
	ID         string `gorm:"primaryKey;size:64"`
	ClientID   string `gorm:"index;size:64"`
	UserID     uint   `gorm:"index"`
	RelationID uint
	Scope      string `gorm:"type:text"`
	Resource   string `gorm:"type:text"`
	Revoked    bool
	CreatedAt  time.Time
	ExpiresAt  time.Time
}
type MCPOAuthToken struct {
	Hash      string `gorm:"primaryKey;size:64"`
	GrantID   string `gorm:"index;size:64"`
	Kind      string `gorm:"size:10"`
	Used      bool
	ExpiresAt time.Time `gorm:"index"`
}
