package service

import "testing"

func TestParseIntegrationHashtags(t *testing.T) {
	items := parseIntegrationHashtags("go to #food #museum: then #food again")
	if len(items) != 3 {
		t.Fatalf("expected 3 hashtags, got %d (%v)", len(items), items)
	}
	if items[0] != "food" || items[1] != "museum" || items[2] != "food" {
		t.Fatalf("unexpected hashtags parsed: %v", items)
	}
}

func TestParseIntegrationHashtagsEmpty(t *testing.T) {
	items := parseIntegrationHashtags("")
	if len(items) != 0 {
		t.Fatalf("expected empty hashtags, got %v", items)
	}
}

