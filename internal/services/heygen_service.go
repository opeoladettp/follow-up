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
	// Fallback: "Jared Headshot" — confirmed present in account
	fallbackAvatarID = "906e3c1914a441bea7c8d0b1bebbc981"
	// Fallback English voice
	fallbackVoiceID = "1bd001e7e50f421d891986aad5158bc8"
)

type HeyGenService struct {
	apiKey   string
	avatarID string
	voiceID  string
	client   *http.Client
}

// --- Request types for POST /v2/video/generate ---

type heygenVideoRequest struct {
	VideoInputs []heygenVideoInput `json:"video_inputs"`
	Dimension   heygenDimension    `json:"dimension"`
	Caption     bool               `json:"caption"`
	CallbackID  string             `json:"callback_id,omitempty"`
}

type heygenVideoInput struct {
	Character heygenCharacter `json:"character"`
	Voice     heygenVoice     `json:"voice"`
}

// heygenCharacter supports both talking_photo and avatar types.
type heygenCharacter struct {
	Type            string `json:"type"`
	TalkingPhotoID  string `json:"talking_photo_id,omitempty"`
	AvatarID        string `json:"avatar_id,omitempty"`
	AvatarStyle     string `json:"avatar_style,omitempty"`
}

// heygenVoice supports text TTS and audio lip-sync.
type heygenVoice struct {
	Type      string  `json:"type"`
	VoiceID   string  `json:"voice_id,omitempty"`
	InputText string  `json:"input_text,omitempty"`
	AudioURL  string  `json:"audio_url,omitempty"`
	Speed     float64 `json:"speed,omitempty"`
}

type heygenDimension struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type heygenCreateResponse struct {
	Error interface{} `json:"error"`
	Data  struct {
		VideoID string `json:"video_id"`
	} `json:"data"`
	// Some error responses use code/message at top level
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// --- Status types for GET /v1/video_status.get ---

type heygenStatusResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		VideoID  string `json:"video_id"`
		Status   string `json:"status"`
		VideoURL string `json:"video_url"`
		Error    *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	} `json:"data"`
}

// HeyGenWebhookPayload is the POST body HeyGen sends to our webhook endpoint.
type HeyGenWebhookPayload struct {
	EventType string `json:"event_type"`
	EventData struct {
		VideoID    string `json:"video_id"`
		URL        string `json:"url"`
		Msg        string `json:"msg"`
		CallbackID string `json:"callback_id"`
	} `json:"event_data"`
}

func NewHeyGenService(apiKey, avatarID, voiceID string) *HeyGenService {
	return &HeyGenService{
		apiKey:   apiKey,
		avatarID: avatarID,
		voiceID:  voiceID,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

// WithOverrides returns a shallow copy with per-request avatar/voice overrides.
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

// GenerateVideo submits a video job to HeyGen and returns the video_id.
//
// Priority:
//  1. imageURL + audioURL  → talking_photo lip-synced to correspondent's voice
//  2. imageURL only        → talking_photo with TTS voice
//  3. configured avatarID  → HeyGen library avatar with TTS
//  4. fallback avatarID    → "Jared Headshot" with TTS
func (h *HeyGenService) GenerateVideo(script, reportID, imageURL, audioURL string) (string, error) {
	if len(script) > 4900 {
		script = script[:4900] + "..."
	}

	voiceID := h.voiceID
	if voiceID == "" {
		voiceID = fallbackVoiceID
	}

	var character heygenCharacter
	var voice heygenVoice

	switch {
	case imageURL != "" && audioURL != "":
		// Lip-sync: correspondent's photo + their own voice recording
		character = heygenCharacter{
			Type:           "talking_photo",
			TalkingPhotoID: imageURL, // HeyGen accepts URL directly as talking_photo_id for uploaded photos
		}
		voice = heygenVoice{
			Type:     "audio",
			AudioURL: audioURL,
		}
		logrus.WithField("mode", "photo+audio").Info("HeyGen: lip-sync mode")

	case imageURL != "":
		// Correspondent's photo + TTS voice
		character = heygenCharacter{
			Type:           "talking_photo",
			TalkingPhotoID: imageURL,
		}
		voice = heygenVoice{
			Type:      "text",
			VoiceID:   voiceID,
			InputText: script,
			Speed:     1.0,
		}
		logrus.WithField("mode", "photo+tts").Info("HeyGen: photo + TTS mode")

	default:
		// Library avatar fallback
		avatarID := h.avatarID
		if avatarID == "" {
			avatarID = fallbackAvatarID
		}
		character = heygenCharacter{
			Type:        "talking_photo",
			TalkingPhotoID: avatarID,
		}
		voice = heygenVoice{
			Type:      "text",
			VoiceID:   voiceID,
			InputText: script,
			Speed:     1.0,
		}
		logrus.WithFields(logrus.Fields{
			"mode":      "library-avatar",
			"avatar_id": avatarID,
		}).Info("HeyGen: library avatar mode")
	}

	payload := heygenVideoRequest{
		VideoInputs: []heygenVideoInput{{Character: character, Voice: voice}},
		Dimension:   heygenDimension{Width: 1280, Height: 720},
		Caption:     false,
		CallbackID:  reportID,
	}

	body, _ := json.Marshal(payload)

	logrus.WithFields(logrus.Fields{
		"callback_id":  reportID,
		"request_body": string(body),
	}).Info("Submitting video to HeyGen")

	req, err := http.NewRequest("POST", heygenBaseURL+"/v2/video/generate", bytes.NewReader(body))
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
	}).Info("HeyGen submit response")

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("heygen auth failed (HTTP %d) — check HEYGEN_API_KEY", resp.StatusCode)
	}

	var result heygenCreateResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse heygen response (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	if result.Data.VideoID == "" {
		return "", fmt.Errorf("heygen error (HTTP %d): %s — body: %s",
			resp.StatusCode, result.Message, string(respBody))
	}

	logrus.WithFields(logrus.Fields{
		"video_id":    result.Data.VideoID,
		"callback_id": reportID,
	}).Info("HeyGen video job submitted")
	return result.Data.VideoID, nil
}

// GetVideoStatus polls for the status of a video job.
func (h *HeyGenService) GetVideoStatus(videoID string) (status, videoURL string, err error) {
	req, err := http.NewRequest("GET", heygenBaseURL+"/v1/video_status.get?video_id="+videoID, nil)
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

	var result heygenStatusResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", fmt.Errorf("failed to parse heygen status (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	if result.Code != 100 {
		return "", "", fmt.Errorf("heygen status error %d: %s", result.Code, result.Message)
	}

	if result.Data.Error != nil {
		return "failed", "", fmt.Errorf("heygen video failed: %s", result.Data.Error.Message)
	}

	return result.Data.Status, result.Data.VideoURL, nil
}

// RegisterWebhook registers our callback URL with HeyGen for avatar_video events.
func (h *HeyGenService) RegisterWebhook(callbackURL string) error {
	existing, err := h.listWebhooks()
	if err == nil {
		for _, ep := range existing {
			if ep.URL == callbackURL {
				logrus.WithField("url", callbackURL).Info("HeyGen webhook already registered")
				return nil
			}
		}
	}

	payload := map[string]interface{}{
		"url":    callbackURL,
		"events": []string{"avatar_video.success", "avatar_video.fail"},
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", heygenBaseURL+"/v1/webhook/endpoint.add", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", h.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to register webhook: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	logrus.WithFields(logrus.Fields{
		"url":           callbackURL,
		"http_status":   resp.StatusCode,
		"response_body": string(respBody),
	}).Info("HeyGen webhook registration response")

	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook registration failed (HTTP %d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

type heygenWebhookEndpoint struct {
	EndpointID string   `json:"endpoint_id"`
	URL        string   `json:"url"`
	Events     []string `json:"events"`
}

func (h *HeyGenService) listWebhooks() ([]heygenWebhookEndpoint, error) {
	req, err := http.NewRequest("GET", heygenBaseURL+"/v1/webhook/endpoint.list", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", h.apiKey)

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Endpoints []heygenWebhookEndpoint `json:"endpoints"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Data.Endpoints, nil
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
