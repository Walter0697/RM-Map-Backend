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
