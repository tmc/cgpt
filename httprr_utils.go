package cgpt

import (
	"bytes"
	"net/http"
	"regexp"
	"strings"
)

// scrubAuthHeaders removes or redacts Authorization headers from HTTP requests
func scrubAuthHeaders(req *http.Request) error {
	// Scrub Authorization header but preserve type
	if auth := req.Header.Get("Authorization"); auth != "" {
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 {
			req.Header.Set("Authorization", parts[0]+" REDACTED")
		} else {
			req.Header.Set("Authorization", "REDACTED")
		}
	}
	
	// Scrub other common API key headers
	req.Header.Del("X-API-Key")
	req.Header.Del("Api-Key")
	
	return nil
}

// scrubAPIKeys removes or redacts API keys from query parameters
func scrubAPIKeys(req *http.Request) error {
	// Scrub API keys from query parameters
	q := req.URL.Query()
	for key := range q {
		if strings.Contains(strings.ToLower(key), "key") || 
		   strings.Contains(strings.ToLower(key), "token") || 
		   strings.Contains(strings.ToLower(key), "secret") {
			q.Set(key, "REDACTED")
		}
	}
	req.URL.RawQuery = q.Encode()
	
	return nil
}

// scrubTokensFromResponse removes or redacts tokens from HTTP responses
func scrubTokensFromResponse(buf *bytes.Buffer) error {
	// Simple find/replace of common token patterns
	s := buf.String()
	
	// Replace any JWT tokens
	jwtPattern := regexp.MustCompile(`eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+`)
	s = jwtPattern.ReplaceAllString(s, "eyJREDACTED.REDACTED.REDACTED")
	
	// Replace UUID-like strings that might be session tokens
	uuidPattern := regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	s = uuidPattern.ReplaceAllString(s, "00000000-0000-0000-0000-000000000000")
	
	buf.Reset()
	buf.WriteString(s)
	return nil
}

// truncateString truncates a string to the specified length
func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length] + "..."
}