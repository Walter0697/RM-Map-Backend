package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
	"unicode"
)

type AssetStore interface {
	Put(ctx context.Context, key string, content io.Reader, size int64, contentType string) (StoredAsset, error)
}

type StoredAsset struct {
	Bucket         string
	Key            string
	ContentType    string
	Size           int64
	SHA256         string
	RetentionClass string
}

type preparedSocialAnalysisAsset struct {
	ContentType string
	Size        int64
	SHA256      string
	Body        []byte
}

func buildSocialMediaAssetKey(jobID, kind string, index int, extension string) string {
	jobID = sanitizeAssetPathSegment(jobID)
	kind = strings.Trim(strings.TrimSpace(kind), "/")
	extension = strings.TrimPrefix(strings.TrimSpace(extension), ".")
	name := kind
	if index > 0 {
		name = fmt.Sprintf("%s-%d", name, index)
	}
	if extension != "" {
		name += "." + extension
	}
	return "social-analysis/" + jobID + "/" + name
}

func sanitizeAssetPathSegment(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, char := range value {
		if unicode.IsLetter(char) || unicode.IsDigit(char) || char == '-' || char == '_' || char == '.' {
			builder.WriteRune(char)
			continue
		}
		builder.WriteByte('_')
	}
	return strings.Trim(builder.String(), "._")
}

func prepareSocialAnalysisAsset(content io.Reader, maxBytes int64, contentType string) (preparedSocialAnalysisAsset, error) {
	if content == nil {
		return preparedSocialAnalysisAsset{}, fmt.Errorf("asset content is required")
	}
	if maxBytes <= 0 {
		return preparedSocialAnalysisAsset{}, fmt.Errorf("asset size limit must be positive")
	}
	body, err := io.ReadAll(io.LimitReader(content, maxBytes+1))
	if err != nil {
		return preparedSocialAnalysisAsset{}, fmt.Errorf("read asset: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return preparedSocialAnalysisAsset{}, fmt.Errorf("asset exceeds %d bytes", maxBytes)
	}
	digest := sha256.Sum256(body)
	return preparedSocialAnalysisAsset{
		ContentType: strings.TrimSpace(contentType),
		Size:        int64(len(body)),
		SHA256:      fmt.Sprintf("%x", digest[:]),
		Body:        body,
	}, nil
}
