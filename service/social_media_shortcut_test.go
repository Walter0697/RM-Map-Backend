package service

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateSocialMediaShortcutSetupTokenCreatesUniqueURLSafeValues(t *testing.T) {
	first, err := generateSocialMediaShortcutSetupToken()
	if err != nil {
		t.Fatalf("generate first token: %v", err)
	}
	second, err := generateSocialMediaShortcutSetupToken()
	if err != nil {
		t.Fatalf("generate second token: %v", err)
	}
	if first == second {
		t.Fatal("expected setup tokens to be unique")
	}
	if len(first) < 43 || strings.ContainsAny(first, "+/=") {
		t.Fatalf("expected URL-safe token, got %q", first)
	}
}

func TestSocialMediaShortcutSetupTokenExpiryIsTenMinutes(t *testing.T) {
	now := time.Date(2026, 11, 5, 1, 2, 3, 0, time.UTC)
	if got, want := socialMediaShortcutSetupTokenExpiry(now), now.Add(10*time.Minute); !got.Equal(want) {
		t.Fatalf("expected expiry %s, got %s", want, got)
	}
}
