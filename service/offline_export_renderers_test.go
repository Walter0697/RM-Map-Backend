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
			ID:    1,
			Label: "Marker A",
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
			SelectedDate: time.Date(2026, 3, 11, 8, 0, 0, 0, time.UTC),
			EstimateTime: ExportSnapshotEstimatedTime{Display: "12m"},
		}},
	}

	textArtifact, err := renderOfflineExportArtifact(offlineExportFormatText, snapshot, "job_1")
	if err != nil {
		t.Fatalf("text render failed: %v", err)
	}
	if textArtifact.ContentType == "" || !strings.Contains(string(textArtifact.Content), "Offline Export Snapshot") {
		t.Fatalf("unexpected text artifact: %+v", textArtifact)
	}

	imageArtifact, err := renderOfflineExportArtifact(offlineExportFormatImage, snapshot, "job_1")
	if err != nil {
		t.Fatalf("image render failed: %v", err)
	}
	if imageArtifact.ContentType != "image/svg+xml" || !strings.Contains(string(imageArtifact.Content), "<svg") {
		t.Fatalf("unexpected image artifact")
	}

	notionArtifact, err := renderOfflineExportArtifact(offlineExportFormatNotion, snapshot, "job_1")
	if err != nil {
		t.Fatalf("notion render failed: %v", err)
	}
	if notionArtifact.ContentType != "application/json" || !strings.Contains(string(notionArtifact.Content), "\"blocks\"") {
		t.Fatalf("unexpected notion artifact")
	}
}
