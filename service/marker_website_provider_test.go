package service

import (
	"mapmarker/backend/config"
	"mapmarker/backend/constant"
	"mapmarker/backend/database/dbmodel"
	"os"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
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

func TestApplyTabelogJSONLD(t *testing.T) {
	html := `<html><head><script type="application/ld+json">{"@context":"http://schema.org","@type":"Restaurant","@id":"https://tabelog.com/en/tokyo/A1301/A130103/13294162/","name":"Sushi Dokoro Isseki Sanchou","image":"https://img.example/test.jpg","address":{"@type":"PostalAddress","streetAddress":"Shinbashi","addressLocality":"Minato","addressRegion":"Tokyo","postalCode":"1050004","addressCountry":"JP"},"priceRange":"JPY 10,000～JPY 14,999","servesCuisine":"Sushi,Seafood","telephone":"+81-3-6435-9959","aggregateRating":{"@type":"AggregateRating","ratingCount":"467","ratingValue":"3.54"}}</script></head><body></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("failed to parse html: %v", err)
	}

	restaurant := &dbmodel.Restaurant{}
	applyTabelogJSONLD(restaurant, doc)

	if restaurant.Name != "Sushi Dokoro Isseki Sanchou" {
		t.Fatalf("unexpected name: %s", restaurant.Name)
	}
	if restaurant.RestaurantType != "Sushi,Seafood" {
		t.Fatalf("unexpected cuisine: %s", restaurant.RestaurantType)
	}
	if restaurant.Rating != "3.54 (467 reviews)" {
		t.Fatalf("unexpected rating: %s", restaurant.Rating)
	}
	if restaurant.Address == "" || !strings.Contains(restaurant.Address, "Tokyo") {
		t.Fatalf("unexpected address: %s", restaurant.Address)
	}
}

func TestResolveYelpAPIKey(t *testing.T) {
	originalConfigKey := config.Data.APIKEY.Yelp
	originalEnvValue, hadEnv := os.LookupEnv("YELP_API_KEY")
	defer func() {
		config.Data.APIKEY.Yelp = originalConfigKey
		if hadEnv {
			_ = os.Setenv("YELP_API_KEY", originalEnvValue)
		} else {
			_ = os.Unsetenv("YELP_API_KEY")
		}
	}()

	config.Data.APIKEY.Yelp = ""
	_ = os.Setenv("YELP_API_KEY", "env-key")
	if got := resolveYelpAPIKey(); got != "env-key" {
		t.Fatalf("expected env fallback key, got %q", got)
	}

	config.Data.APIKEY.Yelp = "config-key"
	_ = os.Setenv("YELP_API_KEY", "env-key-2")
	if got := resolveYelpAPIKey(); got != "config-key" {
		t.Fatalf("expected config key to take precedence, got %q", got)
	}
}
