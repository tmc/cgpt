package xai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"

	"github.com/tmc/langchaingo/llms"
)

type conversation struct {
	ConversationID string
	LastResponseID string

	client *http.Client

	modelName string
	baseURL   string

	setRequestHeaders func(req *http.Request)
	setCookies        func(req *http.Request)

	verbose              bool // Enable verbose logging
	veryVerbose          bool // Enable very verbose logging
	dumpRequestResponses bool // Enable dumping of request and response bodies

	logger Logger // Logger for outputting diagnostic information
}

type responseRenderer struct {
	ResponseID string
	imageSet   *streamingImageSet
}

func newResponseRenderer(ctx context.Context, responseID string) *responseRenderer {
	return &responseRenderer{
		ResponseID: responseID,
		imageSet:   newStreamingImageSet(ctx),
	}
}

type streamingImageSet struct {
	imageIds  []string
	imageData map[string]StreamingImageGenerationResponse
	images    []*StreamingImageGenerationResponse
}

func newStreamingImageSet(ctx context.Context) *streamingImageSet {
	return &streamingImageSet{
		imageData: make(map[string]StreamingImageGenerationResponse),
	}
}

func (s *streamingImageSet) AddImage(ctx context.Context, data StreamingImageGenerationResponse) {
	s.images = append(s.images, &data)
	if _, ok := s.imageData[data.ImageID]; !ok {
		s.imageData[data.ImageID] = data
		s.imageIds = append(s.imageIds, data.ImageID)
	}
}

type imageResponse struct {
	StreamingImageGenerationResponse
}

func (ir *imageResponse) Render(ctx context.Context, w io.Writer) error {
	// Render the image inline
	return nil
}

// fetchImage fetches the image data from the provided URL using the existing HTTP client
func (c *conversation) fetchImage(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create image request: %w", err)
	}

	setRequestHeaders(req)
	c.setCookies(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image fetch failed with status: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("fetched image is 0 bytes")
	}

	return data, nil
}

// Create creates a new conversation and sets the conversation ID
func (g *conversation) Create(ctx context.Context, opts *llms.CallOptions, message llms.MessageContent, options ...ConversationResponseOption) (*llms.ContentResponse, error) {

	userMessage := partsToString(message.Parts)
	// Create request body
	requestBody := map[string]any{
		"temporary":        false, // TODO; parmeterize
		"modelName":        g.modelName,
		"message":          userMessage,
		"fileAttachments":  []string{},
		"imageAttachments": []string{},
		"disableSearch":    false, // TODO: parameterize

		"enableImageGeneration": true,
		"enableImageStreaming":  true,

		"returnImageBytes":          false,
		"returnRawGrokInXaiRequest": true,
		"imageGenerationCount":      2,
		"forceConcise":              false,
		"toolOverrides":             map[string]bool{},
		"enableSideBySide":          true,
		"isPreset":                  false,
		"sendFinalMetadata":         true,

		//"deepsearchPreset":          "default",
		"isReasoning": false,

		//"customPersonality": "",
		//"customInstructions": "",

		"webpageUrls": []string{},
		//"systemPromptName":          "grok3_personality_cracked_coder",
	}
	fmt.Println(requestBody)
	for _, opt := range options {
		fmt.Println(opt, "before")
		fmt.Println(requestBody)
		opt(requestBody)
		fmt.Println(requestBody)
		fmt.Println(opt, "after")
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	//
	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", g.baseURL+"new", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers and cookies
	g.setRequestHeaders(req)
	g.setCookies(req)

	req.Header.Set("Referer", "https://grok.com/")

	logOut := &greyWriter{w: os.Stderr}
	fmt.Fprintln(logOut, "REQ", string(jsonBody))
	// Debug dump request
	if g.dumpRequestResponses {
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
	if g.verbose {
		log.Printf("Starting new conversation...")
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error connecting to Grok API: %w", err)
	}
	defer resp.Body.Close()

	// Debug dump response
	if g.dumpRequestResponses {
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

	if resp.StatusCode != http.StatusOK {
		// If the request failed due to Cloudflare protection, provide a helpful error message
		if resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("received 403 Forbidden response - Cloudflare protection activated. Your cookies have likely expired. Extract fresh cookies from grok.com and update XAI_SESSION_COOKIE environment variables")
		}
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	return g.streamResponseNewConversation(ctx, resp.Body, opts.StreamingFunc)
}

func (c *conversation) getConversationResponse(ctx context.Context, opts *llms.CallOptions, message llms.MessageContent) (*llms.ContentResponse, error) {
	if c.LastResponseID == "" {
		return nil, errors.New("grok: missing LastResponseID")
	}

	userMessage := partsToString(message.Parts)
	// Create request body
	requestBody := map[string]any{
		"message":       userMessage, // Include message content for follow-up prompts
		"modelName":     c.modelName,
		"disableSearch": false, // TODO: parameterize

		"enableImageGeneration": true,
		"enableImageStreaming":  true,
		"imageGenerationCount":  2,

		"imageAttachments":          []string{},
		"returnImageBytes":          false,
		"returnRawGrokInXaiRequest": false,
		"fileAttachments":           []string{},

		"forceConcise": false,

		"toolOverrides":     map[string]bool{},
		"enableSideBySide":  true,
		"sendFinalMetadata": true,
		//"deepsearchPreset":          "default",
		//"isReasoning": true,
		"webpageUrls": []string{},
		// "maxTokens":   g.maxTokens,
		// "temperature": g.temperature,

		"parentResponseId": c.LastResponseID,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}
	logOut := &greyWriter{w: os.Stderr}
	fmt.Fprintln(logOut, "REQ", string(jsonBody))

	// Build URL
	url := fmt.Sprintf("%s%s/responses", c.baseURL, c.ConversationID)
	referer := fmt.Sprintf("https://grok.com/chat/%s", c.ConversationID)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers and cookies
	c.setRequestHeaders(req)
	c.setCookies(req)
	req.Header.Set("Referer", referer)

	// Debug dump request
	if c.dumpRequestResponses {
		dump, err := httputil.DumpRequestOut(req, true)
		if err != nil {
			log.Printf("Error dumping request: %v", err)
		} else {
			log.Printf("Request dump:\n%s", string(dump))
		}
	}

	// Streaming mode
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error in streaming mode: %w", err)
	}
	defer resp.Body.Close()

	// Debug headers for streaming response
	if c.dumpRequestResponses {
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

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("received 403 Forbidden response in streaming mode - Cloudflare protection activated. Your cookies have likely expired. Extract fresh cookies from grok.com and update XAI_SESSION_COOKIE environment variables")
		}
		return nil, fmt.Errorf("streaming request failed with status: %s", resp.Status)
	}

	return c.streamResponseConversationContinuation(ctx, resp.Body, opts.StreamingFunc)
}

// make a writer wrapper that adds grey color to the output:
type greyWriter struct {
	w io.Writer
}

func (g *greyWriter) Write(p []byte) (n int, err error) {
	// Add grey color to the output
	_, err = fmt.Fprintf(g.w, "\x1b[90m%s\x1b[0m", p)
	return len(p), err
}

var errSkipLine = errors.New("skip line")

// streamResponseNewConversation processes a streaming response body for new conversations
func (g *conversation) streamResponseNewConversation(ctx context.Context, body io.Reader, streamFunc func(context.Context, []byte) error) (*llms.ContentResponse, error) {
	return g.streamResponse(ctx, body, func(ctx context.Context, line string) (token string, additionalData interface{}, err error) {
		var response CreateConversationResponse
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			return "", nil, fmt.Errorf("skipping line: %w - %w", errSkipLine, err)
		}
		if g.ConversationID == "" && response.Result.Conversation.ConversationID != "" {
			g.ConversationID = response.Result.Conversation.ConversationID
		}
		if response.Result.Response.StreamingImageGenerationResponse.ImageID != "" {
			return response.Result.Response.Token, response.Result.Response.StreamingImageGenerationResponse, nil
		}
		if response.Result.Response.ModelResponse.ResponseID != "" {
			g.LastResponseID = response.Result.Response.ModelResponse.ResponseID
		}
		return response.Result.Response.Token, nil, nil
	}, streamFunc)
}

// streamResponseConversationContinuation processes a streaming response body for continued conversations
func (g *conversation) streamResponseConversationContinuation(ctx context.Context, body io.Reader, streamFunc func(context.Context, []byte) error) (*llms.ContentResponse, error) {
	return g.streamResponse(ctx, body, func(ctx context.Context, line string) (token string, additionalData interface{}, err error) {
		var response ContinueConversationResponse
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			return "", nil, fmt.Errorf("skipping line: %w - %w", errSkipLine, err)
		}
		token = response.Result.Token
		if token == "" && response.Result.Response.Token != "" {
			token = response.Result.Response.Token
		}
		if response.Result.Response.StreamingImageGenerationResponse.ImageID != "" {
			return token, response.Result.Response.StreamingImageGenerationResponse, nil
		}
		var data interface{}
		if len(response.Result.WebSearchResults.Results) > 0 {
			data = response.Result.WebSearchResults
		} else if len(response.Result.XSearchResults.Results) > 0 {
			data = response.Result.XSearchResults
		}
		return token, data, nil
	}, streamFunc)
}

// parseResponse parses a response from bytes into a ContentResponse
func (g *conversation) parseResponse(data []byte, streamFunc func(context.Context, []byte) error) (*llms.ContentResponse, error) {
	reader := bytes.NewReader(data)
	return g.streamResponse(context.Background(), reader, func(ctx context.Context, line string) (token string, additionalData interface{}, err error) {
		// Try to parse as CreateConversationResponse first
		var createResp CreateConversationResponse
		if err := json.Unmarshal([]byte(line), &createResp); err == nil {
			if createResp.Result.Response.Token != "" {
				return createResp.Result.Response.Token, nil, nil
			}
		}

		// Try to parse as ContinueConversationResponse
		var contResp ContinueConversationResponse
		if err := json.Unmarshal([]byte(line), &contResp); err == nil {
			token = contResp.Result.Token
			if token == "" && contResp.Result.Response.Token != "" {
				token = contResp.Result.Response.Token
			}
			return token, nil, nil
		}

		// If we can't parse either format, skip this line
		return "", nil, fmt.Errorf("skipping line: %w", errSkipLine)
	}, streamFunc)
}

// lineHandler returns token and additional data
type lineHandler func(ctx context.Context, line string) (token string, additionalData interface{}, err error)

// streamResponse is the generic streaming response processor
func (g *conversation) streamResponse(ctx context.Context, body io.Reader, lineHandler lineHandler, streamFunc func(context.Context, []byte) error) (*llms.ContentResponse, error) {
	var fullResponse strings.Builder

	if g.veryVerbose {
		body = io.TeeReader(body, &greyWriter{w: os.Stderr})
	}

	scanner := bufio.NewScanner(body)
	if g.verbose {
		log.Println("starting response stream...")
	}

	for scanner.Scan() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		line := scanner.Text()
		// if g.verbose {
		// 	fmt.Fprintln(logOut, "", line)
		// }

		if !strings.HasPrefix(line, "{") {
			log.Println("skipping line:", line)
			continue
		}
		if g.verbose {
			log.Printf("Stream data: %s", line)
		}

		token, additionalData, err := lineHandler(ctx, line)
		if err != nil {
			if errors.Is(err, errSkipLine) {
				continue
			}
			return nil, fmt.Errorf("error processing line: %w", err)
		}

		if token != "" {
			fullResponse.WriteString(token)
			if streamFunc != nil {
				if err := streamFunc(ctx, []byte(token)); err != nil {
					log.Printf("Error in streaming callback: %v", err)
				}
			}
		}

		// Handle additional data
		if additionalData != nil {
			g.handleAdditionalData(ctx, additionalData)
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

// handleAdditionalData processes additional data from the streaming response
func (g *conversation) handleAdditionalData(ctx context.Context, additionalData interface{}) {
	switch data := additionalData.(type) {
	case StreamingImageGenerationResponse:
		g.handleStreamingImageGeneration(ctx, data)

	case WebSearchResults:
		if g.verbose {
			log.Println("Received web search results:")
			for i, result := range data.Results {
				log.Printf("Web Result %d: %s - %s", i+1, result.Title, result.URL)
			}
		}
	case XSearchResults:
		if g.verbose {
			log.Println("Received X search results:")
			for i, result := range data.Results {
				log.Printf("X Result %d: %s (@%s) - %s", i+1, result.Name, result.Username, result.Text)
			}
		}
	}
}

// handleStreamingImageGeneration handles the additional data for streaming image generation
func (g *conversation) handleStreamingImageGeneration(ctx context.Context, data StreamingImageGenerationResponse) {
	// TODO: add conversation graph concept+capabilities.
	if os.Getenv("XAI_ENABLE_IMAGES") == "1" {
		log.Printf("Received image data: %s", data.ImageID)
		//g.imageSet.AddImage(ctx, data)
	}
}

// AddResponse adds a response to an existing conversation.
func (c *conversation) AddResponse(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	// Apply call options
	opts := &llms.CallOptions{}
	for _, opt := range options {
		opt(opts)
	}
	createOptions, messageContent, err := llmMessagesToCreateOptions(ctx, opts, messages)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages to create options: %w", err)
	}

	// Create a new conversation if we don't have one yet
	if c.ConversationID == "" {
		return c.Create(ctx, opts, messageContent, createOptions...)
	}

	// Continue the existing conversation
	return c.getConversationResponse(ctx, opts, messageContent)
}

// setRequestHeaders sets common headers for Grok API requests to match successful curl requests
func setRequestHeaders(req *http.Request) {
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

// Image display functionality will be added in a separate PR
