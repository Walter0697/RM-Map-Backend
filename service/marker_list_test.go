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

func TestOrderViewportMarkersLatestFirstUsesDescendingCursor(t *testing.T) {
	db, err := gorm.Open(postgres.Open(""), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open dry-run database: %v", err)
	}

	query := orderViewportMarkersLatestFirst(db.Model(&dbmodel.Marker{}), 42)
	query.Find(&[]dbmodel.Marker{})
	sql := strings.ToLower(query.Statement.SQL.String())
	if !strings.Contains(sql, "id < $1") || !strings.Contains(sql, "order by id desc") {
		t.Fatalf("expected descending viewport cursor ordering, got SQL %q", sql)
	}
}
