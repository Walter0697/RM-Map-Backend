package utils

import "testing"

func TestPasswordHashRoundTrip(t *testing.T) {
	password := "s3cure-password"

	hashed, err := GenerateHashedPassword(password)
	if err != nil {
		t.Fatalf("GenerateHashedPassword returned error: %v", err)
	}

	if hashed == password {
		t.Fatalf("expected hashed password to differ from plaintext")
	}

	if !CompareHash(hashed, password) {
		t.Fatalf("expected CompareHash to validate original password")
	}

	if CompareHash(hashed, "wrong-password") {
		t.Fatalf("expected CompareHash to reject wrong password")
	}
}
