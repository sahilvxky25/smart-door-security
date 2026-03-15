package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type FaceService struct {
	baseURL string
	client  *http.Client
}

type FaceRecognitionResponse struct {
	Match bool   `json:"match"`
	User  string `json:"user"`
	Spoof bool   `json:"spoof"`
	Frame string `json:"frame"`
}

type FaceRecognitionResult struct {
	Match    bool
	User     string
	Spoof    bool
	FrameJPG []byte
}

func NewFaceService(baseURL string) *FaceService {
	return &FaceService{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second, // longer timeout — video capture + anti-spoof takes ~3-5s
		},
	}
}

// CaptureAndRecognize tells the Python face service to capture video,
// run anti-spoof liveness checks, then face recognition.
func (f *FaceService) CaptureAndRecognize() (*FaceRecognitionResult, error) {
	url := f.baseURL + "/capture-and-recognize"

	resp, err := f.client.Post(url, "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("face service unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("face service returned status %d: %s", resp.StatusCode, string(body))
	}

	var raw FaceRecognitionResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode face service response: %w", err)
	}

	result := &FaceRecognitionResult{
		Match: raw.Match,
		User:  raw.User,
		Spoof: raw.Spoof,
	}

	if raw.Frame != "" {
		frameBytes, err := base64.StdEncoding.DecodeString(raw.Frame)
		if err != nil {
			log.Printf("[FaceService] Warning: failed to decode frame base64: %v", err)
		} else {
			result.FrameJPG = frameBytes
		}
	}

	log.Printf("[FaceService] match=%v user=%q spoof=%v frame_size=%d bytes",
		result.Match, result.User, result.Spoof, len(result.FrameJPG))
	return result, nil
}

// Recognize sends a pre-captured image to the face service for recognition.
func (f *FaceService) Recognize(image []byte) (bool, string, error) {
	url := f.baseURL + "/recognize"

	req, err := http.NewRequest("POST", url, bytes.NewReader(image))
	if err != nil {
		return false, "", err
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := f.client.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("face service unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("face service returned status %d", resp.StatusCode)
	}

	var result FaceRecognitionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, "", fmt.Errorf("failed to decode face service response: %w", err)
	}

	log.Printf("[FaceService] match=%v user=%q", result.Match, result.User)
	return result.Match, result.User, nil
}
