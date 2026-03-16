package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MediaStorage struct {
	client        *minio.Client
	bucket        string
	endpoint      string
	publicBaseURL string
}

func NewMediaStorage(endpoint, accessKey, secretKey, bucket, publicBaseURL string) (*MediaStorage, error) {
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

	// Set bucket policy to public read so objects can be fetched without auth
	policy := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, bucket)
	if err := client.SetBucketPolicy(ctx, bucket, policy); err != nil {
		log.Printf("[MediaStorage] Warning: could not set public-read policy: %v", err)
	}

	log.Printf("[MediaStorage] Connected to MinIO at %s, bucket=%s", endpoint, bucket)
	return &MediaStorage{client: client, bucket: bucket, endpoint: endpoint, publicBaseURL: strings.TrimRight(publicBaseURL, "/")}, nil
}

// UploadImage stores image bytes in MinIO and returns a backend proxy URL.
func (m *MediaStorage) UploadImage(ctx context.Context, objectName string, data []byte, contentType string) (string, error) {
	reader := bytes.NewReader(data)
	_, err := m.client.PutObject(ctx, m.bucket, objectName, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("minio upload failed: %w", err)
	}

	url := fmt.Sprintf("%s/images/%s", m.publicBaseURL, objectName)
	return url, nil
}

// DeleteObject removes an object from MinIO. Best-effort — logs but does not return error.
func (m *MediaStorage) DeleteObject(ctx context.Context, objectName string) error {
	err := m.client.RemoveObject(ctx, m.bucket, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("minio delete object failed: %w", err)
	}
	return nil
}
func (m *MediaStorage) GetObject(ctx context.Context, objectName string) (io.ReadCloser, string, error) {
	obj, err := m.client.GetObject(ctx, m.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("minio get object failed: %w", err)
	}

	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, "", fmt.Errorf("minio stat object failed: %w", err)
	}

	contentType := info.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return obj, contentType, nil
}
