package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	heygenBaseURL = "https://api.heygen.com"
	// Fallback stock avatar — "Jared Headshot" (professional, news-friendly)
	fallbackAvatarID = "Jared_sitting_sofa_20220818"
	// Fallback English voice (ElevenLabs-backed, neutral accent)
	fallbackVoiceID = "1bd001e7e50f421d891986aad5158bc8"
)

type HeyGenService struct {
	apiKey   string
	avatarID string
	voiceID  string
	client   *http.Client
}

// heygenV2Request matches the current HeyGen POST /v2/videos schema.
type heygenV2Request struct {
	AvatarID    string              `json:"avatar_id"`
	Script      string              `json:"script"`
	VoiceID     string              `json:"voice_id"`
	Title       string              `json:"title,omitempty"`
	AspectRatio string              `json:"aspect_ratio,omitempty"`
	Voice       *heygenVoiceTuning  `json:"voice,omitempty"`
}

type heygenVoiceTuning struct {
	Speed float64 `json:"speed,omitempty"`
}

// heygenV2Response is the response from POST /v2/videos.
type heygenV2Response struct {
	VideoID string `json:"video_id"`
	Status  string `json:"status"`
	// Error fields returned on failure
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// heygenV2StatusResponse is the response from GET /v2/videos/{video_id}.
type heygenV2StatusResponse struct {
	VideoID  string  `json:"video_id"`
	Status   string  `json:"status"`
	VideoURL string  `json:"video_url"`
	Duration float64 `json:"duration"`
	Error    *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	// Legacy wrapper fields
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    *struct {
		VideoID  string `json:"video_id"`
		Status   string `json:"status"`
		VideoURL string `json:"video_url"`
		Error    *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	} `json:"data"`
}

func NewHeyGenService(apiKey, avatarID, voiceID string) *HeyGenService {
	return &HeyGenService{
		apiKey:   apiKey,
		avatarID: avatarID,
		voiceID:  voiceID,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

// WithOverrides returns a shallow copy of the service with per-request avatar/voice overrides.
// Empty strings leave the original value unchanged.
func (h *HeyGenService) WithOverrides(avatarID, voiceID string) *HeyGenService {
	cp := *h
	if avatarID != "" {
		cp.avatarID = avatarID
	}
	if voiceID != "" {
		cp.voiceID = voiceID
	}
	return &cp
}

// GenerateVideo submits a video generation job using the HeyGen v2 API and returns the video_id.
func (h *HeyGenService) GenerateVideo(script string) (string, error) {
	avatarID := h.avatarID
	if avatarID == "" {
		avatarID = fallbackAvatarID
	}
	voiceID := h.voiceID
	if voiceID == "" {
		voiceID = fallbackVoiceID
	}

	// HeyGen script limit — truncate gracefully
	if len(script) > 4900 {
		script = script[:4900] + "..."
	}

	payload := heygenV2Request{
		AvatarID:    avatarID,
		Script:      script,
		VoiceID:     voiceID,
		AspectRatio: "16:9",
		Voice:       &heygenVoiceTuning{Speed: 1.0},
	}

	body, _ := json.Marshal(payload)

	logrus.WithFields(logrus.Fields{
		"avatar_id":  avatarID,
		"voice_id":   voiceID,
		"script_len": len(script),
		"endpoint":   "/v2/videos",
	}).Info("Sending request to HeyGen")

	req, err := http.NewRequest("POST", heygenBaseURL+"/v2/videos", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("X-Api-Key", h.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("heygen request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	logrus.WithFields(logrus.Fields{
		"http_status":   resp.StatusCode,
		"response_body": string(respBody),
	}).Info("HeyGen API response")

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("heygen authentication failed (HTTP %d) — check HEYGEN_API_KEY", resp.StatusCode)
	}

	var result heygenV2Response
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse heygen response (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	// New API returns video_id directly on success; error fields on failure
	if result.VideoID == "" {
		return "", fmt.Errorf("heygen error (HTTP %d, code %d): %s — body: %s",
			resp.StatusCode, result.Code, result.Message, string(respBody))
	}

	logrus.WithField("video_id", result.VideoID).Info("HeyGen video job submitted")
	return result.VideoID, nil
}

// GetVideoStatus polls for the status of a video job using GET /v2/videos/{video_id}.
func (h *HeyGenService) GetVideoStatus(videoID string) (status, videoURL string, err error) {
	req, err := http.NewRequest("GET", heygenBaseURL+"/v2/videos/"+videoID, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create status request: %w", err)
	}
	req.Header.Set("X-Api-Key", h.apiKey)

	resp, err := h.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("heygen status request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	logrus.WithFields(logrus.Fields{
		"video_id":      videoID,
		"http_status":   resp.StatusCode,
		"response_body": string(respBody),
	}).Debug("HeyGen status response")

	var result heygenV2StatusResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", fmt.Errorf("failed to parse heygen status (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	// Handle both flat response and legacy data-wrapped response
	s := result.Status
	u := result.VideoURL
	var vidErr *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	vidErr = result.Error

	if result.Data != nil {
		if result.Data.Status != "" {
			s = result.Data.Status
		}
		if result.Data.VideoURL != "" {
			u = result.Data.VideoURL
		}
		if result.Data.Error != nil {
			vidErr = result.Data.Error
		}
	}

	if vidErr != nil {
		return "failed", "", fmt.Errorf("heygen video failed: %s", vidErr.Message)
	}

	return s, u, nil
}

// WaitForVideo polls until the video is completed or failed (max 10 min).
func (h *HeyGenService) WaitForVideo(videoID string) (string, error) {
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		status, videoURL, err := h.GetVideoStatus(videoID)
		if err != nil {
			return "", err
		}
		logrus.WithFields(logrus.Fields{"video_id": videoID, "status": status}).Debug("HeyGen poll")
		switch status {
		case "completed":
			return videoURL, nil
		case "failed":
			return "", fmt.Errorf("heygen video generation failed")
		}
		time.Sleep(10 * time.Second)
	}
	return "", fmt.Errorf("heygen video generation timed out after 10 minutes")
}
