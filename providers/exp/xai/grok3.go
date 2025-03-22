package xai

// This file is based on the grok3.go file from the langchaingo project.
// Original source:

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"golang.org/x/net/http2"
)

// grok3 represents the Grok-3 model from xAI.
type grok3 struct {
	client *http.Client

	baseURL   string
	modelName string

	maxTokens   int
	temperature float64
	stream      bool

	sessionCookie string // sso cookie

	verbose              bool // Enable verbose logging
	veryVerbose          bool // Enable very verbose logging
	dumpRequestResponses bool // Enable dumping of request and response bodies

	requireHTTP2  bool          // Whether to require HTTP/2 for API connections
	clientTimeout time.Duration // Timeout for HTTP requests

	conversation *conversation
}

// GrokOption configures the grok3 instance.
type GrokOption func(*grok3)

func WithBaseURL(baseURL string) GrokOption {
	return func(g *grok3) {
		g.baseURL = baseURL
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

func WithConversationID(conversationID string) GrokOption {
	return func(g *grok3) {
		if g.conversation == nil {
			g.conversation = &conversation{}
		}
		g.conversation.ConversationID = conversationID
	}
}

func WithVerbose(verbose bool) GrokOption {
	return func(g *grok3) {
		g.verbose = verbose
	}
}

func WithSessionCookie(cookie string) GrokOption {
	return func(g *grok3) {
		g.sessionCookie = cookie
	}
}

func WithHTTPClient(client *http.Client) GrokOption {
	return func(g *grok3) {
		// TODO: force wrap this in h2
		log.Println("warning: setting client")
		g.client = client
	}
}

func WithRequireHTTP2(require bool) GrokOption {
	return func(g *grok3) {
		g.requireHTTP2 = require
	}
}

func WithTimeout(timeout time.Duration) GrokOption {
	return func(g *grok3) {
		// Store timeout to be applied when the client is created
		g.clientTimeout = timeout
	}
}

// NewGrok3 creates a new Grok-3 model instance.
// It requires HTTP/2 support to work properly with the Grok API.
func NewGrok3(options ...GrokOption) (*grok3, error) {
	g := &grok3{
		baseURL:       "https://grok.com/rest/app-chat/conversations/",
		modelName:     "grok-3",
		maxTokens:     4096,
		temperature:   0.05,
		stream:        true,             // Default to streaming mode for better UX
		requireHTTP2:  true,             // Default to requiring HTTP/2 for production use
		clientTimeout: 60 * time.Second, // Default to 60s timeout for streaming responses
		sessionCookie: os.Getenv("XAI_SESSION_COOKIE"),
	}

	for _, opt := range options {
		opt(g)
	}
	if os.Getenv("XAI_DEBUG") == "1" {
		g.dumpRequestResponses = true
	}
	if os.Getenv("XAI_VERY_VERBOSE") == "1" {
		g.veryVerbose = true
	}
	if g.client == nil {
		var err error
		customTransport := &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			ForceAttemptHTTP2: true,
		}
		// Explicitly configure HTTP/2 using the golang.org/x/net/http2 package
		err = http2.ConfigureTransport(customTransport)
		if err != nil {
			log.Printf("WARN: Failed to configure HTTP/2 transport: %v", err)
			return nil, err
		} else if g.verbose {
			log.Printf("Successfully configured HTTP/2 transport")
		}

		// Create client with the custom transport
		g.client = &http.Client{
			Transport: customTransport,
			Timeout:   g.clientTimeout, // Use the configured timeout
		}
	} else {
		log.Printf("WARN: Using custom HTTP client for XAI Grok API")

		// Try to enhance the transport with HTTP/2 if possible
		if transport, ok := g.client.Transport.(*http.Transport); ok {
			// Try to configure HTTP/2 on the existing transport
			err := http2.ConfigureTransport(transport)
			if err != nil {
				log.Printf("WARN: Failed to configure HTTP/2 on custom transport: %v", err)
			} else if g.verbose {
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
			if g.verbose {
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
	for _, msg := range messages[len(messages)-1:] {
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

	// Set up conversation
	if g.conversation == nil {
		g.conversation = g.newConversation(ctx)
	}
	
	// Handle conversation based on whether it already exists
	if g.conversation.ConversationID == "" {
		return g.StartConversation(ctx, opts, messageContent)
	} else {
		return g.ContinueConversation(ctx, opts, messageContent)
	}
}

// WithClient updates the HTTP client and returns the model for chaining.
func (g *grok3) WithClient(client *http.Client) llms.Model {
	if client != nil {
		g.client = client
		// TODO: force h2
	}
	return g
}

// StartConversation creates a new conversation and delegates to the conversation's Create method
func (g *grok3) StartConversation(ctx context.Context, opts *llms.CallOptions, message string) (*llms.ContentResponse, error) {
	if g.conversation == nil {
		g.conversation = g.newConversation(ctx)
	}
	return g.conversation.Create(ctx, opts, message)
}

// ParseResponse delegates to the conversation's parseResponse method
func (g *grok3) ParseResponse(data []byte, streamFunc func(context.Context, []byte) error) (*llms.ContentResponse, error) {
	if g.conversation == nil {
		g.conversation = g.newConversation(context.Background())
	}
	return g.conversation.parseResponse(data, streamFunc)
}

// StreamResponse delegates to the conversation's streamResponse method
func (g *grok3) StreamResponse(ctx context.Context, body io.Reader, lineHandler lineHandler, streamFunc func(context.Context, []byte) error) (*llms.ContentResponse, error) {
	if g.conversation == nil {
		g.conversation = g.newConversation(ctx)
	}
	return g.conversation.streamResponse(ctx, body, lineHandler, streamFunc)
}

// Name returns the model name.
func (g *grok3) Name() string {
	return g.modelName
}

// GetConversationID returns the current conversation ID
func (g *grok3) GetConversationID() string {
	if g.conversation == nil {
		return ""
	}
	return g.conversation.ConversationID
}

// ContinueConversation delegates to the conversation's getConversationResponse method
func (g *grok3) ContinueConversation(ctx context.Context, opts *llms.CallOptions, message string) (*llms.ContentResponse, error) {
	if g.conversation == nil {
		g.conversation = g.newConversation(ctx)
	}
	return g.conversation.getConversationResponse(ctx, opts, message)
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

// setRequestHeaders sets common headers for Grok API requests to match successful curl requests
func (g *grok3) setRequestHeaders(req *http.Request) {
	// Delegate to the common function
	setRequestHeaders(req)
}

type ConversationOption func(*conversation)

// newConversation creates a new conversation instance with options
func (g *grok3) newConversation(ctx context.Context, options ...ConversationOption) *conversation {
	c := &conversation{
		client:              g.client,
		modelName:           g.modelName,
		baseURL:             g.baseURL,
		verbose:             g.verbose,
		veryVerbose:         g.veryVerbose,
		dumpRequestResponses: g.dumpRequestResponses,
		setRequestHeaders:   setRequestHeaders,
		setCookies:          g.setCookies,
	}
	for _, opt := range options {
		opt(c)
	}
	return c
}