package service

import (
	"strings"
	"testing"
	"time"
)

func TestRenderOfflineExportArtifactFormats(t *testing.T) {
	snapshot := &ExportSnapshot{
		SchemaVersion: ExportSnapshotSchemaVersion,
		GeneratedAt:   time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC),
		Timezone:      "UTC",
		Source:        ExportSnapshotSource{RelationID: 9},
	Markers: []ExportSnapshotMarker{{
			ID:          1,
			Label:       "Marker A",
			Type:        "food",
			Address:     "123 King St W, Toronto",
			Description: "Great patio",
			Website: ExportSnapshotWebsite{
				Provider: "yelp",
				Status:   offlineExportWebsiteStatusAvailable,
				URL:      "https://example.com",
			},
			EstimateTime: ExportSnapshotEstimatedTime{Display: "10m"},
		}},
		Schedules: []ExportSnapshotSchedule{{
			ID:           2,
			Label:        "Schedule A",
			Description:  "Dinner booking",
			SelectedDate: time.Date(2026, 3, 11, 8, 0, 0, 0, time.UTC),
			MarkerID:     uintPtr(1),
			EstimateTime: ExportSnapshotEstimatedTime{Display: "12m"},
		}},
	}

	textArtifact, err := renderOfflineExportArtifact(offlineExportFormatText, snapshot, "job_1")
	if err != nil {
		t.Fatalf("text render failed: %v", err)
	}
	if textArtifact.ContentType == "" {
		t.Fatalf("unexpected text artifact: %+v", textArtifact)
	}
	textBody := string(textArtifact.Content)
	if strings.Contains(textBody, "marker_id:") || strings.Contains(textBody, "- [1]") {
		t.Fatalf("text export should not expose marker ids: %s", textBody)
	}
	for _, expected := range []string{"marker_type: food", "address: 123 King St W, Toronto", "website: https://example.com (yelp, available)", "description: Dinner booking"} {
		if !strings.Contains(textBody, expected) {
			t.Fatalf("expected text export to contain %q", expected)
		}
	}
	for _, unexpected := range []string{"# Offline Export Snapshot", "job_id:", "generated_at:", "timezone:", "relation_id:", "## Markers", "- Marker A"} {
		if strings.Contains(textBody, unexpected) {
			t.Fatalf("text export should not contain %q: %s", unexpected, textBody)
		}
	}
	if !strings.Contains(textBody, "========== Wednesday, Mar 11 2026 ==========") {
		t.Fatalf("expected text export to group schedules by date: %s", textBody)
	}

	imageArtifact, err := renderOfflineExportArtifact(offlineExportFormatImage, snapshot, "job_1")
	if err != nil {
		t.Fatalf("image render failed: %v", err)
	}
	if imageArtifact.ContentType != "image/svg+xml" || !strings.Contains(string(imageArtifact.Content), "<svg") {
		t.Fatalf("unexpected image artifact")
	}
}

func uintPtr(value uint) *uint {
	return &value
}
