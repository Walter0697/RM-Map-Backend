package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"mapmarker/backend/config"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

func captureSocialAnalysisArtifacts(ctx context.Context, job *dbmodel.SocialMediaAnalysisJob, post NormalizedSocialPost, sourceJSON []byte, store AssetStore) {
	if store == nil || job == nil {
		return
	}
	if _, err := putSocialAnalysisArtifact(ctx, job, store, buildSocialMediaAssetKey(job.JobID, "source", 0, "json"), bytes.NewReader(sourceJSON), int64(len(sourceJSON)), "application/json", dbmodel.SocialMediaAnalysisArtifactKindSourceJSON, "backend"); err != nil {
		log.Printf("[observability] source snapshot upload skipped job_id=%s err=%v", job.JobID, err)
	}
	if strings.TrimSpace(post.RawBody) != "" {
		rawBody := []byte(post.RawBody)
		if _, err := putSocialAnalysisArtifact(ctx, job, store, buildSocialMediaAssetKey(job.JobID, "source/raw-body", 0, "html"), bytes.NewReader(rawBody), int64(len(rawBody)), "text/html", dbmodel.SocialMediaAnalysisArtifactKindSourceJSON, "source"); err != nil {
			log.Printf("[observability] raw source upload skipped job_id=%s err=%v", job.JobID, err)
		}
	}
	for index, source := range firstSocialAnalysisMedia(post) {
		body, contentType, extension, err := downloadSocialAnalysisMediaFn(ctx, source)
		if err != nil {
			log.Printf("[observability] media snapshot skipped job_id=%s url=%q err=%v", job.JobID, source, err)
			continue
		}
		kind := dbmodel.SocialMediaAnalysisArtifactKindImage
		if index >= len(post.ImageURLs) {
			kind = dbmodel.SocialMediaAnalysisArtifactKindVideo
		}
		key := buildSocialMediaAssetKey(job.JobID, kind, index+1, extension)
		if _, err := putSocialAnalysisArtifact(ctx, job, store, key, bytes.NewReader(body), int64(len(body)), contentType, kind, "source"); err != nil {
			log.Printf("[observability] media artifact upload skipped job_id=%s err=%v", job.JobID, err)
		}
	}
}

var downloadSocialAnalysisMediaFn = downloadSocialAnalysisMedia

func firstSocialAnalysisMedia(post NormalizedSocialPost) []string {
	media := make([]string, 0, 2)
	if len(post.ImageURLs) > 0 {
		media = append(media, post.ImageURLs[0])
	}
	if len(post.VideoURLs) > 0 {
		media = append(media, post.VideoURLs[0])
	}
	return media
}

func putSocialAnalysisArtifact(ctx context.Context, job *dbmodel.SocialMediaAnalysisJob, store AssetStore, key string, content io.Reader, size int64, contentType, kind, provider string) (StoredAsset, error) {
	stored, err := store.Put(ctx, key, content, size, contentType)
	if err != nil {
		return StoredAsset{}, err
	}
	if database.Connection != nil {
		artifact := &dbmodel.SocialMediaAnalysisArtifact{
			AnalysisJobID:  job.ID,
			JobID:          job.JobID,
			ObjectKey:      stored.Key,
			Kind:           kind,
			Provider:       provider,
			ContentType:    stored.ContentType,
			SizeBytes:      stored.Size,
			SHA256:         stored.SHA256,
			RetentionClass: stored.RetentionClass,
		}
		if err := database.Connection.WithContext(ctx).Create(artifact).Error; err != nil {
			return StoredAsset{}, fmt.Errorf("persist asset metadata: %w", err)
		}
	}
	return stored, nil
}

func downloadSocialAnalysisMedia(ctx context.Context, rawURL string) ([]byte, string, string, error) {
	parsed, err := ValidatePublicSourceURL(rawURL)
	if err != nil {
		return nil, "", "", err
	}
	maxBytes := int64(25 * 1024 * 1024)
	if configured := config.Data.MinIO.MaxBytes; configured > 0 {
		maxBytes = configured
	}
	requestContext, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", "", err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, "", "", err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, "", "", fmt.Errorf("media returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return nil, "", "", err
	}
	if int64(len(body)) > maxBytes {
		return nil, "", "", fmt.Errorf("media exceeds %d bytes", maxBytes)
	}
	contentType := strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0])
	extension := "bin"
	if values, _ := mime.ExtensionsByType(contentType); len(values) > 0 {
		extension = strings.TrimPrefix(values[0], ".")
	} else if suffix := path.Ext(parsed.Path); suffix != "" {
		extension = strings.TrimPrefix(suffix, ".")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return body, contentType, extension, nil
}
