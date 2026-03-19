package service

import (
	"encoding/json"
	"fmt"
	"log"
	"mapmarker/backend/config"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	integrationHashtagCacheTTL      = 10 * time.Minute
	integrationHashtagCacheVersion  = "v2"
	integrationHashtagCacheTypeFlag = "testing"
)

var integrationHashtagMatcher = regexp.MustCompile(`#([^\s]*)`)
var integrationHashtagCacheClientFactory = newRawRedisClient

type integrationHashtagItem struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

type integrationHashtagCachePayload struct {
	Items []integrationHashtagItem `json:"items"`
}

func parseIntegrationHashtags(description string) []string {
	if strings.TrimSpace(description) == "" {
		return []string{}
	}

	matches := integrationHashtagMatcher.FindAllString(description, -1)
	if len(matches) == 0 {
		return []string{}
	}

	items := make([]string, 0, len(matches))
	for _, tag := range matches {
		tag = strings.TrimPrefix(strings.TrimSpace(tag), "#")
		if strings.HasSuffix(tag, ":") {
			tag = strings.TrimSuffix(tag, ":")
		}
		if tag != "" {
			items = append(items, tag)
		}
	}
	return items
}

func getIntegrationHashtagItems(relationID uint, includeTesting bool) ([]integrationHashtagItem, error) {
	if cached, ok := loadIntegrationHashtagCache(relationID, includeTesting); ok {
		return cached, nil
	}

	query := database.Connection.Model(&dbmodel.Marker{}).Select("description")
	query = query.Where("relation_id = ?", relationID)
	// Hashtag discovery should cover all relation markers, not only active/visible subsets.
	if !includeTesting {
		query = query.Where("testing = ?", false)
	}

	markers := make([]dbmodel.Marker, 0)
	if err := query.Find(&markers).Error; err != nil {
		return nil, err
	}

	counter := map[string]int{}
	for _, marker := range markers {
		for _, tag := range parseIntegrationHashtags(marker.Description) {
			counter[tag] += 1
		}
	}

	items := make([]integrationHashtagItem, 0, len(counter))
	for tag, count := range counter {
		items = append(items, integrationHashtagItem{
			Tag:   tag,
			Count: count,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return strings.ToLower(items[i].Tag) < strings.ToLower(items[j].Tag)
		}
		return items[i].Count > items[j].Count
	})

	storeIntegrationHashtagCache(relationID, includeTesting, items)
	return items, nil
}

func invalidateIntegrationHashtagCacheForRelation(relationID uint) {
	if relationID == 0 || !config.Data.Redis.Enable {
		return
	}

	client := integrationHashtagCacheClientFactory()
	for _, includeTesting := range []bool{false, true} {
		key := buildIntegrationHashtagCacheKey(relationID, includeTesting)
		if err := client.Del(key); err != nil {
			log.Printf("integration hashtags cache invalidate failed key=%s err=%v", key, err)
		}
	}
}

func loadIntegrationHashtagCache(relationID uint, includeTesting bool) ([]integrationHashtagItem, bool) {
	if !config.Data.Redis.Enable {
		return nil, false
	}

	client := integrationHashtagCacheClientFactory()
	key := buildIntegrationHashtagCacheKey(relationID, includeTesting)
	payload, exists, err := client.Get(key)
	if err != nil {
		log.Printf("integration hashtags cache get failed key=%s err=%v", key, err)
		return nil, false
	}
	if !exists {
		return nil, false
	}

	cachePayload := integrationHashtagCachePayload{}
	if err := json.Unmarshal([]byte(payload), &cachePayload); err != nil {
		log.Printf("integration hashtags cache decode failed key=%s err=%v", key, err)
		return nil, false
	}
	return cachePayload.Items, true
}

func storeIntegrationHashtagCache(relationID uint, includeTesting bool, items []integrationHashtagItem) {
	if !config.Data.Redis.Enable {
		return
	}

	client := integrationHashtagCacheClientFactory()
	key := buildIntegrationHashtagCacheKey(relationID, includeTesting)
	payload, err := json.Marshal(integrationHashtagCachePayload{Items: items})
	if err != nil {
		log.Printf("integration hashtags cache encode failed key=%s err=%v", key, err)
		return
	}
	if err := client.SetEX(key, string(payload), integrationHashtagCacheTTL); err != nil {
		log.Printf("integration hashtags cache set failed key=%s err=%v", key, err)
	}
}

func buildIntegrationHashtagCacheKey(relationID uint, includeTesting bool) string {
	testingFlag := 0
	if includeTesting {
		testingFlag = 1
	}
	return fmt.Sprintf(
		"integration:hashtags:%s:relation:%d:%s:%d",
		integrationHashtagCacheVersion,
		relationID,
		integrationHashtagCacheTypeFlag,
		testingFlag,
	)
}
