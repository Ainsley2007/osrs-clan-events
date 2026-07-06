package logging

import "strings"

const (
	ansiBold     = "\033[1m"
	ansiHiCyan   = "\033[96m"
	ansiHiBlue   = "\033[94m"
	ansiMagenta  = "\033[35m"
)

func highlight(msg string, enable bool) string {
	if !enable || msg == "" {
		return msg
	}

	lines := strings.Split(msg, "\n")
	for i, line := range lines {
		if colored := colorizeLine(line); colored != line {
			lines[i] = colored
		}
	}
	return strings.Join(lines, "\n")
}

func colorizeLine(line string) string {
	switch {
	// Errors and critical paths
	case strings.Contains(line, "CRITICAL"):
		return paint(line, ansiRed)
	case strings.HasPrefix(line, "Error ") || strings.Contains(line, "Error getting"):
		return paint(line, ansiRed)
	case strings.HasPrefix(line, "Failed to") || strings.Contains(line, " failed:") || strings.Contains(line, " failed for "):
		return paint(line, ansiRed)
	case strings.Contains(line, "fetch failed"):
		return paint(line, ansiRed)
	case strings.Contains(line, "pb approval failed") || strings.Contains(line, "pb rejection failed"):
		return paint(line, ansiRed)

	// Success
	case strings.Contains(line, "fetch success"):
		return paint(line, ansiGreen)
	case strings.Contains(line, "Successfully") || strings.Contains(line, " successfully"):
		return paint(line, ansiGreen)
	case strings.Contains(line, "initialized successfully"):
		return paint(line, ansiGreen)
	case strings.Contains(line, "Logged in as"):
		return paint(line, ansiGreen)
	case strings.Contains(line, "Rollover commit complete"):
		return paint(line, ansiBold+ansiGreen)
	case strings.Contains(line, "Rollover Log sent"):
		return paint(line, ansiGreen)
	case strings.Contains(line, "Creating initial snapshots"):
		return paint(line, ansiGreen)
	case strings.Contains(line, "cleanup: complete"):
		return paint(line, ansiGreen)

	// Warnings and deferrals
	case strings.Contains(line, "WARNING"):
		return paint(line, ansiYellow)
	case strings.HasPrefix(line, "Warning:"):
		return paint(line, ansiYellow)
	case strings.Contains(line, "Deferring rollover"):
		return paint(line, ansiYellow)
	case strings.Contains(line, "Found ") && strings.Contains(line, "expired"):
		return paint(line, ansiYellow)
	case strings.Contains(line, "became unavailable"):
		return paint(line, ansiYellow)

	// Scheduler lifecycle
	case strings.Contains(line, "Starting scheduler"):
		return paint(line, ansiBold+ansiHiCyan)
	case strings.Contains(line, "Scheduler started"):
		return paint(line, ansiHiCyan)
	case strings.Contains(line, "Stopping scheduler") || strings.Contains(line, "checker stopped") || strings.Contains(line, "updater stopped"):
		return paint(line, ansiDim)
	case strings.Contains(line, "Taking initial snapshot"):
		return paint(line, ansiDim)

	// Rollover
	case strings.Contains(line, "Starting rollover"):
		return paint(line, ansiBold+ansiHiCyan)
	case strings.Contains(line, "Shutting down"):
		return paint(line, ansiMagenta)

	// Hourly / snapshots
	case strings.HasPrefix(line, "Hourly snapshot update:"):
		return paint(line, ansiHiCyan)
	case strings.Contains(line, "Updating snapshots for"):
		return paint(line, ansiDim)
	case strings.Contains(line, " accounts failed:"):
		return paint(line, ansiYellow)

	// Metric selection blocks
	case strings.Contains(line, " selection"):
		return paint(line, ansiCyan)
	case strings.HasPrefix(line, "  weights"):
		return paint(line, ansiDim)
	case strings.Contains(line, "<- picked"):
		return paint(line, ansiGreen)
	case strings.HasPrefix(line, "  roll:"):
		return paint(line, ansiYellow)
	case strings.HasPrefix(line, "  pool:") || strings.HasPrefix(line, "  recent:"):
		return paint(line, ansiDim)

	// Guild-scoped lines
	case strings.HasPrefix(line, "[Guild "):
		return colorizeGuildLine(line)

	// Guild / lifecycle events
	case strings.Contains(line, "Bot removed from guild"):
		return paint(line, ansiMagenta)
	case strings.Contains(line, "Startup guild cleanup:"):
		return paint(line, ansiHiBlue)
	case strings.Contains(line, "Guild event received"):
		return paint(line, ansiHiBlue)
	case strings.Contains(line, "Discord bot is running"):
		return paint(line, ansiHiCyan)
	}

	return line
}

func colorizeGuildLine(line string) string {
	switch {
	case strings.Contains(line, "CRITICAL") || strings.Contains(line, "Failed") || strings.Contains(line, "failed"):
		return paint(line, ansiRed)
	case strings.Contains(line, "Starting rollover"):
		return paint(line, ansiBold+ansiHiCyan)
	case strings.Contains(line, "Rollover commit complete") || strings.Contains(line, "Rollover Log sent"):
		return paint(line, ansiGreen)
	case strings.Contains(line, "Deferring rollover"):
		return paint(line, ansiYellow)
	case strings.Contains(line, "Renamed category"):
		return paint(line, ansiCyan)
	case strings.Contains(line, "Created category") || strings.Contains(line, "Created channel") || strings.Contains(line, "Created message"):
		return paint(line, ansiGreen)
	case strings.Contains(line, "already in progress"):
		return paint(line, ansiDim)
	case strings.Contains(line, "Creating new guild entry"):
		return paint(line, ansiHiBlue)
	default:
		return paint(line, ansiHiBlue)
	}
}
