package service

import (
	"strings"
	"testing"

	"mapmarker/backend/database/dbmodel"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestOrderActiveMarkersLatestFirst(t *testing.T) {
	db, err := gorm.Open(postgres.Open(""), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open dry-run database: %v", err)
	}

	query := orderActiveMarkersLatestFirst(db.Model(&dbmodel.Marker{}))
	query.Find(&[]dbmodel.Marker{})
	if sql := strings.ToLower(query.Statement.SQL.String()); !strings.Contains(sql, "order by created_at desc,id desc") {
		t.Fatalf("expected newest-first marker ordering, got SQL %q", sql)
	}
}
