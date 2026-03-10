package service

import (
	"encoding/json"
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
	case offlineExportFormatNotion:
		return renderOfflineExportNotion(snapshot, jobID)
	default:
		return offlineRenderedArtifact{}, fmt.Errorf("unsupported format: %s", format)
	}
}

func renderOfflineExportText(snapshot *ExportSnapshot, jobID string) offlineRenderedArtifact {
	builder := &strings.Builder{}
	builder.WriteString("# Offline Export Snapshot\n")
	builder.WriteString(fmt.Sprintf("job_id: %s\n", jobID))
	builder.WriteString(fmt.Sprintf("generated_at: %s\n", snapshot.GeneratedAt.Format(timeRFC3339Milli)))
	builder.WriteString(fmt.Sprintf("timezone: %s\n", snapshot.Timezone))
	builder.WriteString(fmt.Sprintf("relation_id: %d\n", snapshot.Source.RelationID))
	builder.WriteString("\n## Markers\n")
	if len(snapshot.Markers) == 0 {
		builder.WriteString("- none\n")
	}
	for _, marker := range snapshot.Markers {
		builder.WriteString(fmt.Sprintf("- [%d] %s\n", marker.ID, marker.Label))
		builder.WriteString(fmt.Sprintf("  website: provider=%s status=%s url=%s\n", marker.Website.Provider, marker.Website.Status, marker.Website.URL))
		builder.WriteString(fmt.Sprintf("  estimate_time: %s\n", marker.EstimateTime.Display))
	}
	builder.WriteString("\n## Schedules\n")
	if len(snapshot.Schedules) == 0 {
		builder.WriteString("- none\n")
	}
	for _, schedule := range snapshot.Schedules {
		builder.WriteString(fmt.Sprintf("- [%d] %s at %s\n", schedule.ID, schedule.Label, schedule.SelectedDate.Format(timeRFC3339Milli)))
		builder.WriteString(fmt.Sprintf("  marker_id: %s\n", formatNullableUint(schedule.MarkerID)))
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

func renderOfflineExportNotion(snapshot *ExportSnapshot, jobID string) (offlineRenderedArtifact, error) {
	type notionBlock struct {
		Type    string                 `json:"type"`
		Payload map[string]interface{} `json:"payload"`
	}
	type notionDocument struct {
		JobID       string        `json:"job_id"`
		GeneratedAt string        `json:"generated_at"`
		Timezone    string        `json:"timezone"`
		Blocks      []notionBlock `json:"blocks"`
	}

	blocks := make([]notionBlock, 0, len(snapshot.Markers)+len(snapshot.Schedules)+2)
	blocks = append(blocks, notionBlock{Type: "header", Payload: map[string]interface{}{
		"title": "Offline Export Snapshot",
		"jobId": jobID,
	}})
	blocks = append(blocks, notionBlock{Type: "metadata", Payload: map[string]interface{}{
		"schema_version": snapshot.SchemaVersion,
		"relation_id":    snapshot.Source.RelationID,
	}})
	for _, marker := range snapshot.Markers {
		blocks = append(blocks, notionBlock{Type: "marker", Payload: map[string]interface{}{
			"id":             marker.ID,
			"label":          marker.Label,
			"website_status": marker.Website.Status,
			"website_url":    marker.Website.URL,
			"estimate_time":  marker.EstimateTime.Display,
		}})
	}
	for _, schedule := range snapshot.Schedules {
		blocks = append(blocks, notionBlock{Type: "schedule", Payload: map[string]interface{}{
			"id":            schedule.ID,
			"label":         schedule.Label,
			"selected_date": schedule.SelectedDate.Format(timeRFC3339Milli),
			"estimate_time": schedule.EstimateTime.Display,
		}})
	}

	document := notionDocument{
		JobID:       jobID,
		GeneratedAt: snapshot.GeneratedAt.Format(timeRFC3339Milli),
		Timezone:    snapshot.Timezone,
		Blocks:      blocks,
	}
	content, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return offlineRenderedArtifact{}, err
	}
	return offlineRenderedArtifact{
		FileName:    fmt.Sprintf("%s.notion.json", jobID),
		ContentType: "application/json",
		Content:     content,
	}, nil
}

const timeRFC3339Milli = "2006-01-02T15:04:05.000Z07:00"

func formatNullableUint(value *uint) string {
	if value == nil {
		return "null"
	}
	return fmt.Sprintf("%d", *value)
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
