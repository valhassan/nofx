package logger

import (
	"nofx/config"
	"os"

	"github.com/sirupsen/logrus"
)

var (
	// Log global logger instance
	Log *logrus.Logger

	// telegramHook saves hook reference for graceful shutdown
	telegramHook *TelegramHook
)

// ============================================================================
// Initialization functions
// ============================================================================

// Init initializes global logger
// If config is nil, uses default configuration (console output, info level)
func Init(cfg *Config) error {
	Log = logrus.New()

	// If no configuration, use default values
	if cfg == nil {
		cfg = &Config{Level: "info"}
	}

	// Set default values
	cfg.SetDefaults()

	// Set log level
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	Log.SetLevel(level)

	// Set formatter (fixed to use colored text format)
	Log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		ForceColors:     true,
	})

	// Set output target (default stdout)
	Log.SetOutput(os.Stdout)

	// Enable caller location information
	Log.SetReportCaller(true)

	// Add Telegram Hook (optional)
	if cfg.Telegram != nil && cfg.Telegram.Enabled {
		if err := setupTelegramHook(cfg.Telegram); err != nil {
			Log.Warnf("Failed to initialize Telegram push, will continue using regular logging: %v", err)
		}
	}

	return nil
}

// setupTelegramHook sets up Telegram Hook
func setupTelegramHook(telegramCfg *TelegramConfig) error {
	hook, err := NewTelegramHook(telegramCfg)
	if err != nil {
		return err
	}

	Log.AddHook(hook)
	telegramHook = hook
	Log.Info("✅ Telegram log push enabled")
	return nil
}

// InitWithSimpleConfig initializes logger with simplified configuration
// Suitable for scenarios that only need basic functionality
func InitWithSimpleConfig(level string) error {
	return Init(&Config{Level: level})
}

// InitWithTelegram initializes logger with Telegram configuration
func InitWithTelegram(botToken string, chatID int64) error {
	return Init(&Config{
		Level: "info",
		Telegram: &TelegramConfig{
			Enabled:  true,
			BotToken: botToken,
			ChatID:   chatID,
		},
	})
}

// InitFromLogConfig initializes logger from config.LogConfig
func InitFromLogConfig(logConfig *config.LogConfig) error {
	if logConfig == nil {
		return InitWithSimpleConfig("info")
	}

	cfg := &Config{
		Level: logConfig.Level,
	}

	if cfg.Level == "" {
		cfg.Level = "info"
	}

	// If Telegram is enabled, add configuration
	if logConfig.Telegram != nil && logConfig.Telegram.Enabled {
		if botToken := logConfig.Telegram.BotToken; botToken != "" && logConfig.Telegram.ChatID != 0 {
			cfg.Telegram = &TelegramConfig{
				Enabled:  true,
				BotToken: botToken,
				ChatID:   logConfig.Telegram.ChatID,
				MinLevel: logConfig.Telegram.MinLevel,
			}
		}
	}

	return Init(cfg)
}

// InitFromParams initializes logger from parameters
// Suitable for scenarios that don't depend on config package
func InitFromParams(level string, telegramEnabled bool, botToken string, chatID int64) error {
	cfg := &Config{Level: level}

	if telegramEnabled && botToken != "" && chatID != 0 {
		cfg.Telegram = &TelegramConfig{
			Enabled:  true,
			BotToken: botToken,
			ChatID:   chatID,
		}
	}

	return Init(cfg)
}

// Shutdown gracefully shuts down logger (mainly for closing Telegram sender)
func Shutdown() {
	if telegramHook != nil {
		telegramHook.Stop()
		telegramHook = nil
	}
}

// ============================================================================
// Logging functions
// ============================================================================

// WithFields creates a logger entry with fields
func WithFields(fields logrus.Fields) *logrus.Entry {
	return Log.WithFields(fields)
}

// WithField creates a logger entry with a single field
func WithField(key string, value interface{}) *logrus.Entry {
	return Log.WithField(key, value)
}

// add debug, info, warn
func Debug(args ...interface{}) {
	Log.Debug(args...)
}

func Info(args ...interface{}) {
	Log.Info(args...)
}

func Warn(args ...interface{}) {
	Log.Warn(args...)
}

func Debugf(format string, args ...interface{}) {
	Log.Debugf(format, args...)
}

func Infof(format string, args ...interface{}) {
	Log.Infof(format, args...)
}

func Warnf(format string, args ...interface{}) {
	Log.Warnf(format, args...)
}

func Error(args ...interface{}) {
	Log.Error(args...)
}

func Errorf(format string, args ...interface{}) {
	Log.Errorf(format, args...)
}

func Fatal(args ...interface{}) {
	Log.Fatal(args...)
}

func Fatalf(format string, args ...interface{}) {
	Log.Fatalf(format, args...)
}

func Panic(args ...interface{}) {
	Log.Panic(args...)
}

func Panicf(format string, args ...interface{}) {
	Log.Panicf(format, args...)
}
