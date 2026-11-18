package service

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"mapmarker/backend/config"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/utils"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// This test executes the pre-staging SQL from the operator document itself, then
// runs GORM's model migration. It uses a unique disposable schema, never app data.
func TestOIDCPreStagedPostgresMigration(t *testing.T) {
	dsn := os.Getenv("MCP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MCP_TEST_DATABASE_URL not set; OIDC PostgreSQL migration test not run")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	schema := "oidc_test_" + mcpHash(mcpRandom())[:16]
	if err := db.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	defer func() {
		db.Exec("SET search_path TO public")
		db.Exec("DROP SCHEMA " + schema + " CASCADE")
		sqlDB.Close()
	}()
	if err := db.Exec("SET search_path TO " + schema).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&dbmodel.User{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldConfig := database.Connection, config.Data
	database.Connection = db
	defer func() { database.Connection = oldDB; config.Data = oldConfig }()
	config.Data.OIDC.Issuer = "https://provider.example/application/"
	config.Data.App.JWT = "isolated-test-signing-secret"
	mgr := &defaultAuthStateManager{mode: config.AuthStateModeDualWrite, sessionTTL: time.Hour}
	mgr.writePostgresFn = mgr.writePostgres
	mgr.validatePostgresFn = mgr.validatePostgres
	mgr.revokePostgresFn = mgr.revokePostgres
	restore := setAuthStateManagerForTest(mgr)
	defer restore()
	user := dbmodel.User{Username: "existing-user", Role: "admin", IsActivated: true, LoginToken: "existing-session-secret"}
	other := dbmodel.User{Username: "other-user", Role: "user", IsActivated: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	// Token issued by the old browser path before adding identity schema.
	legacyToken, err := utils.GenerateToken(user.Username, user.LoginToken)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := os.ReadFile("../docs/oidc-identity-migration.md")
	if err != nil {
		t.Fatal(err)
	}
	blocks := regexp.MustCompile("(?s)```sql\\n(.*?)```").FindAllSubmatch(doc, -1)
	if len(blocks) < 1 {
		t.Fatal("migration SQL missing")
	}
	for _, sql := range strings.Split(string(blocks[0][1]), ";") {
		if strings.TrimSpace(sql) != "" {
			if err := db.Exec(sql).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := db.Exec("INSERT INTO oidc_identities (issuer,subject,user_id) VALUES (?,?,?)", config.Data.OIDC.Issuer, "verified-subject", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := db.AutoMigrate(&dbmodel.OIDCIdentity{}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := resolveVerifiedOIDCUser(config.Data.OIDC.Issuer, "verified-subject", "renamed-provider-username")
	if err != nil || got.ID != user.ID || got.Username != user.Username || got.Role != user.Role {
		t.Fatalf("binding changed identity: %#v %v", got, err)
	}
	linked, err := resolveVerifiedOIDCUser(config.Data.OIDC.Issuer, "unbound-subject", user.Username)
	if err != nil || linked.ID != user.ID {
		t.Fatalf("configured Authentik username did not link existing account: %#v %v", linked, err)
	}
	var compatibilityBinding dbmodel.OIDCIdentity
	if err := db.Where("issuer = ? AND subject = ?", config.Data.OIDC.Issuer, "unbound-subject").First(&compatibilityBinding).Error; err != nil || compatibilityBinding.UserID != user.ID {
		t.Fatalf("username compatibility binding was not persisted: %#v %v", compatibilityBinding, err)
	}
	if err := BindOIDCIdentity(user.ID, config.Data.OIDC.Issuer, "verified-subject"); err != nil {
		t.Fatal("idempotent enrollment", err)
	}
	if err := BindOIDCIdentity(other.ID, config.Data.OIDC.Issuer, "verified-subject"); err == nil {
		t.Fatal("identity reassigned")
	}
	if err := db.Create(&dbmodel.OIDCIdentity{Issuer: config.Data.OIDC.Issuer, Subject: "verified-subject", UserID: other.ID}).Error; err == nil {
		t.Fatal("unique index missing")
	}
	loggedIn, err := ValidateToken(legacyToken)
	if err != nil || loggedIn == nil || loggedIn.ID != user.ID {
		t.Fatalf("legacy session invalidated: %v", err)
	}
	// Normal browser session issuance after verified identity lookup is unchanged.
	newToken, err := issueTokenForUser(got)
	if err != nil {
		t.Fatal(err)
	}
	loggedIn, err = ValidateToken(newToken)
	if err != nil || loggedIn == nil || loggedIn.ID != user.ID {
		t.Fatalf("bound session failed: %v", err)
	}
	if err := db.Model(&user).Update("is_activated", false).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := resolveVerifiedOIDCUser(config.Data.OIDC.Issuer, "verified-subject", user.Username); err == nil {
		t.Fatal("disabled account accepted")
	}
	if err := db.Model(&user).Update("is_activated", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&user).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := resolveVerifiedOIDCUser(config.Data.OIDC.Issuer, "verified-subject", user.Username); err == nil {
		t.Fatal("deleted account accepted")
	}
}
