package dbmodel

// OIDCIdentity binds a verified issuer and immutable subject to the local account.
// Usernames and email claims must never be used to automatically link accounts.
type OIDCIdentity struct {
	ID      uint   `gorm:"primaryKey"`
	Issuer  string `gorm:"not null;uniqueIndex:oidc_issuer_subject"`
	Subject string `gorm:"not null;uniqueIndex:oidc_issuer_subject"`
	UserID  uint   `gorm:"not null;index"`
}
