package providers

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"gemini-web-to-api/internal/commons/configs"

	"github.com/imroc/req/v3"
	"go.uber.org/zap"
)

type Client struct {
	httpClient *req.Client
	cookies    *CookieStore
	at         string
	mu         sync.RWMutex // protects: at, healthy
	healthy    bool
	log        *zap.Logger

	maxRetries   int
	cachedModels []ModelInfo
}

type CookieStore struct {
	Secure1PSID   string `json:"__Secure-1PSID"`
	Secure1PSIDCC string `json:"__Secure-1PSIDCC"`
	ExtraCookies  string `json:"extra_cookies"`
	mu            sync.RWMutex
}

func NewClient(cfg *configs.Config, log *zap.Logger) *Client {
	cookies := &CookieStore{
		Secure1PSID:   cfg.Gemini.Secure1PSID,
		Secure1PSIDCC: cfg.Gemini.Secure1PSIDCC,
		ExtraCookies:  cfg.Gemini.Cookies,
	}

	client := req.NewClient().
		SetTimeout(10 * time.Minute).
		SetCommonHeaders(DefaultHeaders)

	if proxyURL := os.Getenv("HTTPS_PROXY"); proxyURL != "" {
		client.SetProxyURL(proxyURL)
	}

	return &Client{
		httpClient: client,
		cookies:    cookies,
		maxRetries: cfg.Gemini.MaxRetries,
		log:        log,
	}
}

func (c *Client) Init(ctx context.Context) error {
	// Clean cookies
	c.cookies.Secure1PSID = cleanCookie(c.cookies.Secure1PSID)
	c.cookies.Secure1PSIDCC = cleanCookie(c.cookies.Secure1PSIDCC)
	c.cookies.FillFromExtraCookies()

	// Populate cookies
	c.httpClient.SetCommonCookies(c.cookies.ToHTTPCookies()...)

	// Get SNlM0e token
	err := c.refreshSessionToken()
	if err != nil {
		return err
	}

	c.log.Info("✅ Gemini client initialized successfully")
	return nil
}

func (c *Client) refreshSessionToken() error {
	// 1. Initial hit to google.com to get extra cookies (NID, etc)
	tmpClient := req.NewClient().
		SetTimeout(30 * time.Second).
		SetUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp1, err := tmpClient.R().Get("https://www.google.com/")
	extraCookies := ""
	if err == nil {
		parts := []string{}
		for _, ck := range resp1.Cookies() {
			parts = append(parts, fmt.Sprintf("%s=%s", ck.Name, ck.Value))
			// Also sync to main client
			c.httpClient.SetCommonCookies(ck)
		}
		if len(parts) > 0 {
			extraCookies = strings.Join(parts, "; ") + "; "
		}
	}

	// 2. Prepare full cookie string
	cookieStr := mergeCookieHeader(cookieHeaderFromCookies(c.cookies.ToHTTPCookies()), extraCookies)

	commonHeaders := map[string]string{
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"Accept-Language":           "en-US,en;q=0.9",
		"Cache-Control":             "max-age=0",
		"Origin":                    "https://gemini.google.com",
		"Sec-Ch-Ua":                 `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`,
		"Sec-Ch-Ua-Mobile":          "?0",
		"Sec-Ch-Ua-Platform":        `"Windows"`,
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "none",
		"Sec-Fetch-User":            "?1",
		"Upgrade-Insecure-Requests": "1",
		"X-Same-Domain":             "1",
		"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}

	hClient := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil // follow redirects
		},
	}

	req1, _ := http.NewRequest("GET", "https://gemini.google.com/?hl=en", nil)
	for k, v := range commonHeaders {
		req1.Header.Set(k, v)
	}
	req1.Header.Set("Cookie", cookieStr)
	resp1_direct, _ := hClient.Do(req1)
	if resp1_direct != nil {
		cookieStr = mergeCookies(cookieStr, resp1_direct.Cookies())
		for _, ck := range resp1_direct.Cookies() {
			c.httpClient.SetCommonCookies(ck)
		}
		resp1_direct.Body.Close()
	}

	// 2. The main INIT hit
	req2, _ := http.NewRequest("GET", EndpointInit+"?hl=en", nil)
	for k, v := range commonHeaders {
		req2.Header.Set(k, v)
	}
	req2.Header.Set("Sec-Fetch-Site", "same-origin")
	req2.Header.Set("Cookie", cookieStr)
	req2.Header.Set("Referer", "https://gemini.google.com/")
	req2.Header.Set("Accept-Encoding", "gzip, deflate, br")

	resp, err := hClient.Do(req2)
	if err != nil {
		return fmt.Errorf("failed to reach gemini app: %w", err)
	}
	defer resp.Body.Close()

	// Dump for debugging if it fails
	// reqDump, _ := httputil.DumpRequestOut(req2, false)
	// respDump, _ := httputil.DumpResponse(resp, false)

	var bodyReader io.ReadCloser = resp.Body
	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		gz, err := gzip.NewReader(resp.Body)
		if err == nil {
			bodyReader = gz
			defer gz.Close()
		}
	}

	bodyBytes, _ := io.ReadAll(bodyReader)
	body := string(bodyBytes)

	re := regexp.MustCompile(`"SNlM0e":"([^"]+)"`)
	matches := re.FindStringSubmatch(body)
	if len(matches) < 2 {
		reFallback := regexp.MustCompile(`\["SNlM0e","([^"]+)"\]`)
		matches = reFallback.FindStringSubmatch(body)
		if len(matches) < 2 {

			errMsg := "authentication failed: SNlM0e not found"
			if strings.Contains(body, "Sign in") || strings.Contains(body, "login") {
				errMsg = "authentication failed: cookies invalid. Please provide a fresh GEMINI_COOKIES value from a signed-in browser session"
			}

			// Fallback: use __Secure-1PSIDCC as at token if available
			if c.cookies.Secure1PSIDCC != "" {
				c.at = c.cookies.Secure1PSIDCC
				c.healthy = true
				c.refreshModels(body)
				c.log.Info("Using __Secure-1PSIDCC cookie as at token (SNlM0e not found)")
				return nil
			}

			// Log as Info to avoid stack trace for expected auth failures
			c.log.Info(errMsg)
			return fmt.Errorf("%s", errMsg)
		}
	}

	c.mu.Lock()
	c.at = matches[1]
	c.healthy = true
	c.mu.Unlock()

	// Update dynamic models from the same initialization body
	c.refreshModels(body)

	return nil
}

func (c *Client) refreshModels(body string) {
	var newModels []ModelInfo
	now := time.Now().Unix()

	// Improved regex to find gemini model IDs even when escaped in JSON
	// Matches IDs like gemini-2.0-flash, gemini-1.5-pro, etc.
	// We look for gemini- followed by alphanumeric characters, dots, or dashes.
	modelIDRegex := regexp.MustCompile(`gemini-[a-zA-Z0-9.-]+`)
	matches := modelIDRegex.FindAllString(body, -1)

	uniqueIDs := make(map[string]bool)
	for _, id := range matches {
		// Clean up potential trailing backslashes or quotes if they were caught
		id = strings.Trim(id, `\"' `)

		// Basic validation: ensure it doesn't look like a generic string or partial ID
		if !uniqueIDs[id] && len(id) > 10 {
			uniqueIDs[id] = true
			newModels = append(newModels, ModelInfo{
				ID:       id,
				Created:  now,
				OwnedBy:  "google",
				Provider: "gemini",
			})
		}
	}

	// Add well-known models that may not appear in the initial HTML (loaded
	// dynamically by the web UI) but are accepted by the Gemini backend.
	knownModels := []string{
		"gemini-3.5-flash",
		"gemini-3.1-pro",
	}
	for _, id := range knownModels {
		if !uniqueIDs[id] {
			uniqueIDs[id] = true
			newModels = append(newModels, ModelInfo{
				ID:       id,
				Created:  now,
				OwnedBy:  "google",
				Provider: "gemini",
			})
		}
	}

	c.mu.Lock()
	c.cachedModels = newModels
	c.mu.Unlock()

	if len(newModels) == 0 {
		c.log.Warn("⚠️ No models found in Gemini Web response. Please check your cookies or connection.")
	} else {
		ids := make([]string, 0, len(newModels))
		for _, m := range newModels {
			ids = append(ids, m.ID)
		}
		c.log.Info("🔄 Refreshed available models from Gemini Web", zap.Int("count", len(newModels)), zap.Strings("models", ids))
	}
}

func (c *Client) GetCookies() *CookieStore {
	c.cookies.mu.RLock()
	defer c.cookies.mu.RUnlock()

	return &CookieStore{
		Secure1PSID:   c.cookies.Secure1PSID,
		Secure1PSIDCC: c.cookies.Secure1PSIDCC,
		ExtraCookies:  c.cookies.ExtraCookies,
	}
}

func (c *Client) GenerateContent(ctx context.Context, prompt string, options ...GenerateOption) (*Response, error) {
	config := &GenerateConfig{}
	for _, opt := range options {
		opt(config)
	}

	// Default to first available model if not set or "gemini-pro"
	c.mu.RLock()
	if config.Model == "" || config.Model == "gemini-pro" {
		if len(c.cachedModels) > 0 {
			config.Model = c.cachedModels[0].ID
		}
	}

	// Accept any model matching gemini-* pattern. The HTML page source only exposes
	// a subset; newer models load dynamically and the Gemini backend accepts them.
	modelPattern := regexp.MustCompile(`^gemini-[a-zA-Z0-9.-]+$`)
	at := c.at
	c.mu.RUnlock()

	if config.Model != "" && !modelPattern.MatchString(config.Model) {
		return nil, fmt.Errorf("model '%s' is not a valid Gemini model ID. Available models: %v", config.Model, c.ListModelsIDs())
	}

	if at == "" {
		return nil, errors.New("client not initialized")
	}

	// Build request payload
	// The structure confirmed to work for model selection is [ [prompt], nil, nil, model ]
	inner := []interface{}{
		[]interface{}{prompt},
		nil,
		nil,
		config.Model,
	}

	innerJSON, _ := json.Marshal(inner)
	outer := []interface{}{nil, string(innerJSON)}
	outerJSON, _ := json.Marshal(outer)

	formData := url.Values{}
	formData.Set("at", at)
	formData.Set("f.req", string(outerJSON))

	maxAttempts := c.maxRetries
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	totalStart := time.Now()

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			// Exponential backoff: 1s, 2s, 4s...
			backoff := time.Duration(1<<uint(attempt-2)) * time.Second
			c.log.Warn("Retrying GenerateContent",
				zap.Int("attempt", attempt),
				zap.Int("max_attempts", maxAttempts),
				zap.Duration("backoff", backoff),
				zap.Error(lastErr),
			)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		httpStart := time.Now()
		reqURL := EndpointGenerate + "?at=" + url.QueryEscape(at)
		httpReq, _ := http.NewRequestWithContext(ctx, "POST", reqURL, strings.NewReader(formData.Encode()))
		for k, v := range DefaultHeaders {
			httpReq.Header.Set(k, v)
		}
		httpReq.Header.Set("Accept", "*/*")
		httpReq.Header.Set("Sec-Fetch-Site", "same-origin")
		httpReq.Header.Set("Sec-Fetch-Mode", "cors")
		httpReq.Header.Set("Sec-Fetch-Dest", "empty")
		if c.cookies.ExtraCookies != "" {
			httpReq.Header.Set("Cookie", c.cookies.ExtraCookies)
		} else {
			for _, ck := range c.cookies.ToHTTPCookies() {
				httpReq.AddCookie(ck)
			}
		}
		var httpResp *http.Response
		var err error
		hClient := &http.Client{Timeout: 30 * time.Second}
		httpResp, err = hClient.Do(httpReq)
		var respStr string
		var respCode int
		if err == nil {
			b, _ := io.ReadAll(httpResp.Body)
			httpResp.Body.Close()
			respStr = string(b)
			respCode = httpResp.StatusCode
		}

		httpDuration := time.Since(httpStart)
		if err != nil {
			c.log.Warn("Generate request failed, will retry",
				zap.Error(err),
				zap.Duration("http_duration", httpDuration),
				zap.Int("attempt", attempt),
			)
			lastErr = err
			continue
		}

		if respCode != http.StatusOK {
			lastErr = fmt.Errorf("generate failed with status: %d", respCode)
			// Only retry on 5xx (server errors), not 4xx (client errors)
			if respCode >= 500 {
				c.log.Warn("Server error, will retry",
					zap.Int("status", respCode),
					zap.Int("attempt", attempt),
				)
				continue
			}
			return nil, lastErr
		}

		parseStart := time.Now()
		result, parseErr := c.parseResponse(respStr)
		parseDuration := time.Since(parseStart)

		if parseErr != nil {
			lastErr = parseErr
			c.log.Warn("Failed to parse response, will retry",
				zap.Error(parseErr),
				zap.Int("attempt", attempt),
			)
			continue
		}

		c.log.Debug("GenerateContent timing",
			zap.Duration("gemini_server_rtt", httpDuration),
			zap.Duration("parse_duration", parseDuration),
			zap.Duration("total_duration", time.Since(totalStart)),
			zap.Int("attempt", attempt),
			zap.Int("response_bytes", len(respStr)),
		)

		if attempt > 1 {
			c.log.Info("GenerateContent succeeded after retry", zap.Int("attempt", attempt))
		}
		return result, nil
	}

	c.log.Error("GenerateContent failed after all attempts",
		zap.Int("attempts", maxAttempts),
		zap.Error(lastErr),
	)
	return nil, fmt.Errorf("after %d attempts: %w", maxAttempts, lastErr)
}

func (c *Client) StartChat(options ...ChatOption) ChatSession {
	config := &ChatConfig{}
	for _, opt := range options {
		opt(config)
	}

	c.mu.RLock()
	if config.Model == "" || config.Model == "gemini-pro" {
		if len(c.cachedModels) > 0 {
			config.Model = c.cachedModels[0].ID
		}
	}
	c.mu.RUnlock()

	return &GeminiChatSession{
		client:   c,
		model:    config.Model,
		metadata: config.Metadata,
		history:  []Message{},
	}
}

func (c *Client) Close() error {
	c.mu.Lock()
	c.healthy = false
	c.mu.Unlock()
	return nil
}

func (c *Client) GetName() string {
	return "gemini"
}

func (c *Client) IsHealthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.healthy
}

func (c *Client) ListModels() []ModelInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.cachedModels) == 0 {
		return []ModelInfo{}
	}

	return c.cachedModels
}

func (c *Client) ListModelsIDs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ids := make([]string, 0, len(c.cachedModels))
	for _, m := range c.cachedModels {
		ids = append(ids, m.ID)
	}
	return ids
}

// parseResponse parses Gemini's response format
func (c *Client) parseResponse(text string) (*Response, error) {
	var finalResText string
	var finalMetadata map[string]any
	var finalImages []Image
	found := false

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimPrefix(line, ")]}'")

		var root []interface{}
		if err := json.Unmarshal([]byte(line), &root); err == nil {
			for _, item := range root {
				itemArray, ok := item.([]interface{})
				if !ok || len(itemArray) < 3 {
					continue
				}

				payloadStr, ok := itemArray[2].(string)
				if !ok {
					continue
				}

				var payload []interface{}
				if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
					continue
				}

				if len(payload) > 4 {
					candidates, ok := payload[4].([]interface{})
					if ok && candidates != nil && len(candidates) > 0 {
						firstCandidate, ok := candidates[0].([]interface{})
						if ok && len(firstCandidate) >= 2 {
							contentParts, ok := firstCandidate[1].([]interface{})
							if ok && len(contentParts) > 0 {
								// Collect text and image content from all parts
								var texts []string
								var respImages []Image

								for _, part := range contentParts {
									if s, ok := part.(string); ok {
										texts = append(texts, s)
									} else if pm, ok := part.(map[string]interface{}); ok {
										if id, ok := pm["inlineData"].(map[string]interface{}); ok {
											data, _ := id["data"].(string)
											mime, _ := id["mimeType"].(string)
											respImages = append(respImages, Image{
												URL: fmt.Sprintf("data:%s;base64,%s", mime, data),
											})
										}
									}
								}

								if len(texts) > 0 || len(respImages) > 0 {
									var cid, rid, rcid string
									if len(firstCandidate) > 0 {
										if id, ok := firstCandidate[0].(string); ok {
											rcid = id
										}
									}
									if len(payload) > 1 {
										if id, ok := payload[1].(string); ok {
											cid = id
										}
									}

									finalResText = strings.Join(texts, "\n")
									finalImages = respImages
									finalMetadata = map[string]any{
										"cid":  cid,
										"rid":  rid,
										"rcid": rcid,
									}
									found = true
								}
							}
						}
					}
				}
			}
		}
	}

	if found {
		return &Response{
			Text:     finalResText,
			Images:   finalImages,
			Metadata: finalMetadata,
		}, nil
	}

	sample := text
	if len(sample) > 500 {
		sample = sample[:500]
	}
	return nil, fmt.Errorf("failed to parse response. Sample: %s", sample)
}

func (cs *CookieStore) ToHTTPCookies() []*http.Cookie {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	cookies := []*http.Cookie{}
	domain := ".google.com"

	if cs.Secure1PSID != "" {
		cookies = append(cookies, &http.Cookie{
			Name:     "__Secure-1PSID",
			Value:    cleanCookie(cs.Secure1PSID),
			Domain:   domain,
			Path:     "/",
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteNoneMode,
		})
	}
	if cs.Secure1PSIDCC != "" {
		cookies = append(cookies, &http.Cookie{
			Name:     "__Secure-1PSIDCC",
			Value:    cleanCookie(cs.Secure1PSIDCC),
			Domain:   domain,
			Path:     "/",
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteNoneMode,
		})
	}
	for name, value := range parseCookiePairs(cs.ExtraCookies) {
		cookies = upsertCookie(cookies, &http.Cookie{
			Name:     name,
			Value:    value,
			Domain:   domain,
			Path:     "/",
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteNoneMode,
		})
	}
	return cookies
}

func (cs *CookieStore) FillFromExtraCookies() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	extras := parseCookiePairs(cs.ExtraCookies)
	if cs.Secure1PSID == "" {
		cs.Secure1PSID = extras["__Secure-1PSID"]
	}
	if cs.Secure1PSIDCC == "" {
		cs.Secure1PSIDCC = extras["__Secure-1PSIDCC"]
	}
}

func parseCookiePairs(raw string) map[string]string {
	pairs := make(map[string]string)
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}

		name := strings.TrimSpace(kv[0])
		value := cleanCookie(kv[1])
		if name == "" || value == "" {
			continue
		}
		pairs[name] = value
	}
	return pairs
}

func upsertCookie(cookies []*http.Cookie, cookie *http.Cookie) []*http.Cookie {
	for i, existing := range cookies {
		if existing.Name == cookie.Name {
			cookies[i] = cookie
			return cookies
		}
	}
	return append(cookies, cookie)
}

func cookieHeaderFromCookies(cookies []*http.Cookie) string {
	parts := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie == nil || cookie.Name == "" || cookie.Value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
	}
	return strings.Join(parts, "; ")
}

func mergeCookieHeader(base string, extra string) string {
	pairs := parseCookiePairs(base)
	for name, value := range parseCookiePairs(extra) {
		pairs[name] = value
	}

	parts := make([]string, 0, len(pairs))
	for name, value := range pairs {
		parts = append(parts, fmt.Sprintf("%s=%s", name, value))
	}
	return strings.Join(parts, "; ")
}

func mergeCookies(base string, cookies []*http.Cookie) string {
	pairs := parseCookiePairs(base)
	for _, cookie := range cookies {
		if cookie == nil || cookie.Name == "" || cookie.Value == "" {
			continue
		}
		pairs[cookie.Name] = cookie.Value
	}

	parts := make([]string, 0, len(pairs))
	for name, value := range pairs {
		parts = append(parts, fmt.Sprintf("%s=%s", name, value))
	}
	return strings.Join(parts, "; ")
}

func cleanCookie(v string) string {
	v = strings.TrimSpace(v)
	v = strings.Trim(v, "\"")
	v = strings.Trim(v, "'")
	v = strings.TrimSuffix(v, ";")
	return v
}

const (
	EndpointGoogle    = "https://www.google.com"
	EndpointInit      = "https://gemini.google.com/app"
	EndpointGenerate  = "https://gemini.google.com/_/BardChatUi/data/assistant.lamda.BardFrontendService/StreamGenerate"
	EndpointBatchExec = "https://gemini.google.com/_/BardChatUi/data/batchexecute"
)

var DefaultHeaders = map[string]string{
	"Content-Type":  "application/x-www-form-urlencoded;charset=utf-8",
	"Origin":        "https://gemini.google.com",
	"Referer":       "https://gemini.google.com/",
	"User-Agent":    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"X-Same-Domain": "1",
}
