package storage

import (
	"bytes"
	"context"
	"fmt"
	"log"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

type MediaStorage struct {
	cld    *cloudinary.Cloudinary
	folder string // root folder in Cloudinary (e.g. "door-images")
}

func NewMediaStorage(cloudName, apiKey, apiSecret string) (*MediaStorage, error) {
	cld, err := cloudinary.NewFromParams(cloudName, apiKey, apiSecret)
	if err != nil {
		return nil, fmt.Errorf("cloudinary init failed: %w", err)
	}

	log.Printf("[MediaStorage] Connected to Cloudinary (cloud=%s)", cloudName)
	return &MediaStorage{cld: cld, folder: "door-images"}, nil
}

// UploadImage uploads image bytes to Cloudinary and returns the public secure URL.
func (m *MediaStorage) UploadImage(ctx context.Context, objectName string, data []byte, contentType string) (string, error) {
	publicID := m.folder + "/" + objectName

	result, err := m.cld.Upload.Upload(ctx, bytes.NewReader(data), uploader.UploadParams{
		PublicID:       publicID,
		ResourceType:  "image",
		Overwrite:     boolPtr(true),
		UniqueFilename: boolPtr(false),
	})
	if err != nil {
		return "", fmt.Errorf("cloudinary upload failed: %w", err)
	}

	log.Printf("[MediaStorage] Uploaded %s → %s", objectName, result.SecureURL)
	return result.SecureURL, nil
}

// DeleteObject removes an object from Cloudinary by its object name (same key used during upload).
func (m *MediaStorage) DeleteObject(ctx context.Context, objectName string) error {
	publicID := m.folder + "/" + objectName

	_, err := m.cld.Upload.Destroy(ctx, uploader.DestroyParams{
		PublicID:     publicID,
		ResourceType: "image",
	})
	if err != nil {
		return fmt.Errorf("cloudinary delete failed: %w", err)
	}

	log.Printf("[MediaStorage] Deleted %s", publicID)
	return nil
}

// DeleteByPublicID removes an object from Cloudinary using the full public_id extracted from a URL.
func (m *MediaStorage) DeleteByPublicID(ctx context.Context, publicID string) error {
	_, err := m.cld.Upload.Destroy(ctx, uploader.DestroyParams{
		PublicID:     publicID,
		ResourceType: "image",
	})
	if err != nil {
		return fmt.Errorf("cloudinary delete failed: %w", err)
	}

	log.Printf("[MediaStorage] Deleted by public_id: %s", publicID)
	return nil
}

func boolPtr(b bool) *bool { return &b }
