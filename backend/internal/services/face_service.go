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
	Match    bool    `json:"match"`
	User     string  `json:"user"`
	UserID   string  `json:"user_id"`
	MemberID string  `json:"member_id"`
	Score    float64 `json:"score"`
	Spoof    bool    `json:"spoof"`
	Frame    string  `json:"frame"`
}

type FaceEmbeddingResponse struct {
	UserID    string    `json:"user_id"`
	MemberID  string    `json:"member_id"`
	Name      string    `json:"name"`
	Embedding []float64 `json:"embedding"`
}

type FaceCandidate struct {
	MemberID  uint      `json:"member_id"`
	Name      string    `json:"name"`
	Embedding []float64 `json:"embedding"`
}

type FaceRecognitionResult struct {
	Match    bool
	User     string
	UserID   string
	MemberID string
	Score    float64
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
	return f.captureAndRecognize(nil)
}

func (f *FaceService) CaptureAndRecognizeForUser(userID uint) (*FaceRecognitionResult, error) {
	return f.captureAndRecognize(map[string]uint{"user_id": userID})
}

func (f *FaceService) CaptureAndRecognizeCandidates(candidates []FaceCandidate) (*FaceRecognitionResult, error) {
	return f.captureAndRecognize(map[string][]FaceCandidate{"candidates": candidates})
}

func (f *FaceService) captureAndRecognize(payload any) (*FaceRecognitionResult, error) {
	url := f.baseURL + "/capture-and-recognize"

	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal recognition request: %w", err)
		}
		body = bytes.NewReader(data)
	}

	resp, err := f.client.Post(url, "application/json", body)
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
		Match:    raw.Match,
		User:     raw.User,
		UserID:   raw.UserID,
		MemberID: raw.MemberID,
		Score:    raw.Score,
		Spoof:    raw.Spoof,
	}

	if raw.Frame != "" {
		frameBytes, err := base64.StdEncoding.DecodeString(raw.Frame)
		if err != nil {
			log.Printf("[FaceService] Warning: failed to decode frame base64: %v", err)
		} else {
			result.FrameJPG = frameBytes
		}
	}

	log.Printf("[FaceService] match=%v user=%q memberID=%q score=%.4f spoof=%v frame_size=%d bytes",
		result.Match, result.User, result.MemberID, result.Score, result.Spoof, len(result.FrameJPG))
	return result, nil
}

// EnrollFace sends image bytes to the face service to enroll a scoped family face.
func (f *FaceService) EnrollFace(userID uint, memberID uint, name string, imageBytes []byte) error {
	type enrollRequest struct {
		UserID   uint   `json:"user_id"`
		MemberID uint   `json:"member_id"`
		Name     string `json:"name"`
		Image    string `json:"image"`
	}

	payload := enrollRequest{
		UserID:   userID,
		MemberID: memberID,
		Name:     name,
		Image:    base64.StdEncoding.EncodeToString(imageBytes),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal enroll request: %w", err)
	}

	resp, err := f.client.Post(f.baseURL+"/enroll", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("face service unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("enroll failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	log.Printf("[FaceService] Enrolled face userID=%d memberID=%d name=%q", userID, memberID, name)
	return nil
}

// EnrollFaceFromURL asks the face service to fetch a Cloudinary image in memory
// and enroll only the generated embedding under the scoped owner/member.
func (f *FaceService) EnrollFaceFromURL(userID uint, memberID uint, name string, imageURL string) error {
	type enrollRequest struct {
		UserID   uint   `json:"user_id"`
		MemberID uint   `json:"member_id"`
		Name     string `json:"name"`
		ImageURL string `json:"image_url"`
	}

	payload := enrollRequest{
		UserID:   userID,
		MemberID: memberID,
		Name:     name,
		ImageURL: imageURL,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal enroll-url request: %w", err)
	}

	resp, err := f.client.Post(f.baseURL+"/enroll-url", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("face service unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("enroll-url failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	log.Printf("[FaceService] Enrolled face from URL userID=%d memberID=%d name=%q", userID, memberID, name)
	return nil
}

func (f *FaceService) ExtractEmbeddingFromURL(userID uint, memberID uint, name string, imageURL string) ([]float64, error) {
	type embedRequest struct {
		UserID   uint   `json:"user_id"`
		MemberID uint   `json:"member_id"`
		Name     string `json:"name"`
		ImageURL string `json:"image_url"`
	}

	payload := embedRequest{
		UserID:   userID,
		MemberID: memberID,
		Name:     name,
		ImageURL: imageURL,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embed-url request: %w", err)
	}

	resp, err := f.client.Post(f.baseURL+"/embed-url", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("face service unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embed-url failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result FaceEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode embed-url response: %w", err)
	}
	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("face service returned empty embedding")
	}

	return result.Embedding, nil
}

// DeleteFace removes an enrolled scoped face from the face service.
func (f *FaceService) DeleteFace(userID uint, memberID uint) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/faces/%d/%d", f.baseURL, userID, memberID), nil)
	if err != nil {
		return fmt.Errorf("failed to build delete request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("face service unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Already gone — treat as success
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete face failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	log.Printf("[FaceService] Deleted face userID=%d memberID=%d", userID, memberID)
	return nil
}

// ListFaces returns the names of all enrolled faces from the face service.
func (f *FaceService) ListFaces() ([]string, error) {
	resp, err := f.client.Get(f.baseURL + "/faces")
	if err != nil {
		return nil, fmt.Errorf("face service unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list faces failed (status %d)", resp.StatusCode)
	}

	var result struct {
		Faces json.RawMessage `json:"faces"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode list faces response: %w", err)
	}

	var legacy []string
	if err := json.Unmarshal(result.Faces, &legacy); err == nil {
		return legacy, nil
	}

	var scoped map[string][]struct {
		MemberID string `json:"member_id"`
		Name     string `json:"name"`
	}
	if err := json.Unmarshal(result.Faces, &scoped); err != nil {
		return nil, fmt.Errorf("failed to decode scoped face list: %w", err)
	}

	names := make([]string, 0)
	for userID, faces := range scoped {
		for _, face := range faces {
			names = append(names, fmt.Sprintf("%s/%s:%s", userID, face.MemberID, face.Name))
		}
	}
	return names, nil
}
func (f *FaceService) Recognize(userID uint, image []byte) (bool, string, error) {
	url := fmt.Sprintf("%s/recognize?user_id=%d", f.baseURL, userID)

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
