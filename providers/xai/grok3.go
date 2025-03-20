package xai

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"golang.org/x/net/http2"
)

// grok3 represents the Grok-3 model from xAI.
type grok3 struct {
	client         *http.Client
	baseURL        string
	conversationID string // Dynamic conversation ID for API endpoint
	modelName      string
	maxTokens      int
	temperature    float64
	stream         bool
	sessionCookie  string // sso cookie
	// apiKey         string // API key for authentication
	// anonUserID     string // x-anonuserid cookie
	// challenge      string // x-challenge cookie
	// signature      string // x-signature cookie
	// ssoRW          string // sso-rw cookie
	// cfClearance    string // cf_clearance cookie
	requireHTTP2 bool // Whether to require HTTP/2 for API connections
}

// GrokOption configures the grok3 instance.
type GrokOption func(*grok3)

func WithBaseURL(baseURL string) GrokOption {
	return func(g *grok3) {
		g.baseURL = baseURL
	}
}

func WithConversationID(id string) GrokOption {
	return func(g *grok3) {
		g.conversationID = id
	}
}

func WithModel(modelName string) GrokOption {
	return func(g *grok3) {
		g.modelName = modelName
	}
}

func WithMaxTokens(maxTokens int) GrokOption {
	return func(g *grok3) {
		g.maxTokens = maxTokens
	}
}

func WithTemperature(temperature float64) GrokOption {
	return func(g *grok3) {
		g.temperature = temperature
	}
}

func WithStream(stream bool) GrokOption {
	return func(g *grok3) {
		g.stream = stream
	}
}

// func WithAPIKey(apiKey string) GrokOption {
// 	return func(g *grok3) {
// 		g.apiKey = apiKey
// 	}
// }

func WithSessionCookie(cookie string) GrokOption {
	return func(g *grok3) {
		g.sessionCookie = cookie
	}
}

// func WithAnonUserID(anonUserID string) GrokOption {
// 	return func(g *grok3) {
// 		g.anonUserID = anonUserID
// 	}
// }

// func WithChallenge(challenge string) GrokOption {
// 	return func(g *grok3) {
// 		g.challenge = challenge
// 	}
// }

// func WithSignature(signature string) GrokOption {
// 	return func(g *grok3) {
// 		g.signature = signature
// 	}
// }

// func WithSSORW(ssoRW string) GrokOption {
// 	return func(g *grok3) {
// 		g.ssoRW = ssoRW
// 	}
// }

// func WithCFClearance(cfClearance string) GrokOption {
// 	return func(g *grok3) {
// 		g.cfClearance = cfClearance
// 	}
// }

// WithHTTPClient sets a custom HTTP client for the grok3 instance.
func WithHTTPClient(client *http.Client) GrokOption {
	return func(g *grok3) {
		// TODO: force wrap this in h2
		log.Println("warning: setting client")
		g.client = client
	}
}

// WithRequireHTTP2 configures whether HTTP/2 is strictly required for API connections.
// Default is true for production use. Set to false for testing with httprr or similar tools.
func WithRequireHTTP2(require bool) GrokOption {
	return func(g *grok3) {
		g.requireHTTP2 = require
	}
}

// NewGrok3 creates a new Grok-3 model instance.
// It requires HTTP/2 support to work properly with the Grok API.
func NewGrok3(options ...GrokOption) (*grok3, error) {
	g := &grok3{
		baseURL:      "https://grok.com/rest/app-chat/conversations/",
		modelName:    "grok-3",
		maxTokens:    4096,
		temperature:  0.05,
		stream:       true, // Default to streaming mode for better UX
		requireHTTP2: true, // Default to requiring HTTP/2 for production use
		// apiKey:        os.Getenv("XAI_API_KEY"),
		sessionCookie: os.Getenv("XAI_SESSION_COOKIE"),
		// anonUserID:    os.Getenv("XAI_ANON_USERID"),
		// challenge:     os.Getenv("XAI_CHALLENGE"),
		// signature:     os.Getenv("XAI_SIGNATURE"),
		// ssoRW:         os.Getenv("XAI_SSO_RW"),
		// cfClearance:   os.Getenv("XAI_CF_CLEARANCE"),
	}

	for _, opt := range options {
		opt(g)
	}
	if g.client == nil {
		log.Println("setting up client")
		var err error
		// Create a custom transport with HTTP/2 enabled for Grok API communication
		// HTTP/2 is required for proper handling of streaming responses and matches browser behavior
		// proxyURL, err := url.Parse("socks5://localhost:8889") // Change to your proxy
		// // For SOCKS5: proxyURL, err := url.Parse("socks5://proxy-server:port")
		// if err != nil {
		// 	return nil, err
		// }
		// r, err := http.NewRequest(http.MethodPost, "https://grok.com", nil)
		// if err != nil {
		// 	return nil, err
		// }
		// proxyURL, err = http.ProxyFromEnvironment(r)
		// if err != nil {
		// 	return nil, err
		// }
		// log.Println("proxy url:", proxyURL)
		customTransport := &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			ForceAttemptHTTP2: true,
		}
		//customTransport = &http.Transport{
		//	//Proxy: http.ProxyFromEnvironment(
		//}

		// Explicitly configure HTTP/2 using the golang.org/x/net/http2 package
		// This provides better control over HTTP/2 settings
		err = http2.ConfigureTransport(customTransport)
		if err != nil {
			log.Printf("WARN: Failed to configure HTTP/2 transport: %v", err)
			return nil, err
		} else if os.Getenv("XAI_DEBUG") == "1" {
			log.Printf("Successfully configured HTTP/2 transport")
		}

		// Create client with the custom transport
		g.client = &http.Client{
			Transport: customTransport,
			Timeout:   10 * time.Second, // Use a shorter timeout for tests
		}
	} else {
		log.Printf("WARN: Using custom HTTP client for XAI Grok API")

		// Try to enhance the transport with HTTP/2 if possible
		if transport, ok := g.client.Transport.(*http.Transport); ok {
			// Try to configure HTTP/2 on the existing transport
			err := http2.ConfigureTransport(transport)
			if err != nil {
				log.Printf("WARN: Failed to configure HTTP/2 on custom transport: %v", err)
			} else if os.Getenv("XAI_DEBUG") == "1" {
				log.Printf("Successfully configured HTTP/2 on custom transport")
			}
		} else {
			// If the transport is not a standard http.Transport, wrap with http2.Transport
			// Create a direct HTTP/2 transport
			h2Transport := &http2.Transport{
				AllowHTTP: false,
				DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
					return tls.Dial(network, addr, cfg)
				},
			}
			g.client.Transport = h2Transport
			if os.Getenv("XAI_DEBUG") == "1" {
				log.Printf("Applied dedicated HTTP/2 transport to custom client")
			}
		}
	}

	// Require at least sso cookie
	if g.sessionCookie == "" {
		return nil, fmt.Errorf("XAI_SESSION_COOKIE environment variable is required")
	}

	return g, nil
}

// Call implements the llms.Model interface for simple prompt calls.
func (g *grok3) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}
	resp, err := g.GenerateContent(ctx, messages, options...)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}
	return resp.Choices[0].Content, nil
}

// GenerateContent generates text content using the Grok-3 API.
func (g *grok3) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	// Apply call options
	opts := &llms.CallOptions{}
	for _, opt := range options {
		opt(opts)
	}

	// Validate inputs
	if len(messages) == 0 || len(messages) > 100 {
		return nil, fmt.Errorf("invalid message count: %d (must be 1-100)", len(messages))
	}

	// Get the message content to send to Grok
	var messageContent string
	for _, msg := range messages {
		for _, part := range msg.Parts {
			s := fmt.Sprint(part)
			if len(s) <= 1024 { // Basic sanitization
				if messageContent != "" {
					messageContent += " "
				}
				messageContent += strings.TrimSpace(s)
			}
		}
	}

	// Start a new conversation if we don't have one yet
	if g.conversationID == "" {
		if err := g.startNewConversation(ctx, messageContent); err != nil {
			return nil, fmt.Errorf("failed to start conversation: %w", err)
		}
		// Return the response from the initial conversation
		// We need to send the message again for the first response
		return g.getConversationResponse(ctx, opts, messageContent)
	} else {
		// For existing conversations, pass the message to continue the conversation
		// The Grok API requires a non-empty message for all prompts
		return g.getConversationResponse(ctx, opts, messageContent)
	}
}

// setRequestHeaders sets common headers for Grok API requests to match successful curl requests
func (g *grok3) setRequestHeaders(req *http.Request) {
	// These headers exactly match the successful curl request pattern from new-proxy.sh
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36")
	req.Header.Set("sec-ch-ua-full-version-list", `"Chromium";v="134.0.6998.89", "Not:A-Brand";v="24.0.0.0", "Google Chrome";v="134.0.6998.89"`)
	req.Header.Set("sec-ch-ua-platform", `"macOS"`)
	req.Header.Set("sec-ch-ua", `"Chromium";v="134", "Not:A-Brand";v="24", "Google Chrome";v="134"`)
	req.Header.Set("sec-ch-ua-bitness", `"64"`)
	req.Header.Set("sec-ch-ua-model", `""`)
	req.Header.Set("sec-ch-ua-mobile", `?0`)
	req.Header.Set("sec-ch-ua-arch", `"arm"`)
	req.Header.Set("sec-ch-ua-full-version", `"134.0.6998.89"`)
	req.Header.Set("sec-ch-ua-platform-version", `"15.2.0"`)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("DNT", "1")
}

// setCookies adds required cookies to requests
func (g *grok3) setCookies(req *http.Request) {
	// Only use the sso cookie as that seems to be enough to bypass Cloudflare protection
	// This matches the working curl script in new-proxy.sh
	if g.sessionCookie != "" {
		req.AddCookie(&http.Cookie{Name: "sso", Value: g.sessionCookie})
	}

	/*
		if g.anonUserID != "" {
			req.AddCookie(&http.Cookie{Name: "x-anonuserid", Value: g.anonUserID})
		}
		if g.challenge != "" {
			req.AddCookie(&http.Cookie{Name: "x-challenge", Value: g.challenge})
		}
		if g.signature != "" {
			req.AddCookie(&http.Cookie{Name: "x-signature", Value: g.signature})
		}
		if g.ssoRW != "" {
			req.AddCookie(&http.Cookie{Name: "sso-rw", Value: g.ssoRW})
		}
		if g.cfClearance != "" {
			req.AddCookie(&http.Cookie{Name: "cf_clearance", Value: g.cfClearance})
		}
	*/
}

// Name returns the model name.
func (g *grok3) Name() string {
	return g.modelName
}

// WithClient updates the HTTP client and returns the model for chaining.
func (g *grok3) WithClient(client *http.Client) llms.Model {
	if client != nil {
		g.client = client
		// TODO: force h2
	}
	return g
}

// startNewConversation creates a new conversation and sets the conversation ID
func (g *grok3) startNewConversation(ctx context.Context, message string) error {
	// Create request body
	requestBody := map[string]any{
		"temporary":                 false,
		"modelName":                 g.modelName,
		"message":                   message,
		"fileAttachments":           []string{},
		"imageAttachments":          []string{},
		"disableSearch":             false,
		"enableImageGeneration":     true,
		"returnImageBytes":          false,
		"returnRawGrokInXaiRequest": false,
		"enableImageStreaming":      true,
		"imageGenerationCount":      2,
		"forceConcise":              false,
		"toolOverrides":             map[string]bool{},
		"enableSideBySide":          true,
		"isPreset":                  false,
		"sendFinalMetadata":         true,
		"isReasoning":               false,
		"webpageUrls":               []string{},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", g.baseURL+"new", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers and cookies
	g.setRequestHeaders(req)
	g.setCookies(req)
	req.Header.Set("Referer", "https://grok.com/")

	// Debug dump request
	if os.Getenv("XAI_DEBUG") == "1" {
		dump, err := httputil.DumpRequestOut(req, true)
		if err != nil {
			log.Printf("Error dumping request: %v", err)
		} else {
			log.Printf("Request dump:\n%s", string(dump))
		}

		// Log HTTP version info to help debug HTTP/2 issues
		if transport, ok := g.client.Transport.(*http.Transport); ok {
			log.Printf("Transport config: ForceAttemptHTTP2=%v, TLSHandshakeTimeout=%v",
				transport.ForceAttemptHTTP2, transport.TLSHandshakeTimeout)
		}
	}

	// Execute request
	log.Printf("Starting new conversation...")
	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("network error connecting to Grok API: %w", err)
	}
	defer resp.Body.Close()

	// Debug dump response
	if os.Getenv("XAI_DEBUG") == "1" {
		dump, err := httputil.DumpResponse(resp, true)
		if err != nil {
			log.Printf("Error dumping response: %v", err)
		} else {
			log.Printf("Response dump:\n%s", string(dump))
		}

		// Log the HTTP protocol version used
		log.Printf("HTTP Protocol Version: %d.%d", resp.ProtoMajor, resp.ProtoMinor)
		if resp.ProtoMajor == 2 {
			log.Printf("Successfully using HTTP/2")
		} else if resp.ProtoMajor == 1 {
			log.Printf("ERROR: Using HTTP/1.1 instead of HTTP/2")
		}
	}

	// Check for HTTP/2 if required
	if resp.ProtoMajor != 2 && g.requireHTTP2 {
		return fmt.Errorf("HTTP/2 is required for Grok API connection but HTTP/%d.%d was negotiated. "+
			"Check your HTTP/2 configuration or use WithRequireHTTP2(false) for testing",
			resp.ProtoMajor, resp.ProtoMinor)
	}

	if resp.StatusCode != http.StatusOK {
		// If the request failed due to Cloudflare protection, provide a helpful error message
		if resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("received 403 Forbidden response - Cloudflare protection activated. Your cookies have likely expired. Extract fresh cookies from grok.com and update XAI_SESSION_COOKIE and XAI_CF_CLEARANCE environment variables")
		}
		return fmt.Errorf("request failed with status: %s", resp.Status)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Log the response body for debugging
	if os.Getenv("XAI_DEBUG") == "1" {
		log.Printf("Response body: %s", string(body))
	}

	// Parse conversation ID
	var convResp struct {
		Result struct {
			Conversation struct {
				ConversationID string `json:"conversationId"`
			} `json:"conversation"`
		} `json:"result"`
	}

	// The response may include multiple JSON lines - try to extract the first valid one
	scanner := bufio.NewScanner(bytes.NewReader(body))
	jsonParsed := false
	
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "{") {
			continue
		}

		// Try to parse each line
		if err := json.Unmarshal([]byte(line), &convResp); err == nil {
			jsonParsed = true
			break
		} else if os.Getenv("XAI_DEBUG") == "1" {
			log.Printf("Failed to parse line as JSON: %v, line: %s", err, line)
		}
	}

	if !jsonParsed {
		// Fallback: try to parse the entire body
		if err := json.Unmarshal(body, &convResp); err != nil {
			return fmt.Errorf("failed to parse conversation response: %w", err)
		}
	}

	if convResp.Result.Conversation.ConversationID == "" {
		return fmt.Errorf("no conversation ID in response")
	}

	// Store the conversation ID
	g.conversationID = convResp.Result.Conversation.ConversationID
	log.Printf("Got conversation ID: %s", g.conversationID)

	// Wait a moment for the conversation to be ready
	time.Sleep(1 * time.Second)
	return nil
}

// getConversationResponse fetches responses from an existing conversation
// The message parameter is optional for the first message (already handled by startNewConversation)
// but required for follow-up messages in the same conversation
func (g *grok3) getConversationResponse(ctx context.Context, opts *llms.CallOptions, message ...string) (*llms.ContentResponse, error) {
	if g.conversationID == "" {
		return nil, fmt.Errorf("no conversation ID available")
	}

	// Get the message content if provided
	messageContent := ""
	if len(message) > 0 {
		messageContent = message[0]
	}

	// Create request body
	requestBody := map[string]any{
		"message":                   messageContent, // Include message content for follow-up prompts
		"modelName":                 g.modelName,
		"disableSearch":             false,
		"enableImageGeneration":     true,
		"imageAttachments":          []string{},
		"returnImageBytes":          false,
		"returnRawGrokInXaiRequest": false,
		"fileAttachments":           []string{},
		"enableImageStreaming":      true,
		"imageGenerationCount":      2,
		"forceConcise":              false,
		"toolOverrides":             map[string]bool{},
		"enableSideBySide":          true,
		"sendFinalMetadata":         true,
		"deepsearchPreset":          "default",
		"isReasoning":               false,
		"webpageUrls":               []string{},
		"maxTokens":                 g.maxTokens,
		"temperature":               g.temperature,
	}

	if opts.MaxTokens > 0 {
		requestBody["maxTokens"] = opts.MaxTokens
	}
	if opts.Temperature > 0 {
		requestBody["temperature"] = opts.Temperature
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Build URL
	url := fmt.Sprintf("%s%s/responses", g.baseURL, g.conversationID)
	referer := fmt.Sprintf("https://grok.com/chat/%s", g.conversationID)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers and cookies
	g.setRequestHeaders(req)
	g.setCookies(req)
	req.Header.Set("Referer", referer)

	// Debug dump request
	if os.Getenv("XAI_DEBUG") == "1" {
		dump, err := httputil.DumpRequestOut(req, true)
		if err != nil {
			log.Printf("Error dumping request: %v", err)
		} else {
			log.Printf("Request dump:\n%s", string(dump))
		}
	}

	isStreaming := g.stream && opts.StreamingFunc != nil

	if !isStreaming {
		// Non-streaming mode
		log.Printf("Executing non-streaming response request...")
		resp, err := g.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("network error: %w", err)
		}
		defer resp.Body.Close()

		// Debug dump response
		if os.Getenv("XAI_DEBUG") == "1" {
			dump, err := httputil.DumpResponse(resp, true)
			if err != nil {
				log.Printf("Error dumping response: %v", err)
			} else {
				log.Printf("Response dump:\n%s", string(dump))
			}

			// Log the HTTP protocol version used
			log.Printf("HTTP Protocol Version: %d.%d", resp.ProtoMajor, resp.ProtoMinor)
			if resp.ProtoMajor == 2 {
				log.Printf("Successfully using HTTP/2")
			} else if resp.ProtoMajor == 1 {
				log.Printf("ERROR: Using HTTP/1.1 instead of HTTP/2")
			}
		}

		// Always fail if not using HTTP/2 - it's required for proper operation
		if resp.ProtoMajor != 2 {
			return nil, fmt.Errorf("HTTP/2 is required for Grok API connection but HTTP/%d.%d was negotiated. Check your HTTP/2 configuration",
				resp.ProtoMajor, resp.ProtoMinor)
		}

		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == http.StatusForbidden {
				return nil, fmt.Errorf("received 403 Forbidden response - Cloudflare protection activated. Your cookies have likely expired. Extract fresh cookies from grok.com and update XAI_SESSION_COOKIE and XAI_CF_CLEARANCE environment variables")
			}
			return nil, fmt.Errorf("request failed with status: %s", resp.Status)
		}

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		return g.parseResponse(body, nil)
	}

	// Streaming mode
	log.Printf("Starting streaming response...")
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error in streaming mode: %w", err)
	}
	defer resp.Body.Close()

	// Debug headers for streaming response
	if os.Getenv("XAI_DEBUG") == "1" {
		dump, err := httputil.DumpResponse(resp, false) // Don't dump body for streaming
		if err != nil {
			log.Printf("Error dumping streaming response headers: %v", err)
		} else {
			log.Printf("Streaming response headers:\n%s", string(dump))
		}

		// Log the HTTP protocol version used for streaming
		log.Printf("Streaming HTTP Protocol Version: %d.%d", resp.ProtoMajor, resp.ProtoMinor)
		if resp.ProtoMajor == 2 {
			log.Printf("Successfully using HTTP/2 for streaming")
		} else if resp.ProtoMajor == 1 {
			log.Printf("ERROR: Using HTTP/1.1 instead of HTTP/2 for streaming")
		}
	}

	// Check for HTTP/2 if required
	if resp.ProtoMajor != 2 && g.requireHTTP2 {
		return nil, fmt.Errorf("HTTP/2 is required for Grok API connection but HTTP/%d.%d was negotiated. "+
			"Check your HTTP/2 configuration or use WithRequireHTTP2(false) for testing",
			resp.ProtoMajor, resp.ProtoMinor)
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("received 403 Forbidden response in streaming mode - Cloudflare protection activated. Your cookies have likely expired. Extract fresh cookies from grok.com and update XAI_SESSION_COOKIE and XAI_CF_CLEARANCE environment variables")
		}
		return nil, fmt.Errorf("streaming request failed with status: %s", resp.Status)
	}

	return g.streamResponse(ctx, resp.Body, opts.StreamingFunc)
}

// parseResponse parses the multi-line JSON response and extracts tokens
func (g *grok3) parseResponse(body []byte, streamFunc func(context.Context, []byte) error) (*llms.ContentResponse, error) {
	var fullResponse strings.Builder
	ctx := context.Background()

	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "{") {
			continue
		}

		var result struct {
			Result struct {
				Response struct {
					Token      string `json:"token"`
					IsThinking bool   `json:"isThinking"`
					IsSoftStop bool   `json:"isSoftStop"`
				} `json:"response"`
			} `json:"result"`
		}

		if err := json.Unmarshal([]byte(line), &result); err != nil {
			continue // Skip lines that can't be parsed
		}

		// Process tokens
		if result.Result.Response.Token != "" {
			token := result.Result.Response.Token
			fullResponse.WriteString(token)

			// Call streaming function if provided
			if streamFunc != nil {
				if err := streamFunc(ctx, []byte(token)); err != nil {
					log.Printf("Error in streaming callback: %v", err)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan response: %w", err)
	}

	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{{
			Content:    fullResponse.String(),
			StopReason: "stop",
		}},
	}, nil
}

// streamResponse processes a streaming response body
func (g *grok3) streamResponse(ctx context.Context, body io.Reader, streamFunc func(context.Context, []byte) error) (*llms.ContentResponse, error) {
	var fullResponse strings.Builder

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		// Check if context was canceled
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "{") {
			continue
		}

		// Debug streaming data if enabled
		if os.Getenv("XAI_DEBUG") == "1" {
			log.Printf("Stream data: %s", line)
		}

		var result struct {
			Result struct {
				Response struct {
					Token      string `json:"token"`
					IsThinking bool   `json:"isThinking"`
					IsSoftStop bool   `json:"isSoftStop"`
				} `json:"response"`
			} `json:"result"`
		}

		if err := json.Unmarshal([]byte(line), &result); err != nil {
			continue // Skip lines that can't be parsed
		}

		// Process tokens
		if result.Result.Response.Token != "" {
			token := result.Result.Response.Token
			fullResponse.WriteString(token)

			// Call streaming function
			if err := streamFunc(ctx, []byte(token)); err != nil {
				log.Printf("Error in streaming callback: %v", err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error in response stream: %w", err)
	}

	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{{
			Content:    fullResponse.String(),
			StopReason: "stop",
		}},
	}, nil
}
