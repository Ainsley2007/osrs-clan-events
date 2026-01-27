package osrs

import "fmt"

type PlayerNotFoundError struct {
	RSN string
}

func (e *PlayerNotFoundError) Error() string {
	return fmt.Sprintf("player not found: %s", e.RSN)
}

type RateLimitError struct {
	Message string
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded: %s", e.Message)
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (status %d): %s", e.StatusCode, e.Message)
}
