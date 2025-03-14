package cgpt

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

// WithLatencyInjector returns a function that wraps an existing http.RoundTripper
// to add artificial latency to requests
func WithLatencyInjector(latency time.Duration, failureRate float64) InferenceProviderOption {
	return func(mo *inferenceProviderOptions) {
		if mo.httpClient == nil {
			mo.httpClient = http.DefaultClient
		}
		
		originalTransport := mo.httpClient.Transport
		if originalTransport == nil {
			originalTransport = http.DefaultTransport
		}
		
		mo.httpClient.Transport = &artificialLatencyTransport{
			base:        originalTransport,
			latency:     latency,
			failureRate: failureRate,
		}
	}
}

// artificialLatencyTransport is an http.RoundTripper that adds artificial latency
// and can inject failures at a specified rate
type artificialLatencyTransport struct {
	base        http.RoundTripper
	latency     time.Duration
	failureRate float64
}

func (t *artificialLatencyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Determine failure type based on probability
	if t.failureRate > 0 && rand.Float64() < t.failureRate {
		failureType := rand.Float64()
		
		// 50% chance of client-side network error
		if failureType < 0.5 {
			return nil, fmt.Errorf("artificial client-side network error injected (failure rate: %.2f)", t.failureRate)
		} 
		// 50% chance of server rejection with error status code
		// Create a mock response with an error status code
		statusCode := chooseErrorStatusCode()
		mockResp := &http.Response{
			StatusCode: statusCode,
			Status:     fmt.Sprintf("%d Artificial Server Error", statusCode),
			Body:       http.NoBody,
			Header:     make(http.Header),
			Request:    req,
		}
		mockResp.Header.Set("Content-Type", "application/json")
		mockResp.Body = io.NopCloser(strings.NewReader(`{"error": "artificial_error", "message": "This is an artificial server error injected for testing"}`))
		
		return mockResp, nil
	}
	
	// Add artificial latency if requested
	if t.latency > 0 {
		time.Sleep(t.latency)
	}
	
	// Perform the actual request
	return t.base.RoundTrip(req)
}

// chooseErrorStatusCode returns a randomly selected HTTP error status code
func chooseErrorStatusCode() int {
	// Common error status codes for API failures
	errorCodes := []int{
		400, // Bad Request
		401, // Unauthorized
		403, // Forbidden
		429, // Too Many Requests
		500, // Internal Server Error
		502, // Bad Gateway
		503, // Service Unavailable
		504, // Gateway Timeout
	}
	
	return errorCodes[rand.Intn(len(errorCodes))]
}