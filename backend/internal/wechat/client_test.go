package wechat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestClientLoginAndPhoneNumber(t *testing.T) {
	var tokenCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/sns/jscode2session":
			if r.URL.Query().Get("js_code") != "login-code" {
				t.Fatalf("unexpected login code")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"openid": "openid-1"})
		case "/cgi-bin/token":
			atomic.AddInt32(&tokenCalls, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access-token", "expires_in": 7200})
		case "/wxa/business/getuserphonenumber":
			if r.URL.Query().Get("access_token") != "access-token" {
				t.Fatalf("unexpected access token")
			}
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["code"] != "phone-code" {
				t.Fatalf("unexpected phone code")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errcode":    0,
				"phone_info": map[string]string{"purePhoneNumber": "13800138000"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient("appid", "secret", server.URL)
	openid, err := client.Code2Session(context.Background(), "login-code")
	if err != nil || openid != "openid-1" {
		t.Fatalf("Code2Session() = %q, %v", openid, err)
	}
	for range 2 {
		phone, err := client.GetPhoneNumber(context.Background(), "phone-code")
		if err != nil || phone != "13800138000" {
			t.Fatalf("GetPhoneNumber() = %q, %v", phone, err)
		}
	}
	if calls := atomic.LoadInt32(&tokenCalls); calls != 1 {
		t.Fatalf("access token calls = %d, want 1", calls)
	}
}

func TestClientRequiresServerCredentials(t *testing.T) {
	client := NewClient("", "", "https://api.weixin.qq.com")
	if _, err := client.Code2Session(context.Background(), "code"); err == nil {
		t.Fatal("expected missing credentials error")
	}
}
