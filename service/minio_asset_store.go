package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"mapmarker/backend/config"
)

type minioAssetUploader interface {
	PutObject(context.Context, string, string, []byte, string) error
}

type minioAssetStore struct {
	bucket         string
	maxBytes       int64
	retentionClass string
	uploader       minioAssetUploader
}

func NewMinIOAssetStore(setting config.MinIOSetting, secrets SecretResolver, client *http.Client) (AssetStore, error) {
	if !setting.Enable {
		return nil, nil
	}
	endpoint := strings.TrimSpace(setting.Endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("MinIO endpoint is required")
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "http://" + endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("invalid MinIO endpoint")
	}
	accessKey := strings.TrimSpace(setting.AccessKey)
	secretKey := strings.TrimSpace(setting.SecretKey)
	if secrets != nil {
		if accessKey == "" {
			accessKey, err = secrets.Resolve(context.Background(), "MINIO_ACCESS_KEY")
			if err != nil {
				return nil, err
			}
		}
		if secretKey == "" {
			secretKey, err = secrets.Resolve(context.Background(), "MINIO_SECRET_KEY")
			if err != nil {
				return nil, err
			}
		}
	}
	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("MinIO credentials are not configured")
	}
	options := &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: strings.EqualFold(parsed.Scheme, "https"),
		Region: setting.Region,
	}
	if client != nil {
		options.Transport = client.Transport
	}
	minioClient, err := minio.New(parsed.Host, options)
	if err != nil {
		return nil, fmt.Errorf("create MinIO client: %w", err)
	}
	return &minioAssetStore{
		bucket:         setting.Bucket,
		maxBytes:       setting.MaxBytes,
		retentionClass: "social-analysis-source",
		uploader:       minioClientUploader{client: minioClient},
	}, nil
}

func newTestMinIOAssetStore(setting config.MinIOSetting, uploader minioAssetUploader) (AssetStore, error) {
	if !setting.Enable {
		return nil, nil
	}
	if strings.TrimSpace(setting.Bucket) == "" || uploader == nil {
		return nil, fmt.Errorf("MinIO bucket and uploader are required")
	}
	return &minioAssetStore{bucket: setting.Bucket, maxBytes: setting.MaxBytes, retentionClass: "social-analysis-source", uploader: uploader}, nil
}

func (store *minioAssetStore) Put(ctx context.Context, key string, content io.Reader, size int64, contentType string) (StoredAsset, error) {
	if store == nil || store.uploader == nil {
		return StoredAsset{}, fmt.Errorf("MinIO asset store is not configured")
	}
	if strings.TrimSpace(key) == "" {
		return StoredAsset{}, fmt.Errorf("asset key is required")
	}
	prepared, err := prepareSocialAnalysisAsset(content, store.maxBytes, contentType)
	if err != nil {
		return StoredAsset{}, err
	}
	if size >= 0 && int64(len(prepared.Body)) != size {
		return StoredAsset{}, fmt.Errorf("asset size mismatch")
	}
	if err := store.uploader.PutObject(ctx, store.bucket, key, prepared.Body, prepared.ContentType); err != nil {
		return StoredAsset{}, fmt.Errorf("upload MinIO asset: %w", err)
	}
	return StoredAsset{Bucket: store.bucket, Key: key, ContentType: prepared.ContentType, Size: prepared.Size, SHA256: prepared.SHA256, RetentionClass: store.retentionClass}, nil
}

type minioClientUploader struct {
	client *minio.Client
}

func (uploader minioClientUploader) PutObject(ctx context.Context, bucket, key string, body []byte, contentType string) error {
	_, err := uploader.client.PutObject(ctx, bucket, key, bytes.NewReader(body), int64(len(body)), minio.PutObjectOptions{ContentType: contentType})
	return err
}
