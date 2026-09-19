package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"accounter/backend/internal/ai"

	"github.com/gin-gonic/gin"
)

func TestDisabledAIRejectsModelEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &ai.Service{}
	service.ConfigureEnabled(false)
	handler := &AIHandler{aiService: service}

	router := gin.New()
	router.POST("/ai/chat", handler.Chat)
	request := httptest.NewRequest(http.MethodPost, "/ai/chat", strings.NewReader(`{"message":"午餐25元"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("AI 关闭时状态码应为 503，实际为 %d", response.Code)
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if body.Code != "AI_DISABLED" {
		t.Fatalf("AI 关闭时错误码不正确: %q", body.Code)
	}
}
