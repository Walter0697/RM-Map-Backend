package constant

import (
	"strings"
	"testing"
)

func TestGetImageLinkNameUsesURLPathExtension(t *testing.T) {
	imageURL := "https://scontent.example.com/v/t51/photo.jpg?nc_ht=scontent.example.com&oe=6AF06D09"
	name := GetImageLinkName(MarkerPreviewPath, imageURL)
	if !strings.HasSuffix(name, ".jpg") {
		t.Fatalf("expected image filename to preserve .jpg path extension, got %q", name)
	}
}
