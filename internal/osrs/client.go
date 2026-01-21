package osrs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	httpClient  *http.Client
	rateLimiter *RateLimiter
	baseURL     string
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		rateLimiter: NewRateLimiter(),
		baseURL:     "https://secure.runescape.com/m=hiscore_oldschool",
	}
}

func (c *Client) GetPlayerStats(ctx context.Context, rsn string) (*PlayerStats, error) {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait failed: %w", err)
	}

	requestURL := fmt.Sprintf("%s/index_lite.json?player=%s", c.baseURL, url.QueryEscape(rsn))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		body, err := io.ReadAll(resp.Body)
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
		return nil, &PlayerNotFoundError{RSN: rsn}

	case http.StatusTooManyRequests:
		return nil, &RateLimitError{
			Message: "too many requests to OSRS API",
		}

	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}
	}
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
