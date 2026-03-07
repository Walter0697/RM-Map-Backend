package service

import (
	"mapmarker/backend/constant"
	"testing"
)

func TestGetMarkerWebsiteProvider(t *testing.T) {
	openrice, ok := GetMarkerWebsiteProvider(constant.Openrice)
	if !ok || openrice == nil {
		t.Fatalf("expected openrice provider to exist")
	}
	yelp, ok := GetMarkerWebsiteProvider(constant.Yelp)
	if !ok || yelp == nil {
		t.Fatalf("expected yelp provider to exist")
	}
	tabelog, ok := GetMarkerWebsiteProvider(constant.Tabelog)
	if !ok || tabelog == nil {
		t.Fatalf("expected tabelog provider to exist")
	}
}

func TestValidateMarkerWebsiteIntegration(t *testing.T) {
	if err := ValidateMarkerWebsiteIntegration("", ""); err != nil {
		t.Fatalf("expected empty integration payload to be valid: %v", err)
	}
	if err := ValidateMarkerWebsiteIntegration(constant.Yelp, "north-york-cafe"); err != nil {
		t.Fatalf("expected valid yelp id: %v", err)
	}
	if err := ValidateMarkerWebsiteIntegration(constant.Yelp, "bad/id"); err == nil {
		t.Fatalf("expected invalid yelp id to fail")
	}
	if err := ValidateMarkerWebsiteIntegration(constant.Tabelog, "tokyo/A1304/A130401/13000001"); err != nil {
		t.Fatalf("expected valid tabelog id: %v", err)
	}
	if err := ValidateMarkerWebsiteIntegration(constant.Tabelog, "https://tabelog.com/tokyo/A1304"); err == nil {
		t.Fatalf("expected invalid tabelog id to fail")
	}
	if err := ValidateMarkerWebsiteIntegration("unknown-provider", "abc123"); err == nil {
		t.Fatalf("expected unknown provider to fail")
	}
}
