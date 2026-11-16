package utils

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveImageFromURLNormalizesMismatchedImageExtension(t *testing.T) {
	imageBytes := bytes.NewBuffer(nil)
	source := image.NewRGBA(image.Rect(0, 0, 2, 2))
	source.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := jpeg.Encode(imageBytes, source, nil); err != nil {
		t.Fatalf("encode source image: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(imageBytes.Bytes())
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "image.png")
	if err := SaveImageFromURL(path, server.URL+"/image.png"); err != nil {
		t.Fatalf("save mismatched image: %v", err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open saved image: %v", err)
	}
	defer file.Close()
	if _, format, err := image.Decode(file); err != nil || format != "png" {
		t.Fatalf("expected normalized PNG, format=%q err=%v", format, err)
	}
}

func TestSaveImageFromURLRejectsNonImageResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>blocked</html>"))
	}))
	defer server.Close()

	err := SaveImageFromURL(filepath.Join(t.TempDir(), "image.png"), server.URL+"/image.png")
	if err == nil {
		t.Fatal("expected non-image response to fail")
	}
}
