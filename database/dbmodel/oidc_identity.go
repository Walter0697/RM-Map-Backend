package dbmodel

// OIDCIdentity binds a verified issuer and immutable subject to the local account.
// Usernames and email claims must never be used to automatically link accounts.
type OIDCIdentity struct {
	ID      uint   `gorm:"primaryKey"`
	Issuer  string `gorm:"not null;uniqueIndex:oidc_issuer_subject"`
	Subject string `gorm:"not null;uniqueIndex:oidc_issuer_subject"`
	UserID  uint   `gorm:"not null;index"`
}

// Keep the persisted name stable: GORM v1.21 splits the OIDC initialism into
// o_id_c by default, which would otherwise diverge from the bootstrap SQL.
func (OIDCIdentity) TableName() string { return "oidc_identities" }
