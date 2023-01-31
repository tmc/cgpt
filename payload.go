package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
)

const (
	defaultModel = "text-davinci-003"
)

type Payload struct {
	Model       string `json:"model"`
	Prompt      string `json:"prompt"`
	Temperature int    `json:"temperature"`
	MaxTokens   int    `json:"max_tokens"`
}

type ResponsePayload struct {
	ID      string  `json:"id,omitempty"`
	Created float64 `json:"created,omitempty"`
	Choices []struct {
		FinishReason string      `json:"finish_reason,omitempty"`
		Index        float64     `json:"index,omitempty"`
		Logprobs     interface{} `json:"logprobs,omitempty"`
		Text         string      `json:"text,omitempty"`
	} `json:"choices,omitempty"`
	Model  string `json:"model,omitempty"`
	Object string `json:"object,omitempty"`
	Usage  struct {
		CompletionTokens float64 `json:"completion_tokens,omitempty"`
		PromptTokens     float64 `json:"prompt_tokens,omitempty"`
		TotalTokens      float64 `json:"total_tokens,omitempty"`
	} `json:"usage,omitempty"`
}

func post(apiToken string, payload Payload) (*ResponsePayload, error) {
	if payload.Model == "" {
		payload.Model = defaultModel
	}
	if payload.MaxTokens == 0 {
		payload.MaxTokens = 20
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	body := bytes.NewReader(payloadBytes)

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/completions", body)
	if err != nil {
		return nil, err
	}
	if apiToken == "" {
		apiToken = os.Getenv("OPENAI_API_KEY")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiToken)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	var response ResponsePayload
	err = json.NewDecoder(r.Body).Decode(&response)
	if err != nil {
		return nil, err
	}
	return &response, nil
}
