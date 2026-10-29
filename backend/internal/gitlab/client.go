package gitlab

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	userAgent           = "dbx-gitlab-plugin/0.2.3"
	maxJSONBody         = 8 << 20
	defaultMaxFileBytes = 1 << 20
	maxErrorBody        = 64 << 10
	// MR diff 是唯一的「大响应」：几十个文件的差异轻松超过 8MB。
	maxDiffBody int64 = 64 << 20
	// 大 MR 首次拉 diff 时 GitLab 要现算，15s 的连接超时远远不够。
	maxDiffTimeout = 90 * time.Second
)

// Config 是连接 GitLab 所需的全部配置。
// 凭据二选一：Token（访问令牌）或 Username+Password（账号密码换 OAuth token）。
type Config struct {
	BaseURL string
	Token   string
	// Username / Password 走 Resource Owner Password Credentials 换 access token。
	Username string
	Password string
	// ClientID / ClientSecret 可选，GitLab.com 或启用客户端认证的实例需要。
	ClientID     string
	ClientSecret string

	ReadOnly           bool
	InsecureSkipVerify bool
	Timeout            time.Duration
	// MaxFileBytes 限制单文件读取上限，超过时只返回元信息（0 使用默认 1 MiB）。
	MaxFileBytes int64
}

// Client 是 GitLab REST 客户端。按连接复用，并发安全。
type Client struct {
	baseURL      string
	token        string
	mode         authMode
	username     string
	password     string
	clientID     string
	clientSecret string
	readOnly     bool
	http         *http.Client
	maxFileBytes int64

	mu           sync.Mutex
	version      string
	currentUser  *User
	accessToken  string
	refreshToken string
	expiresAt    time.Time
}

// NewClient 校验配置并构造客户端。
func NewClient(cfg Config) (*Client, error) {
	base := strings.TrimSpace(cfg.BaseURL)
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, invalidConfig("invalid GitLab base URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, invalidConfig("unsupported protocol %q", parsed.Scheme)
	}

	token := strings.TrimSpace(cfg.Token)
	username := strings.TrimSpace(cfg.Username)
	password := cfg.Password

	mode := authToken
	if token == "" {
		if username == "" || password == "" {
			return nil, invalidConfig("access token or username/password is required")
		}
		mode = authPassword
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	maxFileBytes := cfg.MaxFileBytes
	if maxFileBytes <= 0 {
		maxFileBytes = defaultMaxFileBytes
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			// 仅用于内网自签证书实例，默认关闭，需在连接表单显式开启。
			InsecureSkipVerify: cfg.InsecureSkipVerify,
		},
	}

	return &Client{
		baseURL:      strings.TrimRight(parsed.String(), "/"),
		token:        token,
		mode:         mode,
		username:     username,
		password:     password,
		clientID:     strings.TrimSpace(cfg.ClientID),
		clientSecret: cfg.ClientSecret,
		readOnly:     cfg.ReadOnly,
		http:         &http.Client{Transport: transport, Timeout: timeout},
		maxFileBytes: maxFileBytes,
	}, nil
}

// ReadOnly 返回连接是否处于只读模式。
func (c *Client) ReadOnly() bool { return c.readOnly }

// Version 返回最近一次探测到的 GitLab 版本。
func (c *Client) Version() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.version
}

// CloseIdleConnections 释放连接池里的空闲连接（断开连接时调用）。
func (c *Client) CloseIdleConnections() { c.http.CloseIdleConnections() }

// WhoAmI 读取当前认证用户，并刷新缓存。
func (c *Client) WhoAmI(ctx context.Context) (*User, error) {
	var user User
	if _, err := c.getJSON(ctx, "/user", nil, &user); err != nil {
		return nil, err
	}
	snapshot := user
	c.mu.Lock()
	c.currentUser = &snapshot
	c.mu.Unlock()
	return &user, nil
}

// CurrentUser 返回当前认证用户，结果按连接缓存，避免翻页时重复请求 /user。
func (c *Client) CurrentUser(ctx context.Context) (*User, error) {
	c.mu.Lock()
	cached := c.currentUser
	c.mu.Unlock()
	if cached != nil {
		snapshot := *cached
		return &snapshot, nil
	}
	return c.WhoAmI(ctx)
}

// ServerVersion 读取 GitLab 版本，并缓存到客户端。
func (c *Client) ServerVersion(ctx context.Context) (*VersionInfo, error) {
	var info VersionInfo
	header, err := c.getJSON(ctx, "/version", nil, &info)
	if err != nil {
		return nil, err
	}
	if info.Version == "" {
		info.Version = header.Get("X-Gitlab-Version")
	}
	c.mu.Lock()
	c.version = info.Version
	c.mu.Unlock()
	return &info, nil
}

// TestConnection 验证凭据并返回身份信息。版本探测失败不视为连接失败。
func (c *Client) TestConnection(ctx context.Context) (*ConnectionInfo, error) {
	user, err := c.WhoAmI(ctx)
	if err != nil {
		return nil, err
	}
	info := &ConnectionInfo{User: user, ReadOnly: c.readOnly}
	if version, versionErr := c.ServerVersion(ctx); versionErr == nil {
		info.Version = version.Version
	}
	return info, nil
}

// do 发起一次 API 请求。非 2xx 响应转换为 *APIError。
func (c *Client) do(
	ctx context.Context,
	method, path string,
	query url.Values,
	body any,
) (*http.Response, error) {
	return c.doWith(ctx, c.http, method, path, query, body)
}

// diffClient 克隆一份只放宽 Timeout 的 client。
// Transport 是共享的，连接池与认证行为完全一致，只是读大 diff 时不会 15s 被掐断。
func (c *Client) diffClient() *http.Client {
	clone := *c.http
	clone.Timeout = maxDiffTimeout
	return &clone
}

// doWith 用指定 client 发请求。
func (c *Client) doWith(
	ctx context.Context,
	httpClient *http.Client,
	method, path string,
	query url.Values,
	body any,
) (*http.Response, error) {
	if err := c.ensureAuthorized(ctx); err != nil {
		return nil, err
	}

	endpoint := c.baseURL + "/api/v4" + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, invalidConfig("cannot encode request body")
		}
		reader = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, invalidConfig("cannot build request: %v", err)
	}
	c.applyAuth(request)
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := httpClient.Do(request)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, &APIError{Kind: ErrNetwork, Message: "request timed out"}
		}
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return nil, &APIError{Kind: ErrNetwork, Message: "request timed out"}
		}
		return nil, &APIError{Kind: ErrNetwork, Message: err.Error()}
	}

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return response, nil
	}

	defer response.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBody))
	return nil, mapHTTPError(response, payload)
}

// getJSON 发起 GET 并解码 JSON，返回响应头（用于分页）。
func (c *Client) getJSON(
	ctx context.Context,
	path string,
	query url.Values,
	out any,
) (http.Header, error) {
	response, err := c.do(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	// 多读 1 字节用于判断「超出上限」，否则截断后的 JSON 会报成格式错误。
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxJSONBody+1))
	if err != nil {
		return nil, &APIError{Kind: ErrNetwork, Message: err.Error()}
	}
	if len(payload) > maxJSONBody {
		return nil, &APIError{Kind: ErrServer, Message: "response too large"}
	}
	if out != nil {
		if err := json.Unmarshal(payload, out); err != nil {
			return nil, &APIError{Kind: ErrServer, Message: "unexpected response format"}
		}
	}
	return response.Header, nil
}

// getJSONLimited 与 getJSON 相同，但允许调用方指定更大的读取上限。
// 用流式解码（json.Decoder）而不是 ReadAll，几十 MB 的 diff 不会把内存翻倍。
func (c *Client) getJSONLimited(
	ctx context.Context,
	path string,
	query url.Values,
	out any,
	limit int64,
) (http.Header, error) {
	// 大 diff 单独给更长的读取窗口，避免刚拿到响应头就被掐断。
	response, err := c.doWith(ctx, c.diffClient(), http.MethodGet, path, query, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	limited := &io.LimitedReader{R: response.Body, N: limit}
	if out == nil {
		_, _ = io.Copy(io.Discard, limited)
		return response.Header, nil
	}
	// 先嗅探开头：解析失败时能看出返回的是 JSON、HTML 还是 gzip 二进制。
	// Peek 返回的是 bufio 内部缓冲切片，Decode 之后会被复用覆盖，必须当场拷走。
	buffer := bufio.NewReader(limited)
	peek, _ := buffer.Peek(120)
	snippet := printableSnippet(append([]byte(nil), peek...))

	if err := json.NewDecoder(buffer).Decode(out); err != nil {
		if limited.N <= 0 {
			return nil, &APIError{Kind: ErrServer, Message: "response too large"}
		}
		// 读 body 期间超时 / 连接被重置时，JSON 一定是残缺的；
		// 这跟「格式不对」是两件事，分开报才能让用户知道该怎么处理。
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, context.DeadlineExceeded) {
			return nil, &APIError{Kind: ErrNetwork, Message: "request timed out"}
		}
		// 带上 Content-Type、Go 的具体解码原因和响应开头：
		// 一端是网关返回的 HTML、一端是字段类型不匹配，都能一眼区分。
		contentType := response.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "unknown content-type"
		}
		return nil, &APIError{
			Kind: ErrServer,
			Message: "unexpected response format (" + contentType + "): " + err.Error() +
				" | starts with: " + snippet,
		}
	}
	return response.Header, nil
}

// printableSnippet 把响应开头转成可读片段；
// gzip 等二进制会显示成 \x1f\x8b，网关 HTML 会直接露出 <html。
func printableSnippet(raw []byte) string {
	const maxLen = 100
	const hexDigits = "0123456789abcdef"

	if len(raw) == 0 {
		return "(empty)"
	}
	var builder strings.Builder
	for index, char := range raw {
		if index >= maxLen {
			builder.WriteString("...")
			break
		}
		if char >= 0x20 && char < 0x7f {
			builder.WriteByte(char)
			continue
		}
		builder.WriteString(`\x`)
		builder.WriteByte(hexDigits[char>>4])
		builder.WriteByte(hexDigits[char&0x0f])
	}
	return builder.String()
}

// writeJSON 发起写请求并解码响应；out 为 nil 或响应体为空时跳过解码。
func (c *Client) writeJSON(
	ctx context.Context,
	method, path string,
	query url.Values,
	body any,
	out any,
) (http.Header, error) {
	response, err := c.do(ctx, method, path, query, body)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(response.Body, maxJSONBody))
	if err != nil {
		return nil, &APIError{Kind: ErrNetwork, Message: err.Error()}
	}
	if out != nil && len(payload) > 0 {
		if err := json.Unmarshal(payload, out); err != nil {
			return nil, &APIError{Kind: ErrServer, Message: "unexpected response format"}
		}
	}
	return response.Header, nil
}

// projectPath 返回项目级 API 前缀。GitLab 同时接受数字 ID 与 URL 编码的完整路径。
func projectPath(projectID string) (string, error) {
	id := strings.TrimSpace(projectID)
	if id == "" {
		return "", invalidConfig("project id is required")
	}
	return "/projects/" + url.PathEscape(id), nil
}

// normalizePaging 归一化分页参数，避免请求超大页。
func normalizePaging(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	return page, perPage
}

// pagingQuery 构造分页查询参数。
func pagingQuery(page, perPage int) url.Values {
	page, perPage = normalizePaging(page, perPage)
	query := url.Values{}
	query.Set("page", strconv.Itoa(page))
	query.Set("per_page", strconv.Itoa(perPage))
	return query
}

// pageFromHeader 解析 GitLab 分页响应头。
func pageFromHeader(header http.Header) (page int, hasMore bool, total int) {
	page, _ = strconv.Atoi(header.Get("X-Page"))
	if page == 0 {
		page = 1
	}
	hasMore = strings.TrimSpace(header.Get("X-Next-Page")) != ""
	total, _ = strconv.Atoi(header.Get("X-Total"))
	return page, hasMore, total
}

// ensureSlice 保证 JSON 序列化后是 [] 而不是 null。
func ensureSlice[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

// readFileBody 按上限读取文件正文；超限时返回 too_large。
func (c *Client) readFileBody(response *http.Response, path, ref string) (*FileContent, error) {
	size, _ := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64)
	if size > 0 && size > c.maxFileBytes {
		return &FileContent{Path: path, Ref: ref, Size: size, TooLarge: true}, nil
	}

	payload, err := io.ReadAll(io.LimitReader(response.Body, c.maxFileBytes+1))
	if err != nil {
		return nil, &APIError{Kind: ErrNetwork, Message: err.Error()}
	}
	if int64(len(payload)) > c.maxFileBytes {
		return &FileContent{Path: path, Ref: ref, Size: int64(len(payload)), TooLarge: true}, nil
	}

	content := &FileContent{Path: path, Ref: ref, Size: int64(len(payload)), Text: string(payload)}
	if !utf8.Valid(payload) || bytes.IndexByte(payload, 0) >= 0 {
		content.Binary = true
		content.Text = ""
	}
	return content, nil
}
