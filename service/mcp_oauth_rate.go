package service

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type mcpRateEntry struct {
	count int
	until time.Time
}

var mcpRateState = struct {
	sync.Mutex
	entries map[string]mcpRateEntry
}{entries: make(map[string]mcpRateEntry)}

// Conservative per-process defense. Use a shared edge limit for multi-replica
// installations. Forwarded headers are intentionally not trusted here.
func mcpRateLimit(w http.ResponseWriter, r *http.Request, operation string, limit int) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	now := time.Now()
	key := operation + ":" + host
	mcpRateState.Lock()
	for k, v := range mcpRateState.entries {
		if !v.until.After(now) {
			delete(mcpRateState.entries, k)
		}
	}
	entry, exists := mcpRateState.entries[key]
	if !exists {
		entry = mcpRateEntry{until: now.Add(time.Minute)}
	}
	denied := entry.count >= limit || (!exists && len(mcpRateState.entries) >= 4096)
	if !denied {
		entry.count++
		mcpRateState.entries[key] = entry
	}
	mcpRateState.Unlock()
	if denied {
		w.Header().Set("Retry-After", "60")
		mcpError(w, http.StatusTooManyRequests, "temporarily_unavailable")
		return false
	}
	return true
}
