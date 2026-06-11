package osrs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	httpClient    *http.Client
	rateLimiter   *RateLimiter
	baseURL       string
	retryBackoff  time.Duration
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		rateLimiter:  NewRateLimiter(),
		baseURL:      "https://secure.runescape.com/m=hiscore_oldschool",
		retryBackoff: retryBackoffBase,
	}
}

const (
	maxRetries      = 3
	retryBackoffBase = 2 * time.Second
	maxBackoff       = 30 * time.Second
)

// retryableStatus returns true for transient server/gateway errors (502, 503, 504).
func retryableStatus(code int) bool {
	return code == http.StatusBadGateway || code == http.StatusServiceUnavailable || code == http.StatusGatewayTimeout
}

// sleepOrDone sleeps for d or until ctx is cancelled, whichever is first.
func sleepOrDone(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-ctx.Done():
	}
}

// parseRetryAfter reads the Retry-After header and returns the indicated wait duration.
// Returns 0 when the header is absent or unparseable.
func parseRetryAfter(resp *http.Response) time.Duration {
	val := resp.Header.Get("Retry-After")
	if val == "" {
		return 0
	}
	// Integer seconds
	var secs int64
	if _, err := fmt.Sscanf(val, "%d", &secs); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	// HTTP-date
	if t, err := http.ParseTime(val); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}

// backoffFor returns the delay to wait before attempt number `attempt` (0-indexed).
// It takes the larger of the server-suggested retryAfter and an exponential base delay,
// capped at maxBackoff.
func (c *Client) backoffFor(attempt int, retryAfter time.Duration) time.Duration {
	exp := c.retryBackoff * (1 << uint(attempt))
	d := exp
	if retryAfter > d {
		d = retryAfter
	}
	if d > maxBackoff {
		d = maxBackoff
	}
	return d
}

// sanitizeErrorMessage cleans up error messages, especially HTML error pages.
// If the message looks like HTML, it returns a concise summary instead.
func sanitizeErrorMessage(body []byte, statusCode int) string {
	bodyStr := strings.TrimSpace(string(body))
	
	// If it's empty or very short, return as-is
	if len(bodyStr) < 50 {
		return bodyStr
	}
	
	// Check if it looks like HTML (starts with <!doctype, <html, or contains common HTML tags)
	bodyLower := strings.ToLower(bodyStr)
	if strings.HasPrefix(bodyLower, "<!doctype") || 
	   strings.HasPrefix(bodyLower, "<html") ||
	   strings.Contains(bodyLower, "<html") ||
	   strings.Contains(bodyLower, "<body") {
		// Try to extract a meaningful error message from HTML
		if idx := strings.Index(bodyLower, "oops, something went wrong"); idx != -1 {
			return fmt.Sprintf("OSRS API returned %d: server error (HTML error page)", statusCode)
		}
		if idx := strings.Index(bodyLower, "error"); idx != -1 {
			// Try to find error text in common HTML error pages
			return fmt.Sprintf("OSRS API returned %d: server error (HTML error page)", statusCode)
		}
		return fmt.Sprintf("OSRS API returned %d: server error (HTML error page, %d bytes)", statusCode, len(body))
	}
	
	// If it's very long and not HTML, truncate it
	if len(bodyStr) > 500 {
		return bodyStr[:500] + "... (truncated)"
	}
	
	return bodyStr
}

func (c *Client) GetPlayerStats(ctx context.Context, rsn string) (*PlayerStats, error) {
	requestURL := fmt.Sprintf("%s/index_lite.json?player=%s", c.baseURL, url.QueryEscape(rsn))

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter wait failed: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to execute request: %w", err)
			if attempt < maxRetries {
				sleepOrDone(ctx, c.backoffFor(attempt, 0))
			}
			continue
		}

		switch resp.StatusCode {
		case http.StatusOK:
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return nil, fmt.Errorf("failed to read response body: %w", err)
			}

			var stats PlayerStats
			if err := json.Unmarshal(body, &stats); err != nil {
				return nil, fmt.Errorf("failed to decode JSON response: %w", err)
			}

			if stats.Name == "" {
				return nil, &APIError{
					StatusCode: resp.StatusCode,
					Message:    "invalid response: missing player name",
				}
			}

			return &stats, nil

		case http.StatusNotFound:
			resp.Body.Close()
			return nil, &PlayerNotFoundError{RSN: rsn}

		case http.StatusTooManyRequests:
			retryAfter := parseRetryAfter(resp)
			resp.Body.Close()
			lastErr = &RateLimitError{Message: "too many requests to OSRS API"}
			if attempt < maxRetries {
				sleepOrDone(ctx, c.backoffFor(attempt, retryAfter))
				continue
			}
			return nil, lastErr

		default:
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			sanitizedMsg := sanitizeErrorMessage(body, resp.StatusCode)
			lastErr = &APIError{
				StatusCode: resp.StatusCode,
				Message:    sanitizedMsg,
			}
			if retryableStatus(resp.StatusCode) && attempt < maxRetries {
				sleepOrDone(ctx, c.backoffFor(attempt, 0))
				continue
			}
			return nil, lastErr
		}
	}

	return nil, lastErr
}

func (c *Client) PlayerExists(ctx context.Context, rsn string) (bool, error) {
	_, err := c.GetPlayerStats(ctx, rsn)
	if err != nil {
		var notFoundErr *PlayerNotFoundError
		if errors.As(err, &notFoundErr) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
