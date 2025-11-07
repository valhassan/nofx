package logger

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

// TelegramHook implements logrus.Hook interface to push logs to Telegram
type TelegramHook struct {
	sender  *TelegramSender
	levels  []logrus.Level
	enabled bool
}

// NewTelegramHook creates a Telegram Hook
func NewTelegramHook(config *TelegramConfig) (*TelegramHook, error) {
	if !config.Enabled {
		return &TelegramHook{enabled: false}, nil
	}

	if config.BotToken == "" || config.ChatID == 0 {
		return nil, fmt.Errorf("telegram configuration incomplete: bot_token and chat_id cannot be empty")
	}

	// Create sender (using default parameters)
	sender, err := NewTelegramSender(config.BotToken, config.ChatID)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram sender: %w", err)
	}

	hook := &TelegramHook{
		sender:  sender,
		levels:  config.GetLogrusLevels(),
		enabled: true,
	}

	return hook, nil
}

// Levels returns log levels that need to trigger
func (h *TelegramHook) Levels() []logrus.Level {
	if !h.enabled {
		return []logrus.Level{}
	}
	return h.levels
}

// Fire is called when log is triggered
func (h *TelegramHook) Fire(entry *logrus.Entry) error {
	if !h.enabled {
		return nil
	}

	// Format message
	message := h.formatMessage(entry)

	// Send asynchronously (non-blocking)
	h.sender.SendAsync(message)

	return nil
}

// formatMessage formats log message for Telegram
func (h *TelegramHook) formatMessage(entry *logrus.Entry) string {
	// Level emoji
	levelEmoji := h.getLevelEmoji(entry.Level)

	// Basic information
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("%s *%s*: System Log Alert\n", levelEmoji, strings.ToUpper(entry.Level.String())))
	builder.WriteString(fmt.Sprintf("📝 Message: `%s`\n", escapeMarkdown(entry.Message)))

	// Field information
	if len(entry.Data) > 0 {
		builder.WriteString("📊 Fields:\n")
		for key, value := range entry.Data {
			builder.WriteString(fmt.Sprintf("  • %s: `%v`\n", key, value))
		}
	}

	// Caller location
	if entry.HasCaller() {
		file := entry.Caller.File
		// Only keep relative path
		if idx := strings.Index(file, "nofx/"); idx >= 0 {
			file = file[idx:]
		}
		builder.WriteString(fmt.Sprintf("📍 Location: `%s:%d`\n", file, entry.Caller.Line))
	} else {
		// If entry has no caller, get it manually
		if _, file, line, ok := runtime.Caller(8); ok {
			if idx := strings.Index(file, "nofx/"); idx >= 0 {
				file = file[idx:]
			}
			builder.WriteString(fmt.Sprintf("📍 Location: `%s:%d`\n", file, line))
		}
	}

	// Timestamp
	builder.WriteString(fmt.Sprintf("🕐 Time: `%s`", entry.Time.Format("2006-01-02 15:04:05")))

	return builder.String()
}

// getLevelEmoji gets emoji corresponding to log level
func (h *TelegramHook) getLevelEmoji(level logrus.Level) string {
	switch level {
	case logrus.PanicLevel:
		return "🔴"
	case logrus.FatalLevel:
		return "🔴"
	case logrus.ErrorLevel:
		return "🟠"
	case logrus.WarnLevel:
		return "🟡"
	case logrus.InfoLevel:
		return "🟢"
	case logrus.DebugLevel:
		return "🔵"
	default:
		return "⚪"
	}
}

// escapeMarkdown escapes Markdown special characters
func escapeMarkdown(text string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	)
	return replacer.Replace(text)
}

// Stop stops Hook (graceful shutdown)
func (h *TelegramHook) Stop() {
	if h.enabled && h.sender != nil {
		h.sender.Stop()
	}
}
