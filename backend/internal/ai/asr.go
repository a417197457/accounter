package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// audioMime 格式名→MIME 映射
func audioMime(format string) string {
	format = strings.ToLower(strings.TrimPrefix(format, "."))
	switch format {
	case "mp3":
		return "audio/mpeg"
	case "wav":
		return "audio/wav"
	case "webm":
		return "audio/webm"
	case "m4a":
		return "audio/mp4"
	case "aac":
		return "audio/aac"
	case "ogg":
		return "audio/ogg"
	case "flac":
		return "audio/flac"
	default:
		return "audio/mpeg"
	}
}

// asrRequest 百炼 ASR 的请求体（OpenAI 兼容 chat/completions + input_audio data URI）
type asrRequest struct {
	Model    string          `json:"model"`
	Messages []asrMessage    `json:"messages"`
}

type asrMessage struct {
	Role    string        `json:"role"`
	Content []asrContent  `json:"content"`
}

type asrContent struct {
	Type       string      `json:"type"`
	InputAudio asrAudioRef `json:"input_audio"`
}

type asrAudioRef struct {
	Data string `json:"data"`
}

type asrResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Transcribe 语音识别：百炼 qwen3-asr-flash，走 chat/completions + input_audio
// audioB64 是音频的 base64 编码（不含 data: 前缀），format 是 mp3/wav/webm/m4a/aac/ogg
func (s *Service) Transcribe(ctx context.Context, audioB64 string, format string) (string, error) {
	s.mu.RLock()
	baseURL := s.asrBaseURL
	apiKey := s.asrAPIKey
	model := s.asrModel
	s.mu.RUnlock()

	if apiKey == "" || apiKey == "YOUR_BAILIAN_API_KEY" {
		return "", errors.New("ASR 未配置，请在 config.yaml 填入 ai.api_key 或 ai.asr.api_key")
	}
	if baseURL == "" {
		return "", errors.New("ASR base_url 未配置")
	}

	mime := audioMime(format)
	dataURI := fmt.Sprintf("data:%s;base64,%s", mime, audioB64)

	reqBody := asrRequest{
		Model: model,
		Messages: []asrMessage{
			{
				Role: "user",
				Content: []asrContent{
					{
						Type: "input_audio",
						InputAudio: asrAudioRef{Data: dataURI},
					},
				},
			},
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// 端点：base_url + /chat/completions（百炼 compatible-mode/v1 已含 /v1）
	url := strings.TrimRight(baseURL, "/") + "/chat/completions"

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("创建 ASR 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ASR 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ASR 返回 %d: %s", resp.StatusCode, string(respBytes))
	}

	var asrResp asrResponse
	if err := json.Unmarshal(respBytes, &asrResp); err != nil {
		return "", fmt.Errorf("ASR 响应解析失败: %w", err)
	}
	if asrResp.Error != nil {
		return "", errors.New(asrResp.Error.Message)
	}
	if len(asrResp.Choices) == 0 {
		return "", errors.New("ASR 无响应")
	}
	return strings.TrimSpace(asrResp.Choices[0].Message.Content), nil
}
