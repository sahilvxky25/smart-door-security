package services

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type FaceService struct {
	apiURL string
}

type FaceRecognitionResponse struct {
	Match bool   `json:"match"`
	User  string `json:"user"`
}

func NewFaceService() *FaceService {
	return &FaceService{
		apiURL: "http://localhost:5000/recognize",
	}
}

func (f *FaceService) Recognize(image []byte) (bool, string, error) {

	req, err := http.NewRequest("POST", f.apiURL, bytes.NewBuffer(image))
	if err != nil {
		return false, "", err
	}

	req.Header.Set("Content-Type", "application/octet-stream")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return false, "", err
	}

	defer resp.Body.Close()

	var result FaceRecognitionResponse

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return false, "", err
	}

	return result.Match, result.User, nil
}