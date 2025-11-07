package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// TraderConfig configuration for a single trader
type TraderConfig struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`  // Whether to enable this trader
	AIModel string `json:"ai_model"` // "qwen" or "deepseek"

	// Exchange platform selection (choose one)
	Exchange string `json:"exchange"` // "binance" or "hyperliquid"

	// Binance configuration
	BinanceAPIKey    string `json:"binance_api_key,omitempty"`
	BinanceSecretKey string `json:"binance_secret_key,omitempty"`

	// Hyperliquid configuration
	HyperliquidPrivateKey string `json:"hyperliquid_private_key,omitempty"`
	HyperliquidWalletAddr string `json:"hyperliquid_wallet_addr,omitempty"`
	HyperliquidTestnet    bool   `json:"hyperliquid_testnet,omitempty"`

	// Aster configuration
	AsterUser       string `json:"aster_user,omitempty"`        // Aster main wallet address
	AsterSigner     string `json:"aster_signer,omitempty"`      // Aster API wallet address
	AsterPrivateKey string `json:"aster_private_key,omitempty"` // Aster API wallet private key

	// AI configuration
	QwenKey     string `json:"qwen_key,omitempty"`
	DeepSeekKey string `json:"deepseek_key,omitempty"`

	// Custom AI API configuration (supports any OpenAI-compatible API)
	CustomAPIURL    string `json:"custom_api_url,omitempty"`
	CustomAPIKey    string `json:"custom_api_key,omitempty"`
	CustomModelName string `json:"custom_model_name,omitempty"`

	InitialBalance      float64 `json:"initial_balance"`
	ScanIntervalMinutes int     `json:"scan_interval_minutes"`
}

// LeverageConfig leverage configuration
type LeverageConfig struct {
	BTCETHLeverage  int `json:"btc_eth_leverage"` // Leverage multiplier for BTC and ETH (main account: 5-50 recommended, sub-account: ≤5)
	AltcoinLeverage int `json:"altcoin_leverage"` // Leverage multiplier for altcoins (main account: 5-20 recommended, sub-account: ≤5)
}

// LogConfig log configuration
type LogConfig struct {
	Level    string          `json:"level"`    // Log level: debug, info, warn, error (default: info)
	Telegram *TelegramConfig `json:"telegram"` // Telegram push configuration (optional)
}

// TelegramConfig Telegram push configuration (simplified, only essential fields)
type TelegramConfig struct {
	Enabled  bool   `json:"enabled"`   // Whether to enable (default: false)
	BotToken string `json:"bot_token"` // Bot Token
	ChatID   int64  `json:"chat_id"`   // Chat ID
	MinLevel string `json:"min_level"` // Minimum log level, logs at this level and above will be pushed to Telegram (optional, default: error)
}

// Config main configuration
type Config struct {
	Traders            []TraderConfig `json:"traders"`
	UseDefaultCoins    bool           `json:"use_default_coins"` // Whether to use default mainstream coin list
	DefaultCoins       []string       `json:"default_coins"`     // Default mainstream coin pool
	APIServerPort      int            `json:"api_server_port"`
	MaxDailyLoss       float64        `json:"max_daily_loss"`
	MaxDrawdown        float64        `json:"max_drawdown"`
	StopTradingMinutes int            `json:"stop_trading_minutes"`
	Leverage           LeverageConfig `json:"leverage"` // Leverage configuration
	Log                *LogConfig     `json:"log"`      // Log configuration (optional)
}

// LoadConfig loads configuration from file
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set default values: ensure using default coin list
	if !config.UseDefaultCoins {
		config.UseDefaultCoins = true
	}

	// Set default coin pool
	if len(config.DefaultCoins) == 0 {
		config.DefaultCoins = []string{
			"BTCUSDT",
			"ETHUSDT",
			"SOLUSDT",
			"BNBUSDT",
			"XRPUSDT",
			"DOGEUSDT",
			"ADAUSDT",
			"HYPEUSDT",
		}
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// Validate validates configuration validity
func (c *Config) Validate() error {
	if len(c.Traders) == 0 {
		return fmt.Errorf("at least one trader must be configured")
	}

	traderIDs := make(map[string]bool)
	for i, trader := range c.Traders {
		if trader.ID == "" {
			return fmt.Errorf("trader[%d]: ID cannot be empty", i)
		}
		if traderIDs[trader.ID] {
			return fmt.Errorf("trader[%d]: duplicate ID '%s'", i, trader.ID)
		}
		traderIDs[trader.ID] = true

		if trader.Name == "" {
			return fmt.Errorf("trader[%d]: Name cannot be empty", i)
		}
		if trader.AIModel != "qwen" && trader.AIModel != "deepseek" && trader.AIModel != "custom" {
			return fmt.Errorf("trader[%d]: ai_model must be 'qwen', 'deepseek' or 'custom'", i)
		}

		// Validate exchange platform configuration
		if trader.Exchange == "" {
			trader.Exchange = "binance" // Default to Binance
		}
		if trader.Exchange != "binance" && trader.Exchange != "hyperliquid" && trader.Exchange != "aster" {
			return fmt.Errorf("trader[%d]: exchange must be 'binance', 'hyperliquid' or 'aster'", i)
		}

		// Validate corresponding keys based on platform
		if trader.Exchange == "binance" {
			if trader.BinanceAPIKey == "" || trader.BinanceSecretKey == "" {
				return fmt.Errorf("trader[%d]: binance_api_key and binance_secret_key must be configured when using Binance", i)
			}
		} else if trader.Exchange == "hyperliquid" {
			if trader.HyperliquidPrivateKey == "" {
				return fmt.Errorf("trader[%d]: hyperliquid_private_key must be configured when using Hyperliquid", i)
			}
		} else if trader.Exchange == "aster" {
			if trader.AsterUser == "" || trader.AsterSigner == "" || trader.AsterPrivateKey == "" {
				return fmt.Errorf("trader[%d]: aster_user, aster_signer and aster_private_key must be configured when using Aster", i)
			}
		}

		if trader.AIModel == "qwen" && trader.QwenKey == "" {
			return fmt.Errorf("trader[%d]: qwen_key must be configured when using Qwen", i)
		}
		if trader.AIModel == "deepseek" && trader.DeepSeekKey == "" {
			return fmt.Errorf("trader[%d]: deepseek_key must be configured when using DeepSeek", i)
		}
		if trader.AIModel == "custom" {
			if trader.CustomAPIURL == "" {
				return fmt.Errorf("trader[%d]: custom_api_url must be configured when using custom API", i)
			}
			if trader.CustomAPIKey == "" {
				return fmt.Errorf("trader[%d]: custom_api_key must be configured when using custom API", i)
			}
			if trader.CustomModelName == "" {
				return fmt.Errorf("trader[%d]: custom_model_name must be configured when using custom API", i)
			}
		}
		if trader.InitialBalance <= 0 {
			return fmt.Errorf("trader[%d]: initial_balance must be greater than 0", i)
		}
		if trader.ScanIntervalMinutes <= 0 {
			trader.ScanIntervalMinutes = 3 // Default 3 minutes
		}
	}

	if c.APIServerPort <= 0 {
		c.APIServerPort = 8080 // Default port 8080
	}

	// Set leverage default values (adapted for Binance sub-account limits, max 5x)
	if c.Leverage.BTCETHLeverage <= 0 {
		c.Leverage.BTCETHLeverage = 5 // Default 5x (safe value, adapted for sub-accounts)
	}
	if c.Leverage.BTCETHLeverage > 5 {
		fmt.Printf("⚠️  Warning: BTC/ETH leverage set to %dx, may fail if using sub-account (sub-account limit ≤5x)\n", c.Leverage.BTCETHLeverage)
	}
	if c.Leverage.AltcoinLeverage <= 0 {
		c.Leverage.AltcoinLeverage = 5 // Default 5x (safe value, adapted for sub-accounts)
	}
	if c.Leverage.AltcoinLeverage > 5 {
		fmt.Printf("⚠️  Warning: Altcoin leverage set to %dx, may fail if using sub-account (sub-account limit ≤5x)\n", c.Leverage.AltcoinLeverage)
	}

	return nil
}

// GetScanInterval gets scan interval
func (tc *TraderConfig) GetScanInterval() time.Duration {
	return time.Duration(tc.ScanIntervalMinutes) * time.Minute
}
