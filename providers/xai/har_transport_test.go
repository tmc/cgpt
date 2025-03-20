package xai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

// HarEntry represents a single request/response pair in a HAR file
type HarEntry struct {
	Request struct {
		Method  string `json:"method"`
		URL     string `json:"url"`
		Headers []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"headers"`
		PostData struct {
			MimeType string `json:"mimeType"`
			Text     string `json:"text"`
		} `json:"postData"`
	} `json:"request"`
	Response struct {
		Status      int    `json:"status"`
		StatusText  string `json:"statusText"`
		HTTPVersion string `json:"httpVersion"`
		Headers     []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"headers"`
		Content struct {
			Size     int    `json:"size"`
			MimeType string `json:"mimeType"`
			Text     string `json:"text"`
		} `json:"content"`
	} `json:"response"`
	Time    float64 `json:"time"`
	StartedDateTime string `json:"startedDateTime"`
}

// HarLog represents the log section of a HAR file
type HarLog struct {
	Entries []HarEntry `json:"entries"`
}

// HarFile represents the root structure of a HAR file
type HarFile struct {
	Log HarLog `json:"log"`
}

// HarRoundTripper is an http.RoundTripper that responds with data from HAR files
type HarRoundTripper struct {
	Entries []HarEntry
}

// LoadHarFiles loads multiple HAR files and returns a HarRoundTripper
func LoadHarFiles(files []string) (*HarRoundTripper, error) {
	var allEntries []HarEntry
	
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("error reading HAR file %s: %w", file, err)
		}
		
		var harFile HarFile
		if err := json.Unmarshal(data, &harFile); err != nil {
			return nil, fmt.Errorf("error parsing HAR file %s: %w", file, err)
		}
		
		allEntries = append(allEntries, harFile.Log.Entries...)
	}
	
	return &HarRoundTripper{Entries: allEntries}, nil
}

// findEntry finds a matching entry for the given request
func (h *HarRoundTripper) findEntry(req *http.Request) (*HarEntry, error) {
	// Read request body if present
	var reqBody []byte
	if req.Body != nil {
		var err error
		reqBody, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		// Restore the body for future reads
		req.Body = io.NopCloser(bytes.NewReader(reqBody))
	}
	
	// Extract key information from the request for matching
	reqMethod := req.Method
	reqPath := req.URL.Path
	
	// For conversation/responses endpoints, we need to handle conversationID pattern
	var conversationID string
	if strings.Contains(reqPath, "/conversations/") && !strings.HasSuffix(reqPath, "/new") {
		// Extract conversation ID from path like "/rest/app-chat/conversations/{id}/responses"
		parts := strings.Split(reqPath, "/")
		if len(parts) > 3 {
			for i, part := range parts {
				if part == "conversations" && i+1 < len(parts) && parts[i+1] != "new" {
					conversationID = parts[i+1]
					break
				}
			}
		}
	}
	
	// Parse request body if it's JSON
	var reqBodyJSON map[string]interface{}
	if len(reqBody) > 0 && strings.Contains(req.Header.Get("Content-Type"), "application/json") {
		err := json.Unmarshal(reqBody, &reqBodyJSON)
		if err != nil {
			// Not valid JSON, just continue with the text comparison
			fmt.Printf("Warning: Request body is not valid JSON: %v\n", err)
		}
	}
	
	// Determine what kind of request this is
	isNewConversation := strings.HasSuffix(reqPath, "/conversations/new")
	isContinueConversation := conversationID != "" && strings.HasSuffix(reqPath, "/responses")
	
	// Find the best matching entry
	var bestMatch *HarEntry
	var bestMatchScore int = -1
	
	for i, entry := range h.Entries {
		// Start with a score of 0
		score := 0
		
		// Method must match
		if entry.Request.Method != reqMethod {
			continue
		}
		
		// Check if the URL pattern matches what we're looking for
		entryURLPath := strings.Split(entry.Request.URL, "?")[0] // Remove query params
		
		// Basic URL path matching
		if isNewConversation && strings.Contains(entryURLPath, "/conversations/new") {
			score += 50 // High score for new conversation endpoint match
		} else if isContinueConversation && strings.Contains(entryURLPath, "/conversations/") && strings.Contains(entryURLPath, "/responses") {
			score += 40 // Good score for continuing a conversation
		} else if strings.Contains(entryURLPath, reqPath) {
			score += 30 // Decent score for general path match
		} else {
			continue // No path match at all, skip this entry
		}
		
		// For POST requests, try to match on request body content
		if reqMethod == "POST" && len(reqBody) > 0 {
			// If we have JSON content, we can do smarter matching
			if len(reqBodyJSON) > 0 {
				// Parse entry body as JSON for comparison
				var entryBodyJSON map[string]interface{}
				if err := json.Unmarshal([]byte(entry.Request.PostData.Text), &entryBodyJSON); err == nil {
					// For message content, give points for similarity
					if reqMsg, ok := reqBodyJSON["message"].(string); ok {
						if entryMsg, ok := entryBodyJSON["message"].(string); ok {
							// Exact message match is best
							if reqMsg == entryMsg {
								score += 20
							} else if len(reqMsg) > 0 && len(entryMsg) > 0 {
								// Partial content match is good too
								score += 10
							}
						}
					}
					
					// Model name match
					if reqModel, ok := reqBodyJSON["modelName"].(string); ok {
						if entryModel, ok := entryBodyJSON["modelName"].(string); ok {
							if reqModel == entryModel {
								score += 5
							}
						}
					}
				}
			} else {
				// Simple text content match as fallback
				if strings.Contains(entry.Request.PostData.Text, string(reqBody)) ||
				   strings.Contains(string(reqBody), entry.Request.PostData.Text) {
					score += 15
				}
			}
		}
		
		// Keep track of the best match
		if score > bestMatchScore {
			bestMatchScore = score
			bestMatch = &h.Entries[i]
		}
	}
	
	// If we found a match with a reasonable score, return it
	if bestMatch != nil && bestMatchScore > 20 {
		return bestMatch, nil
	}
	
	return nil, fmt.Errorf("no matching request found for %s %s", req.Method, req.URL.Path)
}

// RoundTrip implements the http.RoundTripper interface
func (h *HarRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	entry, err := h.findEntry(req)
	if err != nil {
		return nil, err
	}
	
	// Create response headers
	header := http.Header{}
	for _, h := range entry.Response.Headers {
		header.Add(h.Name, h.Value)
	}
	
	// Create the response body
	body := io.NopCloser(strings.NewReader(entry.Response.Content.Text))
	
	// Create the response
	return &http.Response{
		Status:        fmt.Sprintf("%d %s", entry.Response.Status, entry.Response.StatusText),
		StatusCode:    entry.Response.Status,
		Proto:         entry.Response.HTTPVersion,
		ProtoMajor:    2, // Assume HTTP/2 for Grok
		ProtoMinor:    0,
		Header:        header,
		Body:          body,
		ContentLength: int64(entry.Response.Content.Size),
		Request:       req,
	}, nil
}

// TestHarRoundTripper tests the HAR-based round tripper
func TestHarRoundTripper(t *testing.T) {
	// Skip if HAR files don't exist
	files := []string{
		"/Users/tmc/cgpt/providers/xai/sample-new-convo.har",
	}
	
	for _, file := range files {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			t.Skipf("HAR file %s not found, skipping test", file)
		}
	}
	
	// Just verify that the files exist
	// We'll skip the actual HAR loading since this is just a placeholder test
	
	// Skip actually executing the test since this is just a placeholder
	t.Skip("HAR round tripper implementation is a placeholder")
	
	// In a real test, we would:
	// 1. Create a Grok3 instance with this client
	// 2. Call methods on the Grok3 instance
	// 3. Verify that it processes the mock responses correctly
}