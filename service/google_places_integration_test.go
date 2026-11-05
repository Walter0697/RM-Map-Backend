package service

import (
	"context"
	"crypto/sha256"
	"os"
	"testing"

	"mapmarker/backend/config"
)

// Run explicitly with RUN_GOOGLE_PLACES_INTEGRATION=1. It uses the same
// config and Infisical resolver as the production worker, but never logs the
// API key itself.
func TestGooglePlacesInfisicalIntegration(t *testing.T) {
	if os.Getenv("RUN_GOOGLE_PLACES_INTEGRATION") != "1" {
		t.Skip("set RUN_GOOGLE_PLACES_INTEGRATION=1 to call the live Google Places API")
	}

	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(".."); err != nil {
		t.Fatalf("change to backend directory: %v", err)
	}
	defer func() { _ = os.Chdir(workingDirectory) }()
	config.Init()
	setting := config.Data.Infisical
	setting.GoogleMapsAPIKey = config.Data.GoogleMaps.APIKey
	resolver := NewSecretResolver(setting, nil)
	apiKey, err := resolver.Resolve(context.Background(), "GOOGLE_MAPS_API_KEY")
	if err != nil {
		t.Fatalf("resolve GOOGLE_MAPS_API_KEY: %v", err)
	}
	digest := sha256.Sum256([]byte(apiKey))
	t.Logf("Google Places endpoint=%s infisical_environment=%s infisical_path=%s key_sha256=%x", config.Data.GoogleMaps.BaseURL, config.Data.Infisical.Environment, config.Data.Infisical.SecretPath, digest[:])

	provider := NewGooglePlacesProvider(config.Data.GoogleMaps, resolver, nil)
	places, err := provider.Search(context.Background(), "Chateau Juju Kagurazaka Tokyo")
	if err != nil {
		t.Fatalf("Google Places live request failed (key_sha256=%x): %v", digest[:], err)
	}
	t.Logf("Google Places returned %d candidates", len(places))
	if len(places) == 0 {
		t.Log("Google accepted the request but returned no candidates")
	}
}
