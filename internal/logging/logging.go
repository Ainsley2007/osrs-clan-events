package logging

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

const timeFormat = "2006/01/02 15:04:05"

const (
	ansiReset  = "\033[0m"
	ansiCyan   = "\033[36m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiRed    = "\033[31m"
	ansiDim    = "\033[90m"
)

// Logger writes human-readable logs to stdout (optionally colored) and a plain rotating file.
type Logger struct {
	console  zerolog.Logger
	file     zerolog.Logger
	useColor bool
}

func New(filePath string) *Logger {
	lj := &lumberjack.Logger{
		Filename:   filePath,
		MaxSize:    50,
		MaxAge:     24,
		MaxBackups: 2,
	}

	useColor := isatty.IsTerminal(os.Stdout.Fd()) && os.Getenv("LOG_NO_COLOR") == ""

	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: timeFormat,
		NoColor:    true,
		PartsOrder: []string{zerolog.TimestampFieldName, zerolog.MessageFieldName},
	}
	fileWriter := zerolog.ConsoleWriter{
		Out:        lj,
		TimeFormat: timeFormat,
		NoColor:    true,
		PartsOrder: []string{zerolog.TimestampFieldName, zerolog.MessageFieldName},
	}

	return &Logger{
		console:  zerolog.New(consoleWriter).With().Timestamp().Logger(),
		file:     zerolog.New(fileWriter).With().Timestamp().Logger(),
		useColor: useColor,
	}
}

func (l *Logger) Printf(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	l.console.Info().Msg(highlight(msg, l.useColor))
	l.file.Info().Msg(msg)
}

// StdWriter adapts the logger for the standard library log package.
func (l *Logger) StdWriter() io.Writer {
	return writerFunc(func(p []byte) error {
		msg := strings.TrimRight(string(p), "\n")
		if msg == "" {
			return nil
		}
		l.Printf("%s", msg)
		return nil
	})
}

type writerFunc func([]byte) error

func (f writerFunc) Write(p []byte) (int, error) {
	return len(p), f(p)
}

func paint(line, code string) string {
	return code + line + ansiReset
}
