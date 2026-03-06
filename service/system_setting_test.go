package service

import "testing"

func TestValidateIOSShortcutInstallURL(t *testing.T) {
	t.Run("accepts absolute https URL", func(t *testing.T) {
		value, ok := ValidateIOSShortcutInstallURL("https://www.icloud.com/shortcuts/example")
		if !ok {
			t.Fatalf("expected URL to be valid")
		}
		if value != "https://www.icloud.com/shortcuts/example" {
			t.Fatalf("unexpected normalized value: %s", value)
		}
	})

	t.Run("rejects non-http scheme", func(t *testing.T) {
		if _, ok := ValidateIOSShortcutInstallURL("ftp://example.com"); ok {
			t.Fatalf("expected ftp URL to be invalid")
		}
	})

	t.Run("rejects relative URL", func(t *testing.T) {
		if _, ok := ValidateIOSShortcutInstallURL("/shortcuts/example"); ok {
			t.Fatalf("expected relative URL to be invalid")
		}
	})

	t.Run("rejects blank value", func(t *testing.T) {
		if _, ok := ValidateIOSShortcutInstallURL("   "); ok {
			t.Fatalf("expected blank URL to be invalid")
		}
	})
}

func TestValidateScheduleTravelThresholds(t *testing.T) {
	t.Run("accepts valid threshold ordering", func(t *testing.T) {
		if !ValidateScheduleTravelThresholds(20, 45) {
			t.Fatalf("expected thresholds to be valid")
		}
	})

	t.Run("rejects non-positive values", func(t *testing.T) {
		if ValidateScheduleTravelThresholds(0, 45) {
			t.Fatalf("expected zero easy threshold to be invalid")
		}
		if ValidateScheduleTravelThresholds(20, -1) {
			t.Fatalf("expected negative difficult threshold to be invalid")
		}
	})

	t.Run("rejects equal or reversed ordering", func(t *testing.T) {
		if ValidateScheduleTravelThresholds(45, 45) {
			t.Fatalf("expected equal thresholds to be invalid")
		}
		if ValidateScheduleTravelThresholds(60, 45) {
			t.Fatalf("expected easy threshold above difficult threshold to be invalid")
		}
	})
}
