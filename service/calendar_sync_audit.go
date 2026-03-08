package service

import (
	"log"
	"sort"
	"strings"
)

func calendarAudit(event string, fields map[string]string) {
	parts := make([]string, 0, len(fields)+1)
	parts = append(parts, "event="+strings.TrimSpace(event))
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts = append(parts, key+"="+strings.TrimSpace(fields[key]))
	}
	log.Printf("[calendar-sync-audit] %s", strings.Join(parts, " "))
}
