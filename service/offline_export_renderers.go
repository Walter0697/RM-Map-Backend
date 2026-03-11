package service

import (
	"fmt"
	"strings"
)

const offlineExportImageTemplateVersion = "v1"

type offlineRenderedArtifact struct {
	FileName    string
	ContentType string
	Content     []byte
}

func renderOfflineExportArtifact(format string, snapshot *ExportSnapshot, jobID string) (offlineRenderedArtifact, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case offlineExportFormatText:
		return renderOfflineExportText(snapshot, jobID), nil
	case offlineExportFormatImage:
		return renderOfflineExportImage(snapshot, jobID), nil
	default:
		return offlineRenderedArtifact{}, fmt.Errorf("unsupported format: %s", format)
	}
}

func renderOfflineExportText(snapshot *ExportSnapshot, jobID string) offlineRenderedArtifact {
	markerByID := buildExportMarkerLookup(snapshot.Markers)
	builder := &strings.Builder{}
	if len(snapshot.Schedules) == 0 {
		builder.WriteString("- none\n")
	}
	lastDateKey := ""
	for _, schedule := range snapshot.Schedules {
		dateKey := schedule.SelectedDate.Format("2006-01-02")
		if dateKey != lastDateKey {
			if lastDateKey != "" {
				builder.WriteString("\n")
			}
			builder.WriteString(fmt.Sprintf("========== %s ==========\n", schedule.SelectedDate.Format("Monday, Jan 2 2006")))
			lastDateKey = dateKey
		} else {
			builder.WriteString("\n")
		}
		linkedMarker := exportMarkerForSchedule(schedule, markerByID)
		builder.WriteString(fmt.Sprintf("- %s at %s\n", schedule.Label, schedule.SelectedDate.Format(timeRFC3339Milli)))
		builder.WriteString(fmt.Sprintf("  marker_type: %s\n", fallbackExportValue(linkedMarker.Type)))
		builder.WriteString(fmt.Sprintf("  address: %s\n", fallbackExportValue(linkedMarker.Address)))
		builder.WriteString(fmt.Sprintf("  website: %s\n", formatWebsiteForExport(linkedMarker.Website)))
		builder.WriteString(fmt.Sprintf("  description: %s\n", fallbackExportValue(firstNonEmptyExportValue(schedule.Description, linkedMarker.Description))))
		builder.WriteString(fmt.Sprintf("  estimate_time: %s\n", schedule.EstimateTime.Display))
	}
	return offlineRenderedArtifact{
		FileName:    fmt.Sprintf("%s.txt", jobID),
		ContentType: "text/plain; charset=utf-8",
		Content:     []byte(builder.String()),
	}
}

func renderOfflineExportImage(snapshot *ExportSnapshot, jobID string) offlineRenderedArtifact {
	safeJobID := escapeXML(jobID)
	safeGeneratedAt := escapeXML(snapshot.GeneratedAt.Format(timeRFC3339Milli))
	safeTimezone := escapeXML(snapshot.Timezone)
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="630" viewBox="0 0 1200 630">
  <rect width="1200" height="630" fill="#f6f7f9"/>
  <rect x="32" y="32" width="1136" height="566" rx="20" fill="#ffffff" stroke="#d9dde4"/>
  <text x="80" y="120" font-size="42" font-family="Verdana">Offline Export Snapshot</text>
  <text x="80" y="180" font-size="24" font-family="Verdana">Template: %s</text>
  <text x="80" y="220" font-size="24" font-family="Verdana">Job: %s</text>
  <text x="80" y="260" font-size="24" font-family="Verdana">Generated: %s</text>
  <text x="80" y="300" font-size="24" font-family="Verdana">Timezone: %s</text>
  <text x="80" y="340" font-size="24" font-family="Verdana">Markers: %d</text>
  <text x="80" y="380" font-size="24" font-family="Verdana">Schedules: %d</text>
</svg>`,
		offlineExportImageTemplateVersion,
		safeJobID,
		safeGeneratedAt,
		safeTimezone,
		len(snapshot.Markers),
		len(snapshot.Schedules),
	)
	return offlineRenderedArtifact{
		FileName:    fmt.Sprintf("%s.svg", jobID),
		ContentType: "image/svg+xml",
		Content:     []byte(svg),
	}
}

const timeRFC3339Milli = "2006-01-02T15:04:05.000Z07:00"

func formatNullableUint(value *uint) string {
	if value == nil {
		return "null"
	}
	return fmt.Sprintf("%d", *value)
}

func buildExportMarkerLookup(markers []ExportSnapshotMarker) map[uint]ExportSnapshotMarker {
	result := make(map[uint]ExportSnapshotMarker, len(markers))
	for _, marker := range markers {
		result[marker.ID] = marker
	}
	return result
}

func exportMarkerForSchedule(schedule ExportSnapshotSchedule, markerByID map[uint]ExportSnapshotMarker) ExportSnapshotMarker {
	if schedule.MarkerID == nil {
		return ExportSnapshotMarker{}
	}
	return markerByID[*schedule.MarkerID]
}

func formatWebsiteForExport(website ExportSnapshotWebsite) string {
	if strings.TrimSpace(website.URL) == "" {
		return fmt.Sprintf("status=%s", fallbackExportValue(website.Status))
	}
	if strings.TrimSpace(website.Provider) == "" || strings.EqualFold(strings.TrimSpace(website.Provider), "unknown") {
		return website.URL
	}
	return fmt.Sprintf("%s (%s, %s)", website.URL, website.Provider, fallbackExportValue(website.Status))
}

func fallbackExportValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "N/A"
	}
	return trimmed
}

func firstNonEmptyExportValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func escapeXML(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}
