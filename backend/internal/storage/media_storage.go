package storage

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MediaStorage struct {
	client   *minio.Client
	bucket   string
	endpoint string
}

func NewMediaStorage(endpoint, accessKey, secretKey, bucket string) (*MediaStorage, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client init failed: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("minio bucket check failed: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("minio make bucket failed: %w", err)
		}
		log.Printf("[MediaStorage] Created bucket %q", bucket)
	}

	log.Printf("[MediaStorage] Connected to MinIO at %s, bucket=%s", endpoint, bucket)
	return &MediaStorage{client: client, bucket: bucket, endpoint: endpoint}, nil
}

// UploadImage stores image bytes in MinIO and returns the object URL.
func (m *MediaStorage) UploadImage(ctx context.Context, objectName string, data []byte, contentType string) (string, error) {
	reader := bytes.NewReader(data)
	_, err := m.client.PutObject(ctx, m.bucket, objectName, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("minio upload failed: %w", err)
	}

	url := fmt.Sprintf("http://%s/%s/%s", m.endpoint, m.bucket, objectName)
	return url, nil
}
