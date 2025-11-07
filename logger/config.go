package logger

import (
	"github.com/sirupsen/logrus"
)

// Config log configuration (simplified version)
type Config struct {
	Level    string          `json:"level"`    // Log level: debug, info, warn, error (default: info)
	Telegram *TelegramConfig `json:"telegram"` // Telegram push configuration (optional)
}

// TelegramConfig Telegram push configuration (simplified version, advanced parameters use default values)
type TelegramConfig struct {
	Enabled  bool   `json:"enabled"`   // Whether to enable (default: false)
	BotToken string `json:"bot_token"` // Bot Token
	ChatID   int64  `json:"chat_id"`   // Chat ID
	MinLevel string `json:"min_level"` // Minimum log level, logs at this level and above will be pushed to Telegram (optional, default: error)
}

// SetDefaults sets default values
func (c *Config) SetDefaults() {
	if c.Level == "" {
		c.Level = "info"
	}
}

// GetLogrusLevels returns log levels to be pushed to Telegram
// Returns all log levels at and above the configured MinLevel
// If not configured or invalid, defaults to error, fatal, panic (backward compatible)
func (tc *TelegramConfig) GetLogrusLevels() []logrus.Level {
	// If not configured, use default value error (backward compatible)
	minLevelStr := tc.MinLevel
	if minLevelStr == "" {
		minLevelStr = "error"
	}

	// Parse configured log level
	minLevel, err := logrus.ParseLevel(minLevelStr)
	if err != nil {
		// If parsing fails, use default value error (backward compatible)
		minLevel = logrus.ErrorLevel
	}

	// Define all log levels (from high to low: panic, fatal, error, warn, info, debug)
	allLevels := []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}

	// Return all log levels greater than or equal to minLevel
	var result []logrus.Level
	for _, level := range allLevels {
		if level <= minLevel {
			result = append(result, level)
		}
	}

	return result
}
