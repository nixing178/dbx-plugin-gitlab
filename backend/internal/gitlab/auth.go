package gitlab

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type authMode int

const (
	authToken authMode = iota
	authPassword
)

// tokenRefreshWindow 提前刷新窗口，避免临界过期导致请求失败。
const tokenRefreshWindow = 60 * time.Second

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	CreatedAt    int64  `json:"created_at"`
}

// applyAuth 按认证模式写入请求头。
func (c *Client) applyAuth(request *http.Request) {
	c.mu.Lock()
	mode := c.mode
	token := c.token
	accessToken := c.accessToken
	c.mu.Unlock()

	if mode == authToken {
		request.Header.Set("PRIVATE-TOKEN", token)
		return
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
}

// ensureAuthorized 保证密码模式持有一个可用的 access token。
func (c *Client) ensureAuthorized(ctx context.Context) error {
	c.mu.Lock()
	if c.mode == authToken {
		c.mu.Unlock()
		return nil
	}
	valid := c.accessToken != "" && time.Now().Add(tokenRefreshWindow).Before(c.expiresAt)
	refreshToken := c.refreshToken
	c.mu.Unlock()
	if valid {
		return nil
	}

	// 先尝试 refresh_token；失败再回落到密码重新换 token。
	if refreshToken != "" {
		token, err := c.exchangeToken(ctx, url.Values{
			"grant_type":    {"refresh_token"},
			"refresh_token": {refreshToken},
		})
		if err == nil {
			c.storeToken(token)
			return nil
		}
	}

	token, err := c.exchangeToken(ctx, url.Values{
		"grant_type": {"password"},
		"username":   {c.username},
		"password":   {c.password},
	})
	if err != nil {
		return err
	}
	c.storeToken(token)
	return nil
}

// exchangeToken 调用 /oauth/token 换取或刷新 token。
func (c *Client) exchangeToken(ctx context.Context, grant url.Values) (*tokenResponse, error) {
	form := url.Values{}
	for key, values := range grant {
		for _, value := range values {
			form.Set(key, value)
		}
	}
	form.Set("scope", "api")
	if c.clientID != "" {
		form.Set("client_id", c.clientID)
		if c.clientSecret != "" {
			form.Set("client_secret", c.clientSecret)
		}
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/oauth/token",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return nil, invalidConfig("cannot build token request: %v", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", userAgent)

	response, err := c.http.Do(request)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, &APIError{Kind: ErrNetwork, Message: ctxErr.Error()}
		}
		return nil, &APIError{Kind: ErrNetwork, Message: err.Error()}
	}
	defer response.Body.Close()

	payload, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBody))

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if response.StatusCode == http.StatusBadRequest || response.StatusCode == http.StatusUnauthorized {
			return nil, &APIError{
				Kind:   ErrUnauthorized,
				Status: response.StatusCode,
				Message: "password login rejected; check the username/password. " +
					"Accounts with two-factor authentication must use an access token.",
			}
		}
		return nil, mapHTTPError(response, payload)
	}

	var token tokenResponse
	if err := json.Unmarshal(payload, &token); err != nil || token.AccessToken == "" {
		return nil, &APIError{Kind: ErrServer, Message: "unexpected token response"}
	}
	return &token, nil
}

func (c *Client) storeToken(token *tokenResponse) {
	expires := time.Duration(token.ExpiresIn) * time.Second
	if expires <= 0 {
		expires = 2 * time.Hour
	}

	c.mu.Lock()
	c.accessToken = token.AccessToken
	if token.RefreshToken != "" {
		c.refreshToken = token.RefreshToken
	}
	c.expiresAt = time.Now().Add(expires)
	c.mu.Unlock()
}
