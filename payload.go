package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// newCompletionPayload creates a new completion payload.
func newCompletionPayload(cfg *Config) *ChatCompletionPayload {
	p := &ChatCompletionPayload{
		Model: cfg.Model,
		N:     1,

		// Stream: true,
	}
	if p.Model == "" {
		p.Model = defaultModel
	}
	if p.MaxTokens == 0 {
		p.MaxTokens = defaultMaxTokens
	}
	return p
}

// ChatCompletionPayload is the request payload for the OpenAI API.
// See https://platform.openai.com/docs/api-reference/chat/create
type ChatCompletionPayload struct {
	Model       string                  `json:"model"`
	Messages    []ChatCompletionMessage `json:"messages"`
	Temperature *float64                `json:"temperature,omitempty"`
	TopP        *float64                `json:"top_p,omitempty"`
	N           int                     `json:"n,omitempty"`
	Stream      bool                    `json:"stream,omitempty"`
	Stop        string                  `json:"stop,omitempty"`

	MaxTokens        int      `json:"max_tokens,omitempty"`
	PresencePenalty  *float64 `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`

	Logprobs int `json:"logprobs,omitempty"`
}

// ChatCompletionMessage is the message payload for the OpenAI chat completion API.
type ChatCompletionMessage struct {
	Role    string `json:"role"` // one of "system", "assistant", "user"
	Content string `json:"content"`
}

func (p *ChatCompletionPayload) addMessage(role, content string) {
	p.Messages = append(p.Messages, ChatCompletionMessage{
		Role:    role,
		Content: content,
	})
}

func (p *ChatCompletionPayload) addSystemMessage(content string) {
	p.addMessage("system", content)
}

func (p *ChatCompletionPayload) addUserMessage(content string) {
	p.addMessage("user", content)
}

func (p *ChatCompletionPayload) addAssistantMessage(content string) {
	p.addMessage("assistant", content)
}

// ResponsePayload is the response payload from the OpenAI API.
// See https://beta.openai.com/docs/api-reference/create-completion.
type ResponsePayload struct {
	// ID is the ID of the completion.
	ID string `json:"id,omitempty"`
	// Created is the time at which the completion was created.
	Created float64 `json:"created,omitempty"`
	// Choices is the list of completions.
	Choices []struct {
		// FinishReason is the reason the completion finished.
		FinishReason string `json:"finish_reason,omitempty"`
		// Index is the index of the completion.
		Index float64 `json:"index,omitempty"`
		// Logprobs is the log probabilities of the tokens.
		Logprobs interface{} `json:"logprobs,omitempty"`
		Message  struct {
			Content string `json:"content,omitempty"`
			Role    string `json:"role,omitempty"`
		} `json:"message,omitempty"`
	} `json:"choices,omitempty"`
	// Model is the name of the model used.
	Model string `json:"model,omitempty"`
	// Object is the type of the response.
	Object string `json:"object,omitempty"`
	// Usage is the usage information about the completion.
	Usage struct {
		// CompletionTokens is the number of tokens used to complete the prompt.
		CompletionTokens float64 `json:"completion_tokens,omitempty"`
		// PromptTokens is the number of tokens used in the prompt.
		PromptTokens float64 `json:"prompt_tokens,omitempty"`
		// TotalTokens is the total number of tokens used.
		TotalTokens float64 `json:"total_tokens,omitempty"`
	} `json:"usage,omitempty"`
}

// performCompletion posts the request to the OpenAI API.
func performCompletion(ctx context.Context, apiToken string, payload *ChatCompletionPayload) (*ResponsePayload, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	defer spin()()

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	requestBody := bytes.NewReader(payloadBytes)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", requestBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiToken)
	// req.Header.Set("X-OpenAI-Client-User-Agent", `{"bindings_version": "0.27.4", "httplib": "requests", "lang": "python", "lang_version": "3.11.2", "platform": "macOS-13.3.1-arm64-i386-64bit", "publisher": "openai", "uname": "Darwin 22.4.0 Darwin Kernel Version 22.4.0: Mon Mar  6 21:01:02 PST 2023; root:xnu-8796.101.5~3/RELEASE_ARM64_T8112 arm64 i386"}`)
	// req.Header.Set("User-Agent", "OpenAI/v1 PythonBindings/0.27.4")

	// dump request
	/*
		dump, err := httputil.DumpRequest(req, true)
		if err != nil {
			return nil, err
		}
		fmt.Println(string(dump))
	*/

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	/*
		// dump response:
		dump, err = httputil.DumpResponse(response, true)
		if err != nil {
			return nil, err
		}
		fmt.Println(string(dump))
	*/

	responseBody, _ := io.ReadAll(response.Body)
	//fmt.Fprintln(os.Stderr, string(responseBody))
	var responsePayload ResponsePayload
	err = json.NewDecoder(bytes.NewReader(responseBody)).Decode(&responsePayload)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		errMsg := fmt.Sprintf("API request failed with status code %d, body: %v", response.StatusCode, string(responseBody))
		return nil, errors.New(errMsg)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &responsePayload, nil
}
