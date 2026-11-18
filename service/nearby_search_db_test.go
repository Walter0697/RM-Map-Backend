package service

import (
	"os"
	"testing"

	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestNearbySearchFiltersBeforeLimitPostgres(t *testing.T) {
	dsn := os.Getenv("MCP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MCP_TEST_DATABASE_URL not set; PostgreSQL nearby search test not run")
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
	t.Cleanup(func() { sqlDB.Close() })
	schema := "nearby_test_" + mcpHash(mcpRandom())[:16]
	exec := func(stmt string, args ...interface{}) {
		t.Helper()
		if err := db.Exec(stmt, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	exec("CREATE SCHEMA " + schema)
	t.Cleanup(func() {
		db.Exec("SET search_path TO public")
		if err := db.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
	})
	exec("SET search_path TO " + schema)
	exec(`CREATE TABLE marker_types (value text, hidden boolean)`)
	exec(`INSERT INTO marker_types VALUES ('food', false), ('walk', false), ('hidden', true)`)
	exec(`CREATE TABLE markers (
		id bigint PRIMARY KEY, relation_id bigint, label text, latitude double precision, longitude double precision,
		type text, status text, testing boolean, to_time timestamptz, deleted_at timestamptz)`)
	exec(`INSERT INTO markers (id, relation_id, label, latitude, longitude, type, status, testing) VALUES
		(1, 7, 'closest-food', 43.77501, -79.347, 'food', 'pending', false),
		(2, 8, 'other-map',    43.77502, -79.347, 'walk', 'pending', false),
		(3, 7, 'testing',      43.77503, -79.347, 'walk', 'pending', true),
		(4, 7, 'hidden',       43.77504, -79.347, 'hidden', 'pending', false),
		(5, 7, 'visited',      43.77505, -79.347, 'walk', ?, false),
		(6, 7, 'walk',         43.77520, -79.347, 'walk', 'pending', false)`, constant.Arrived)
	oldDB := database.Connection
	database.Connection = db
	t.Cleanup(func() { database.Connection = oldDB })

	relation := dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 7}}
	base := integrationNearbySearchQuery{Latitude: 43.775, Longitude: -79.347, RadiusMeters: 3000, Limit: 1}
	check := func(q integrationNearbySearchQuery, want uint) {
		t.Helper()
		rows, err := findNearbyMarkersByDistance(relation, q, false)
		if err != nil || len(rows) != 1 || rows[0].ID != want || rows[0].DistanceMeters <= 0 {
			t.Fatalf("query %+v: wanted marker %d; got %+v (%v)", q, want, rows, err)
		}
	}
	check(base, 1)
	base.ExcludeMarkerType = "food"
	check(base, 6)
	base.ExcludeMarkerType, base.MarkerType = "", "walk"
	check(base, 6)
}
