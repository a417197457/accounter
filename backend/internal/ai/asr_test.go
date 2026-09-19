package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTranscribeUsesConfiguredAudioModelAndCompatibleEndpoint(t *testing.T) {
	var gotModel string
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("Authorization 不正确: %q", r.Header.Get("Authorization"))
		}
		var body asrRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		gotModel = body.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"午餐二十五元"}}]}`))
	}))
	defer server.Close()

	service := &Service{}
	service.ConfigureASR(server.URL+"/compatible-mode/v1", "test-key", "qwen-audio-3.0-asr-flash")
	text, err := service.Transcribe(context.Background(), "ZmFrZQ==", "mp3")
	if err != nil {
		t.Fatalf("语音识别请求失败: %v", err)
	}
	if text != "午餐二十五元" {
		t.Fatalf("识别文本不正确: %q", text)
	}
	if gotModel != "qwen-audio-3.0-asr-flash" {
		t.Fatalf("语音模型不正确: %q", gotModel)
	}
	if gotPath != "/compatible-mode/v1/chat/completions" {
		t.Fatalf("兼容接口路径不正确: %q", gotPath)
	}
}

func TestAIServiceCanBeDisabled(t *testing.T) {
	service := &Service{}
	service.ConfigureEnabled(false)
	service.Configure("https://example.com/v1", "test-key", "test-model")
	if service.IsEnabled() {
		t.Fatal("AI 总开关应为关闭")
	}
	if service.IsConfigured() {
		t.Fatal("AI 关闭时不应报告为可用")
	}
}
