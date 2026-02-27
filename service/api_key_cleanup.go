package service

import (
	"log"
	"mapmarker/backend/config"
	"time"
)

func StartAPIKeyCleanupWorker() {
	if !config.Data.IntegrationAuth.EnableCleanup {
		return
	}

	interval := time.Duration(config.Data.IntegrationAuth.CleanupIntervalHours) * time.Hour
	if interval <= 0 {
		interval = 24 * time.Hour
	}

	go func() {
		// Run once during startup so limits are quickly enforced.
		if err := CleanupOldAPIKeyData(); err != nil {
			log.Printf("api key cleanup failed at startup: %v", err)
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			if err := CleanupOldAPIKeyData(); err != nil {
				log.Printf("api key cleanup failed: %v", err)
			}
		}
	}()
}
