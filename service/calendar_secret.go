package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"mapmarker/backend/config"
	"strings"
)

const calendarTokenPrefix = "enc.v1:"

func encryptCalendarSecret(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	key := deriveCalendarSecretKey()
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(trimmed), nil)
	return calendarTokenPrefix + base64.RawStdEncoding.EncodeToString(ciphertext), nil
}

func decryptCalendarSecret(encoded string) (string, error) {
	trimmed := strings.TrimSpace(encoded)
	if trimmed == "" {
		return "", nil
	}
	if !strings.HasPrefix(trimmed, calendarTokenPrefix) {
		return "", fmt.Errorf("unsupported calendar secret format")
	}
	key := deriveCalendarSecretKey()
	rawCiphertext, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(trimmed, calendarTokenPrefix))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(rawCiphertext) < nonceSize {
		return "", fmt.Errorf("invalid calendar secret payload")
	}
	nonce := rawCiphertext[:nonceSize]
	ciphertext := rawCiphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func deriveCalendarSecretKey() [32]byte {
	material := strings.TrimSpace(config.Data.App.JWT)
	if material == "" {
		material = "calendar-sync-fallback-key"
	}
	return sha256.Sum256([]byte(material))
}
