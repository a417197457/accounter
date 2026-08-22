package wechat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Client 只在服务端调用微信接口，AppSecret 不会下发给前端。
type Client struct {
	appID       string
	appSecret   string
	apiBase     string
	httpClient  *http.Client
	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

func NewClient(appID, appSecret, apiBase string) *Client {
	return &Client{
		appID:      appID,
		appSecret:  appSecret,
		apiBase:    strings.TrimRight(apiBase, "/"),
		httpClient: &http.Client{Timeout: 8 * time.Second},
	}
}

func (c *Client) Enabled() bool {
	return c.appID != "" && c.appSecret != ""
}

type apiError struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func (e apiError) check() error {
	if e.ErrCode != 0 {
		return fmt.Errorf("微信接口错误(%d): %s", e.ErrCode, e.ErrMsg)
	}
	return nil
}

// Code2Session 使用 wx.login 返回的 code 换取可信 openid。
func (c *Client) Code2Session(ctx context.Context, code string) (string, error) {
	if !c.Enabled() {
		return "", errors.New("服务端未配置微信小程序 AppID 或 AppSecret")
	}
	params := url.Values{
		"appid": {c.appID}, "secret": {c.appSecret}, "js_code": {code},
		"grant_type": {"authorization_code"},
	}
	var result struct {
		apiError
		OpenID string `json:"openid"`
	}
	if err := c.getJSON(ctx, c.apiBase+"/sns/jscode2session?"+params.Encode(), &result); err != nil {
		return "", err
	}
	if err := result.check(); err != nil {
		return "", err
	}
	if result.OpenID == "" {
		return "", errors.New("微信登录未返回 openid")
	}
	return result.OpenID, nil
}

// GetPhoneNumber 使用 getPhoneNumber 事件返回的一次性 code 换取手机号。
func (c *Client) GetPhoneNumber(ctx context.Context, phoneCode string) (string, error) {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return "", err
	}
	body, _ := json.Marshal(map[string]string{"code": phoneCode})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.apiBase+"/wxa/business/getuserphonenumber?access_token="+url.QueryEscape(token), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	var result struct {
		apiError
		PhoneInfo struct {
			PhoneNumber     string `json:"phoneNumber"`
			PurePhoneNumber string `json:"purePhoneNumber"`
		} `json:"phone_info"`
	}
	if err := c.doJSON(req, &result); err != nil {
		return "", err
	}
	if err := result.check(); err != nil {
		return "", err
	}
	phone := result.PhoneInfo.PurePhoneNumber
	if phone == "" {
		phone = result.PhoneInfo.PhoneNumber
	}
	if phone == "" {
		return "", errors.New("微信授权未返回手机号")
	}
	return phone, nil
}

func (c *Client) getAccessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}
	params := url.Values{"grant_type": {"client_credential"}, "appid": {c.appID}, "secret": {c.appSecret}}
	var result struct {
		apiError
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := c.getJSON(ctx, c.apiBase+"/cgi-bin/token?"+params.Encode(), &result); err != nil {
		return "", err
	}
	if err := result.check(); err != nil {
		return "", err
	}
	if result.AccessToken == "" {
		return "", errors.New("微信接口未返回 access_token")
	}
	seconds := result.ExpiresIn - 300
	if seconds < 60 {
		seconds = 60
	}
	c.accessToken = result.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(seconds) * time.Second)
	return c.accessToken, nil
}

func (c *Client) getJSON(ctx context.Context, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	return c.doJSON(req, target)
}

func (c *Client) doJSON(req *http.Request, target any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求微信接口失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("微信接口 HTTP 状态异常: %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("解析微信接口响应失败: %w", err)
	}
	return nil
}
