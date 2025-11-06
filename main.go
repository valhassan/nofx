package main

import (
	"encoding/json"
	"fmt"
	"log"
	"nofx/api"
	"nofx/auth"
	"nofx/config"
	"nofx/manager"
	"nofx/market"
	"nofx/pool"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
)

// LeverageConfig leverage configuration
type LeverageConfig struct {
	BTCETHLeverage  int `json:"btc_eth_leverage"`
	AltcoinLeverage int `json:"altcoin_leverage"`
}

// ConfigFile configuration file structure, only contains fields that need to be synced to database
type ConfigFile struct {
	AdminMode          bool              `json:"admin_mode"`
	BetaMode           bool              `json:"beta_mode"`
	APIServerPort      int               `json:"api_server_port"`
	UseDefaultCoins    bool              `json:"use_default_coins"`
	DefaultCoins       []string          `json:"default_coins"`
	CoinPoolAPIURL     string            `json:"coin_pool_api_url"`
	OITopAPIURL        string            `json:"oi_top_api_url"`
	MaxDailyLoss       float64           `json:"max_daily_loss"`
	MaxDrawdown        float64           `json:"max_drawdown"`
	StopTradingMinutes int               `json:"stop_trading_minutes"`
	Leverage           LeverageConfig    `json:"leverage"`
	JWTSecret          string            `json:"jwt_secret"`
	DataKLineTime      string            `json:"data_k_line_time"`
	Log                *config.LogConfig `json:"log"` // log configuration
}

// loadConfigFile reads and parses config.json file
func loadConfigFile() (*ConfigFile, error) {
	// Check if config.json exists
	if _, err := os.Stat("config.json"); os.IsNotExist(err) {
		log.Printf("📄 config.json does not exist, using default configuration")
		return &ConfigFile{}, nil
	}

	// Read config.json
	data, err := os.ReadFile("config.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read config.json: %w", err)
	}

	// Parse JSON
	var configFile ConfigFile
	if err := json.Unmarshal(data, &configFile); err != nil {
		return nil, fmt.Errorf("failed to parse config.json: %w", err)
	}

	return &configFile, nil
}

// syncConfigToDatabase syncs configuration to database
func syncConfigToDatabase(database *config.Database, configFile *ConfigFile) error {
	if configFile == nil {
		return nil
	}

	log.Printf("🔄 Starting to sync config.json to database...")

	// Sync each configuration item to database
	configs := map[string]string{
		"admin_mode":           fmt.Sprintf("%t", configFile.AdminMode),
		"beta_mode":            fmt.Sprintf("%t", configFile.BetaMode),
		"api_server_port":      strconv.Itoa(configFile.APIServerPort),
		"use_default_coins":    fmt.Sprintf("%t", configFile.UseDefaultCoins),
		"coin_pool_api_url":    configFile.CoinPoolAPIURL,
		"oi_top_api_url":       configFile.OITopAPIURL,
		"max_daily_loss":       fmt.Sprintf("%.1f", configFile.MaxDailyLoss),
		"max_drawdown":         fmt.Sprintf("%.1f", configFile.MaxDrawdown),
		"stop_trading_minutes": strconv.Itoa(configFile.StopTradingMinutes),
	}

	// Sync default_coins (convert to JSON string for storage)
	if len(configFile.DefaultCoins) > 0 {
		defaultCoinsJSON, err := json.Marshal(configFile.DefaultCoins)
		if err == nil {
			configs["default_coins"] = string(defaultCoinsJSON)
		}
	}

	// Sync leverage configuration
	if configFile.Leverage.BTCETHLeverage > 0 {
		configs["btc_eth_leverage"] = strconv.Itoa(configFile.Leverage.BTCETHLeverage)
	}
	if configFile.Leverage.AltcoinLeverage > 0 {
		configs["altcoin_leverage"] = strconv.Itoa(configFile.Leverage.AltcoinLeverage)
	}

	// If JWT secret is not empty, also sync it
	if configFile.JWTSecret != "" {
		configs["jwt_secret"] = configFile.JWTSecret
	}

	// Update database configuration
	for key, value := range configs {
		if err := database.SetSystemConfig(key, value); err != nil {
			log.Printf("⚠️  Failed to update config %s: %v", key, err)
		} else {
			log.Printf("✓ Synced config: %s = %s", key, value)
		}
	}

	log.Printf("✅ config.json sync completed")
	return nil
}

// loadBetaCodesToDatabase loads beta code file to database
func loadBetaCodesToDatabase(database *config.Database) error {
	betaCodeFile := "beta_codes.txt"

	// Check if beta code file exists
	if _, err := os.Stat(betaCodeFile); os.IsNotExist(err) {
		log.Printf("📄 Beta code file %s does not exist, skipping load", betaCodeFile)
		return nil
	}

	// Get file information
	fileInfo, err := os.Stat(betaCodeFile)
	if err != nil {
		return fmt.Errorf("failed to get beta code file info: %w", err)
	}

	log.Printf("🔄 Found beta code file %s (%.1f KB), starting to load...", betaCodeFile, float64(fileInfo.Size())/1024)

	// Load beta codes to database
	err = database.LoadBetaCodesFromFile(betaCodeFile)
	if err != nil {
		return fmt.Errorf("failed to load beta codes: %w", err)
	}

	// Display statistics
	total, used, err := database.GetBetaCodeStats()
	if err != nil {
		log.Printf("⚠️  Failed to get beta code statistics: %v", err)
	} else {
		log.Printf("✅ Beta code loading completed: Total %d, Used %d, Remaining %d", total, used, total-used)
	}

	return nil
}

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║ 🤖 AI Multi-Model Trading System - DeepSeek & Qwen         ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Load environment variables from .env file if present (for local/dev runs)
	// In Docker Compose, variables are injected by the runtime and this is harmless.
	_ = godotenv.Load()

	// Initialize configuration database
	dbPath := "config.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	// Read configuration file
	configFile, err := loadConfigFile()
	if err != nil {
		log.Fatalf("❌ Failed to read config.json: %v", err)
	}

	log.Printf("📋 Initializing configuration database: %s", dbPath)
	database, err := config.NewDatabase(dbPath)
	if err != nil {
		log.Fatalf("❌ Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Sync config.json to database
	if err := syncConfigToDatabase(database, configFile); err != nil {
		log.Printf("⚠️  Failed to sync config.json to database: %v", err)
	}

	// Load beta codes to database
	if err := loadBetaCodesToDatabase(database); err != nil {
		log.Printf("⚠️  Failed to load beta codes to database: %v", err)
	}

	// Get system configuration
	useDefaultCoinsStr, _ := database.GetSystemConfig("use_default_coins")
	useDefaultCoins := useDefaultCoinsStr == "true"
	apiPortStr, _ := database.GetSystemConfig("api_server_port")

	// Get admin mode configuration
	adminModeStr, _ := database.GetSystemConfig("admin_mode")
	adminMode := adminModeStr != "false" // Defaults to true

	// Set JWT secret
	jwtSecret, _ := database.GetSystemConfig("jwt_secret")
	if jwtSecret == "" {
		jwtSecret = "your-jwt-secret-key-change-in-production-make-it-long-and-random"
		log.Printf("⚠️  Using default JWT secret, recommended to configure in production environment")
	}
	auth.SetJWTSecret(jwtSecret)

	// Admin mode requires admin password, exit if missing
	if adminMode {
		adminPassword := os.Getenv("NOFX_ADMIN_PASSWORD")
		if adminPassword == "" {
			log.Fatalf("Admin mode is enabled but NOFX_ADMIN_PASSWORD is missing. Set NOFX_ADMIN_PASSWORD and restart.")
		}
		if err := auth.SetAdminPasswordFromPlain(adminPassword); err != nil {
			log.Fatalf("Failed to set admin password: %v", err)
		}
		auth.SetAdminMode(true)
		log.Printf("✓ Admin mode enabled. All API endpoints require admin authentication.")
	}

	log.Printf("✓ Configuration database initialized successfully")
	fmt.Println()

	// Read default coin list from database
	defaultCoinsJSON, _ := database.GetSystemConfig("default_coins")
	var defaultCoins []string

	if defaultCoinsJSON != "" {
		// Try to parse from JSON
		if err := json.Unmarshal([]byte(defaultCoinsJSON), &defaultCoins); err != nil {
			log.Printf("⚠️  Failed to parse default_coins configuration: %v, using hardcoded default values", err)
			defaultCoins = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT", "HYPEUSDT"}
		} else {
			log.Printf("✓ Loaded default coin list from database (%d coins): %v", len(defaultCoins), defaultCoins)
		}
	} else {
		// If not configured in database, use hardcoded default values
		defaultCoins = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT", "HYPEUSDT"}
		log.Printf("⚠️  default_coins not configured in database, using hardcoded default values")
	}

	pool.SetDefaultCoins(defaultCoins)
	// Set whether to use default mainstream coins
	pool.SetUseDefaultCoins(useDefaultCoins)
	if useDefaultCoins {
		log.Printf("✓ Default mainstream coin list enabled")
	}

	// Set coin pool API URL
	coinPoolAPIURL, _ := database.GetSystemConfig("coin_pool_api_url")
	if coinPoolAPIURL != "" {
		pool.SetCoinPoolAPI(coinPoolAPIURL)
		log.Printf("✓ AI500 coin pool API configured")
	}

	oiTopAPIURL, _ := database.GetSystemConfig("oi_top_api_url")
	if oiTopAPIURL != "" {
		pool.SetOITopAPI(oiTopAPIURL)
		log.Printf("✓ OI Top API configured")
	}

	// Create TraderManager
	traderManager := manager.NewTraderManager()

	// Load all traders from database to memory
	err = traderManager.LoadTradersFromDatabase(database)
	if err != nil {
		log.Fatalf("❌ Failed to load traders: %v", err)
	}

	// Get all trader configurations from database (for display, using default user)
	traders, err := database.GetTraders("default")
	if err != nil {
		log.Fatalf("❌ Failed to get trader list: %v", err)
	}

	// Display loaded trader information
	fmt.Println()
	fmt.Println("🤖 AI Trader Configurations in Database:")
	if len(traders) == 0 {
		fmt.Println("  • No configured traders, please create via Web interface")
	} else {
		for _, trader := range traders {
			status := "Stopped"
			if trader.IsRunning {
				status = "Running"
			}
			fmt.Printf("  • %s (%s + %s) - Initial Balance: %.0f USDT [%s]\n",
				trader.Name, strings.ToUpper(trader.AIModelID), strings.ToUpper(trader.ExchangeID),
				trader.InitialBalance, status)
		}
	}

	fmt.Println()
	fmt.Println("🤖 AI Full Decision-Making Mode:")
	fmt.Printf("   • AI will autonomously decide leverage for each trade (max 5x for altcoins, max 5x for BTC/ETH)\n")
	fmt.Println("  • AI will autonomously decide position size for each trade")
	fmt.Println("  • AI will autonomously set stop loss and take profit prices")
	fmt.Println("  • AI will make comprehensive analysis based on market data, technical indicators, and account status")
	fmt.Println()
	fmt.Println("⚠️  Risk Warning: AI automated trading has risks, recommend testing with small amounts!")
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	// Get API server port
	apiPort := 8080 // Default port
	if apiPortStr != "" {
		if port, err := strconv.Atoi(apiPortStr); err == nil {
			apiPort = port
		}
	}

	// Create and start API server
	apiServer := api.NewServer(traderManager, database, apiPort)
	go func() {
		if err := apiServer.Start(); err != nil {
			log.Printf("❌ API server error: %v", err)
		}
	}()

	// Start market data stream - default uses all coins set by traders, if no coins set, prioritize system defaults
	go market.NewWSMonitor(150).Start(database.GetCustomCoins())
	//go market.NewWSMonitor(150).Start([]string{}) // This is a usage example, pass empty to use all coins from market
	// Set graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// TODO: Start traders configured as running in database
	// traderManager.StartAll()

	// Wait for shutdown signal
	<-sigChan
	fmt.Println()
	fmt.Println()
	log.Println("📛 Received shutdown signal, stopping all traders...")
	traderManager.StopAll()

	fmt.Println()
	fmt.Println("👋 Thank you for using the AI Trading System!")
}
