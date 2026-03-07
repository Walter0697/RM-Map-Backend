package main

import (
	"fmt"
	"mapmarker/backend/config"
	"mapmarker/backend/database"
	"mapmarker/backend/service"
)

func main() {
	config.Init()
	database.Init()
	updated, err := service.BackfillLegacyReleaseNotes()
	if err != nil {
		panic(err)
	}
	fmt.Printf("backfill complete, updated=%d\n", updated)
}
