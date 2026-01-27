package services

// Logger provides minimal logging abstraction for services.
type Logger interface {
	Printf(format string, v ...any)
}
