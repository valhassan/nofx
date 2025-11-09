# 📁 NOFX Codebase Structure Documentation

**Language:** [English](CODEBASE_STRUCTURE.md) | [中文](CODEBASE_STRUCTURE.zh-CN.md) *(Coming soon)*

Comprehensive guide to understanding the NOFX codebase organization, module structure, and architectural patterns.

---

## 📋 Table of Contents

1. [Executive Summary](#executive-summary)
2. [Directory Structure Deep Dive](#directory-structure-deep-dive)
3. [Backend Architecture](#backend-architecture)
4. [Frontend Architecture](#frontend-architecture)
5. [Data Flow Diagrams](#data-flow-diagrams)
6. [Key Design Patterns](#key-design-patterns)
7. [Technology Stack Details](#technology-stack-details)
8. [File Organization Patterns](#file-organization-patterns)

---

## Executive Summary

### Overview

NOFX is a full-stack AI trading platform built with a modular, microservice-inspired architecture. The system enables autonomous cryptocurrency trading using AI models (DeepSeek, Qwen, or custom APIs) across multiple exchanges (Binance, Hyperliquid, Aster DEX).

### Technology Stack Summary

**Backend:**
- **Language:** Go 1.25+
- **Framework:** Gin (HTTP API)
- **Database:** SQLite (modernc.org/sqlite)
- **Authentication:** JWT + 2FA (TOTP)
- **AI Integration:** OpenAI-compatible API clients

**Frontend:**
- **Framework:** React 18 + TypeScript 5
- **Build Tool:** Vite 6
- **Styling:** TailwindCSS 3
- **State Management:** Zustand 5
- **Data Fetching:** SWR 2
- **Charts:** Recharts 2

### Key Design Principles

1. **Interface-Based Abstraction:** Exchange implementations use unified interfaces
2. **Database-Driven Configuration:** All settings stored in SQLite, no JSON editing
3. **Modular Architecture:** Clear separation of concerns (trader, decision, market, api)
4. **Multi-Trader Support:** Concurrent execution of multiple trading instances
5. **Real-Time Monitoring:** WebSocket streams for market data, polling for UI updates

---

## Directory Structure Deep Dive

### Root-Level Organization

```
nofx/
├── main.go                    # Application entry point
├── config.json                # System configuration (synced to DB)
├── config.db                  # SQLite database (traders, models, exchanges)
├── go.mod / go.sum            # Go dependencies
│
├── api/                       # HTTP API layer
├── auth/                      # Authentication & authorization
├── config/                    # Database & configuration management
├── decision/                  # AI decision engine
├── logger/                    # Logging & performance tracking
├── manager/                   # Multi-trader orchestration
├── market/                    # Market data fetching & analysis
├── mcp/                       # Model Context Protocol (AI API client)
├── pool/                      # Coin pool management
├── trader/                    # Trading execution layer
│
├── decision_logs/             # Decision log storage (JSON files)
│   └── {trader_id}/
│       └── {timestamp}.json
│
├── web/                       # React frontend application
│   ├── src/
│   ├── public/
│   ├── package.json
│   └── vite.config.ts
│
├── docker/                    # Docker configuration files
├── docs/                      # Documentation
├── nginx/                     # Nginx configuration
├── prompts/                   # AI prompt templates
└── scripts/                   # Utility scripts
```

### Backend Modules

#### `api/` - HTTP API Layer
RESTful API server using Gin framework. Handles all frontend-backend communication.

**Key Files:**
- `server.go` - API server setup, route definitions, middleware

**Responsibilities:**
- HTTP request handling
- Authentication middleware (JWT)
- Trader management endpoints
- Market data endpoints
- Configuration endpoints

#### `trader/` - Trading Execution Layer
Core trading logic with multi-exchange support via unified interface.

**Key Files:**
- `auto_trader.go` - Main trading orchestrator (1600+ lines)
- `interface.go` - Unified trader interface (Strategy pattern)
- `binance_futures.go` - Binance Futures API implementation
- `hyperliquid_trader.go` - Hyperliquid DEX implementation
- `aster_trader.go` - Aster DEX implementation

**Responsibilities:**
- Trading cycle orchestration (every 3-5 minutes)
- Position management (open/close)
- Risk control enforcement
- Order execution
- Account status monitoring

#### `decision/` - AI Decision Engine
AI-powered trading decision making with historical feedback.

**Key Files:**
- `engine.go` - Decision logic, prompt generation, AI response parsing
- `prompt_manager.go` - Prompt template system

**Responsibilities:**
- Context building (account, positions, market data)
- Prompt generation (system + user prompts)
- AI API communication
- Decision parsing and validation
- Historical performance analysis integration

#### `market/` - Market Data System
Market data fetching, WebSocket streams, and technical indicator calculation.

**Key Files:**
- `data.go` - Market data fetching and technical indicators (TA-Lib)
- `api_client.go` - REST API client for market data
- `websocket_client.go` - WebSocket client for real-time streams
- `combined_streams.go` - Combined WebSocket streams (single connection)
- `monitor.go` - Market data cache and monitoring
- `types.go` - Market data type definitions

**Responsibilities:**
- K-line data fetching (3min, 4hour timeframes)
- Technical indicator calculation (EMA, MACD, RSI, ATR)
- WebSocket stream management
- Real-time price updates
- Open Interest tracking

#### `manager/` - Multi-Trader Orchestration
Manages multiple trader instances concurrently.

**Key Files:**
- `trader_manager.go` - Trader lifecycle management

**Responsibilities:**
- Trader creation, start, stop, restart
- Resource allocation
- Concurrent execution coordination
- Competition data caching

#### `config/` - Configuration & Database
SQLite database layer for all system configuration.

**Key Files:**
- `database.go` - Database operations, schema, migrations
- `config.go` - Configuration utilities

**Responsibilities:**
- Database schema management
- Trader configuration persistence
- AI model configuration
- Exchange configuration
- User management
- System settings

#### `auth/` - Authentication
JWT-based authentication with optional 2FA support.

**Key Files:**
- `auth.go` - JWT token management, password hashing, 2FA

**Responsibilities:**
- User authentication
- JWT token generation/validation
- 2FA (TOTP) support
- Admin mode enforcement
- Password hashing (bcrypt)

#### `mcp/` - Model Context Protocol
AI API client for OpenAI-compatible APIs.

**Key Files:**
- `client.go` - AI API communication

**Responsibilities:**
- AI API calls (DeepSeek, Qwen, custom)
- Message formatting
- Response parsing
- Error handling

#### `pool/` - Coin Pool Management
Aggregates coin lists from multiple sources (AI500, OI Top, defaults).

**Key Files:**
- `coin_pool.go` - Coin pool aggregation logic

**Responsibilities:**
- Coin list aggregation
- Source merging (AI500 + OI Top)
- Default coin list management
- Liquidity filtering

#### `logger/` - Logging System
Decision logging and performance tracking.

**Key Files:**
- `decision_logger.go` - Decision recording, performance analysis
- `logger.go` - General logging utilities
- `telegram_hook.go` - Telegram notification integration

**Responsibilities:**
- Decision log persistence (JSON files)
- Performance metrics calculation
- Historical trade analysis
- Win rate, P/L ratio, Sharpe ratio

### Frontend Modules (`web/src/`)

#### `components/` - UI Components
React components organized by feature.

**Key Components:**
- `AITradersPage.tsx` - Main trader management page
- `CompetitionPage.tsx` - Multi-AI competition dashboard
- `EquityChart.tsx` - Equity curve visualization
- `ComparisonChart.tsx` - Multi-trader comparison charts
- `TraderConfigModal.tsx` - Trader configuration dialog
- `Header.tsx` - Navigation header
- `LoginPage.tsx` / `RegisterPage.tsx` - Authentication pages
- `landing/` - Landing page components

#### `pages/` - Route-Level Components
Top-level page components for routing.

**Key Files:**
- `LandingPage.tsx` - Public landing page
- `FAQPage.tsx` - FAQ page

#### `contexts/` - React Context Providers
Global state management via React Context.

**Key Files:**
- `AuthContext.tsx` - Authentication state
- `LanguageContext.tsx` - Internationalization

#### `hooks/` - Custom React Hooks
Reusable React hooks.

**Key Files:**
- `useSystemConfig.ts` - System configuration hook
- `useCounterAnimation.ts` - Animation utilities
- `useGitHubStats.ts` - GitHub statistics

#### `lib/` - Library Code
API client, utilities, configuration.

**Key Files:**
- `api.ts` - API client wrapper (all backend endpoints)
- `config.ts` - Frontend configuration
- `utils.ts` - Utility functions

#### `types/` - TypeScript Definitions
Type definitions for API responses and data structures.

**Key Files:**
- `index.ts` - Main type definitions
- `types.ts` - Additional types

#### `i18n/` - Internationalization
Translation files and utilities.

**Key Files:**
- `translations.ts` - Translation mappings

---

## Backend Architecture

### Entry Point (`main.go`)

The application entry point handles initialization, configuration loading, and service startup.

**Initialization Flow:**

```159:349:main.go
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
```

**Key Steps:**
1. Load environment variables
2. Initialize SQLite database
3. Sync `config.json` to database
4. Load beta codes
5. Configure authentication (JWT, admin mode)
6. Initialize coin pool settings
7. Create TraderManager and load traders from database
8. Start API server (goroutine)
9. Start market data WebSocket monitor (goroutine)
10. Wait for shutdown signal

### API Layer (`api/server.go`)

The API server provides RESTful endpoints for frontend communication.

**Server Structure:**

```22:51:api/server.go
// Server HTTP API server
type Server struct {
	router        *gin.Engine
	traderManager *manager.TraderManager
	database      *config.Database
	port          int
}

// NewServer creates an API server
func NewServer(traderManager *manager.TraderManager, database *config.Database, port int) *Server {
	// Set to Release mode (reduce log output)
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()

	// Enable CORS
	router.Use(corsMiddleware())

	s := &Server{
		router:        router,
		traderManager: traderManager,
		database:      database,
		port:          port,
	}

	// Setup routes
	s.setupRoutes()

	return s
}
```

**Key Endpoint Groups:**
- `/api/health` - Health check
- `/api/admin-login` - Admin authentication
- `/api/register`, `/api/login` - User authentication (non-admin mode)
- `/api/models` - AI model configuration
- `/api/exchanges` - Exchange configuration
- `/api/traders` - Trader management (CRUD, start/stop)
- `/api/status`, `/api/account`, `/api/positions` - Trading data
- `/api/decisions`, `/api/statistics` - Performance data
- `/api/competition` - Competition leaderboard

### Trader System (`trader/`)

The trader system implements the core trading logic with a unified interface for multiple exchanges.

**Unified Interface:**

```3:53:trader/interface.go
// Trader is a unified trading interface
// Supports multiple trading platforms (Binance, Hyperliquid, etc.)
type Trader interface {
	// GetBalance gets the account balance
	GetBalance() (map[string]interface{}, error)

	// GetPositions gets all positions
	GetPositions() ([]map[string]interface{}, error)

	// OpenLong opens a long position
	OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error)

	// OpenShort opens a short position
	OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error)

	// CloseLong closes a long position (quantity=0 means close all)
	CloseLong(symbol string, quantity float64) (map[string]interface{}, error)

	// CloseShort closes a short position (quantity=0 means close all)
	CloseShort(symbol string, quantity float64) (map[string]interface{}, error)

	// SetLeverage sets the leverage
	SetLeverage(symbol string, leverage int) error

	// SetMarginMode sets the margin mode (true=cross margin, false=isolated margin)
	SetMarginMode(symbol string, isCrossMargin bool) error

	// GetMarketPrice gets the market price
	GetMarketPrice(symbol string) (float64, error)

	// SetStopLoss sets a stop loss order
	SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error

	// SetTakeProfit sets a take profit order
	SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error

	// CancelStopLossOrders cancels only stop loss orders (fixes BUG: doesn't delete take profit when adjusting stop loss)
	CancelStopLossOrders(symbol string) error

	// CancelTakeProfitOrders cancels only take profit orders (fixes BUG: doesn't delete stop loss when adjusting take profit)
	CancelTakeProfitOrders(symbol string) error

	// CancelAllOrders cancels all pending orders for this symbol
	CancelAllOrders(symbol string) error

	// CancelStopOrders cancels stop loss/take profit orders for this symbol (used when adjusting stop loss/take profit levels)
	CancelStopOrders(symbol string) error

	// FormatQuantity formats the quantity to the correct precision
	FormatQuantity(symbol string, quantity float64) (string, error)
}
```

**AutoTrader Structure:**

```80:100:trader/auto_trader.go
// AutoTrader Auto trader
type AutoTrader struct {
	id                    string // Trader unique identifier
	name                  string // Trader display name
	aiModel               string // AI model name
	exchange              string // Exchange name
	config                AutoTraderConfig
	trader                Trader // Uses Trader interface (supports multiple platforms)
	mcpClient             *mcp.Client
	decisionLogger        *logger.DecisionLogger // Decision logger
	initialBalance        float64
	dailyPnL              float64
	customPrompt          string   // Custom trading strategy prompt
	overrideBasePrompt    bool     // Whether to override base prompt
	systemPromptTemplate  string   // System prompt template name
	defaultCoins          []string // Default coin list (obtained from database)
	tradingCoins          []string // Actual trading coin list
	lastResetTime         time.Time
	stopUntil             time.Time
	isRunning             bool
	startTime             time.Time          // System startup time
```

**Trading Cycle:**
1. Fetch account status
2. Get open positions
3. Fetch market data for candidate coins
4. Build decision context
5. Call AI decision engine
6. Parse and validate decisions
7. Execute orders (close existing first, then open new)
8. Log decisions and update performance

### Decision Engine (`decision/`)

The decision engine orchestrates AI-powered trading decisions with historical feedback.

**Decision Flow:**

```114:146:decision/engine.go
// GetFullDecision retrieves AI's complete trading decision (batch analysis of all coins and positions)
func GetFullDecision(ctx *Context, mcpClient *mcp.Client) (*FullDecision, error) {
	return GetFullDecisionWithCustomPrompt(ctx, mcpClient, "", false, "")
}

// GetFullDecisionWithCustomPrompt retrieves AI's complete trading decision (supports custom prompt and template selection)
func GetFullDecisionWithCustomPrompt(ctx *Context, mcpClient *mcp.Client, customPrompt string, overrideBase bool, templateName string) (*FullDecision, error) {
	// 1. Fetch market data for all coins
	if err := fetchMarketDataForContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to fetch market data: %w", err)
	}

	// 2. Build System Prompt (fixed rules) and User Prompt (dynamic data)
	systemPrompt := buildSystemPromptWithCustom(ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage, customPrompt, overrideBase, templateName)
	userPrompt := buildUserPrompt(ctx)

	// 3. Call AI API (using system + user prompt)
	aiResponse, err := mcpClient.CallWithMessages(systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to call AI API: %w", err)
	}

	// 4. Parse AI response
	decision, err := parseFullDecisionResponse(aiResponse, ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage)
	if err != nil {
		return decision, fmt.Errorf("failed to parse AI response: %w", err)
	}

	decision.Timestamp = time.Now()
	decision.SystemPrompt = systemPrompt // Save system prompt
	decision.UserPrompt = userPrompt     // Save input prompt
	return decision, nil
}
```

**Context Structure:**

```68:81:decision/engine.go
// Context represents trading context (complete information passed to AI)
type Context struct {
	CurrentTime     string                  `json:"current_time"`
	RuntimeMinutes  int                     `json:"runtime_minutes"`
	CallCount       int                     `json:"call_count"`
	Account         AccountInfo             `json:"account"`
	Positions       []PositionInfo          `json:"positions"`
	CandidateCoins  []CandidateCoin         `json:"candidate_coins"`
	MarketDataMap   map[string]*market.Data `json:"-"` // Not serialized, but used internally
	OITopDataMap    map[string]*OITopData   `json:"-"` // OI Top data mapping
	Performance     interface{}             `json:"-"` // Historical performance analysis (logger.PerformanceAnalysis)
	BTCETHLeverage  int                     `json:"-"` // BTC/ETH leverage multiplier (read from config)
	AltcoinLeverage int                     `json:"-"` // Altcoin leverage multiplier (read from config)
}
```

### Market Data System (`market/`)

The market data system provides real-time and historical market data with technical indicators.

**Key Responsibilities:**
- K-line data fetching (3min, 4hour)
- Technical indicator calculation (EMA, MACD, RSI, ATR)
- WebSocket stream management
- Real-time price caching
- Open Interest tracking

### Manager (`manager/`)

The TraderManager orchestrates multiple trader instances.

**Manager Structure:**

```24:39:manager/trader_manager.go
// TraderManager manages multiple trader instances
type TraderManager struct {
	traders          map[string]*trader.AutoTrader // key: trader ID
	competitionCache *CompetitionCache
	mu               sync.RWMutex
}

// NewTraderManager creates a trader manager
func NewTraderManager() *TraderManager {
	return &TraderManager{
		traders: make(map[string]*trader.AutoTrader),
		competitionCache: &CompetitionCache{
			data: make(map[string]interface{}),
		},
	}
}
```

**Key Methods:**
- `LoadTradersFromDatabase()` - Load all traders from database
- `StartTrader()` - Start a trader instance
- `StopTrader()` - Stop a trader instance
- `GetTrader()` - Get trader by ID
- `GetAllTraders()` - Get all traders

### Configuration (`config/`)

The database layer manages all system configuration in SQLite.

**Database Schema:**

```44:150:config/database.go
// createTables creates database tables
func (d *Database) createTables() error {
	queries := []string{
		// AI model configuration table
		`CREATE TABLE IF NOT EXISTS ai_models (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			provider TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// Exchange configuration table
		`CREATE TABLE IF NOT EXISTS exchanges (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			type TEXT NOT NULL, -- 'cex' or 'dex'
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			secret_key TEXT DEFAULT '',
			testnet BOOLEAN DEFAULT 0,
			-- Hyperliquid specific fields
			hyperliquid_wallet_addr TEXT DEFAULT '',
			-- Aster specific fields
			aster_user TEXT DEFAULT '',
			aster_signer TEXT DEFAULT '',
			aster_private_key TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// User signal source configuration table
		`CREATE TABLE IF NOT EXISTS user_signal_sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			coin_pool_url TEXT DEFAULT '',
			oi_top_url TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id)
		)`,

		// Trader configuration table
		`CREATE TABLE IF NOT EXISTS traders (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			ai_model_id TEXT NOT NULL,
			exchange_id TEXT NOT NULL,
			initial_balance REAL NOT NULL,
			scan_interval_minutes INTEGER DEFAULT 3,
			is_running BOOLEAN DEFAULT 0,
			btc_eth_leverage INTEGER DEFAULT 5,
			altcoin_leverage INTEGER DEFAULT 5,
			trading_symbols TEXT DEFAULT '',
			use_coin_pool BOOLEAN DEFAULT 0,
			use_oi_top BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (ai_model_id) REFERENCES ai_models(id),
			FOREIGN KEY (exchange_id) REFERENCES exchanges(id)
		)`,

		// Users table
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			otp_secret TEXT,
			otp_verified BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// System configuration table
		`CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Beta codes table
		`CREATE TABLE IF NOT EXISTS beta_codes (
			code TEXT PRIMARY KEY,
			used BOOLEAN DEFAULT 0,
			used_by TEXT DEFAULT '',
			used_at DATETIME NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Triggers: automatically update updated_at
		`CREATE TRIGGER IF NOT EXISTS update_users_updated_at
			AFTER UPDATE ON users
			BEGIN
				UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_ai_models_updated_at
			AFTER UPDATE ON ai_models
			BEGIN
```

**Key Tables:**
- `users` - User accounts with 2FA support
- `ai_models` - AI model configurations
- `exchanges` - Exchange credentials
- `traders` - Trader instances
- `system_config` - System-wide settings
- `beta_codes` - Beta access codes

---

## Frontend Architecture

### Entry Points

**`main.tsx`** - Application bootstrap
- React DOM rendering
- Root component mounting

**`App.tsx`** - Main application component
- Route configuration
- Context providers (Auth, Language)
- Global layout

### Component Organization

Components are organized by feature:

- **Trading Pages:** `AITradersPage.tsx`, `CompetitionPage.tsx`
- **Charts:** `EquityChart.tsx`, `ComparisonChart.tsx`
- **Configuration:** `TraderConfigModal.tsx`, `TraderConfigViewModal.tsx`
- **Authentication:** `LoginPage.tsx`, `RegisterPage.tsx`, `ResetPasswordPage.tsx`
- **Landing:** `landing/` directory with multiple sections
- **FAQ:** `faq/` directory with search and sidebar

### API Client (`lib/api.ts`)

The API client provides a typed interface to all backend endpoints.

**Structure:**

```32:343:web/src/lib/api.ts
export const api = {
  // AI trader management API
  async getTraders(): Promise<TraderInfo[]> {
    const res = await fetch(`${API_BASE}/my-traders`, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get trader list')
    return res.json()
  },

  // Get public trader list (no authentication required)
  async getPublicTraders(): Promise<any[]> {
    const res = await fetch(`${API_BASE}/traders`)
    if (!res.ok) throw new Error('Failed to get public trader list')
    return res.json()
  },

  async createTrader(request: CreateTraderRequest): Promise<TraderInfo> {
    const res = await fetch(`${API_BASE}/traders`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify(request),
    })
    if (!res.ok) throw new Error('Failed to create trader')
    return res.json()
  },

  async deleteTrader(traderId: string): Promise<void> {
    const res = await fetch(`${API_BASE}/traders/${traderId}`, {
      method: 'DELETE',
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to delete trader')
  },

  async startTrader(traderId: string): Promise<void> {
    const res = await fetch(`${API_BASE}/traders/${traderId}/start`, {
      method: 'POST',
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to start trader')
  },

  async stopTrader(traderId: string): Promise<void> {
    const res = await fetch(`${API_BASE}/traders/${traderId}/stop`, {
      method: 'POST',
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to stop trader')
  },

  async updateTraderPrompt(
    traderId: string,
    customPrompt: string
  ): Promise<void> {
    const res = await fetch(`${API_BASE}/traders/${traderId}/prompt`, {
      method: 'PUT',
      headers: getAuthHeaders(),
      body: JSON.stringify({ custom_prompt: customPrompt }),
    })
    if (!res.ok) throw new Error('Failed to update custom strategy')
  },

  async getTraderConfig(traderId: string): Promise<any> {
    const res = await fetch(`${API_BASE}/traders/${traderId}/config`, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get trader configuration')
    return res.json()
  },

  async updateTrader(
    traderId: string,
    request: CreateTraderRequest
  ): Promise<TraderInfo> {
    const res = await fetch(`${API_BASE}/traders/${traderId}`, {
      method: 'PUT',
      headers: getAuthHeaders(),
      body: JSON.stringify(request),
    })
    if (!res.ok) throw new Error('Failed to update trader')
    return res.json()
  },

  // AI model configuration API
  async getModelConfigs(): Promise<AIModel[]> {
    const res = await fetch(`${API_BASE}/models`, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get model configuration')
    return res.json()
  },

  // Get system supported AI models list (no authentication required)
  async getSupportedModels(): Promise<AIModel[]> {
    const res = await fetch(`${API_BASE}/supported-models`)
    if (!res.ok) throw new Error('Failed to get supported models')
    return res.json()
  },

  async updateModelConfigs(request: UpdateModelConfigRequest): Promise<void> {
    const res = await fetch(`${API_BASE}/models`, {
      method: 'PUT',
      headers: getAuthHeaders(),
      body: JSON.stringify(request),
    })
    if (!res.ok) throw new Error('Failed to update model configuration')
  },

  // Exchange configuration API
  async getExchangeConfigs(): Promise<Exchange[]> {
    const res = await fetch(`${API_BASE}/exchanges`, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get exchange configuration')
    return res.json()
  },

  // Get system supported exchanges list (no authentication required)
  async getSupportedExchanges(): Promise<Exchange[]> {
    const res = await fetch(`${API_BASE}/supported-exchanges`)
    if (!res.ok) throw new Error('Failed to get supported exchanges')
    return res.json()
  },

  async updateExchangeConfigs(
    request: UpdateExchangeConfigRequest
  ): Promise<void> {
    const res = await fetch(`${API_BASE}/exchanges`, {
      method: 'PUT',
      headers: getAuthHeaders(),
      body: JSON.stringify(request),
    })
    if (!res.ok) throw new Error('Failed to update exchange configuration')
  },

  // Get system status (supports trader_id)
  async getStatus(traderId?: string): Promise<SystemStatus> {
    const url = traderId
      ? `${API_BASE}/status?trader_id=${traderId}`
      : `${API_BASE}/status`
    const res = await fetch(url, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get system status')
    return res.json()
  },

  // Get account information (supports trader_id)
  async getAccount(traderId?: string): Promise<AccountInfo> {
    const url = traderId
      ? `${API_BASE}/account?trader_id=${traderId}`
      : `${API_BASE}/account`
    const res = await fetch(url, {
      cache: 'no-store',
      headers: {
        ...getAuthHeaders(),
        'Cache-Control': 'no-cache',
      },
    })
    if (!res.ok) throw new Error('Failed to get account information')
    const data = await res.json()
    console.log('Account data fetched:', data)
    return data
  },

  // Get positions list (supports trader_id)
  async getPositions(traderId?: string): Promise<Position[]> {
    const url = traderId
      ? `${API_BASE}/positions?trader_id=${traderId}`
      : `${API_BASE}/positions`
    const res = await fetch(url, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get positions list')
    return res.json()
  },

  // Get decision logs (supports trader_id)
  async getDecisions(traderId?: string): Promise<DecisionRecord[]> {
    const url = traderId
      ? `${API_BASE}/decisions?trader_id=${traderId}`
      : `${API_BASE}/decisions`
    const res = await fetch(url, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get decision logs')
    return res.json()
  },

  // Get latest decisions (supports trader_id)
  async getLatestDecisions(traderId?: string): Promise<DecisionRecord[]> {
    const url = traderId
      ? `${API_BASE}/decisions/latest?trader_id=${traderId}`
      : `${API_BASE}/decisions/latest`
    const res = await fetch(url, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get latest decisions')
    return res.json()
  },

  // Get statistics (supports trader_id)
  async getStatistics(traderId?: string): Promise<Statistics> {
    const url = traderId
      ? `${API_BASE}/statistics?trader_id=${traderId}`
      : `${API_BASE}/statistics`
    const res = await fetch(url, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get statistics')
    return res.json()
  },

  // Get equity history data (supports trader_id)
  async getEquityHistory(traderId?: string): Promise<any[]> {
    const url = traderId
      ? `${API_BASE}/equity-history?trader_id=${traderId}`
      : `${API_BASE}/equity-history`
    const res = await fetch(url, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get historical data')
    return res.json()
  },

  // Batch get historical data for multiple traders (no authentication required)
  async getEquityHistoryBatch(traderIds: string[]): Promise<any> {
    const res = await fetch(`${API_BASE}/equity-history-batch`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ trader_ids: traderIds }),
    })
    if (!res.ok) throw new Error('Failed to get batch historical data')
    return res.json()
  },

  // Get top 5 traders data (no authentication required)
  async getTopTraders(): Promise<any[]> {
    const res = await fetch(`${API_BASE}/top-traders`)
    if (!res.ok) throw new Error('Failed to get top 5 traders')
    return res.json()
  },

  // Get public trader configuration (no authentication required)
  async getPublicTraderConfig(traderId: string): Promise<any> {
    const res = await fetch(`${API_BASE}/trader/${traderId}/config`)
    if (!res.ok) throw new Error('Failed to get public trader configuration')
    return res.json()
  },

  // Get AI learning performance analysis (supports trader_id)
  async getPerformance(traderId?: string): Promise<any> {
    const url = traderId
      ? `${API_BASE}/performance?trader_id=${traderId}`
      : `${API_BASE}/performance`
    const res = await fetch(url, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get AI learning data')
    return res.json()
  },

  // Get competition data (no authentication required)
  async getCompetition(): Promise<CompetitionData> {
    const res = await fetch(`${API_BASE}/competition`)
    if (!res.ok) throw new Error('Failed to get competition data')
    return res.json()
  },

  // User signal source configuration API
  async getUserSignalSource(): Promise<{
    coin_pool_url: string
    oi_top_url: string
  }> {
    const res = await fetch(`${API_BASE}/user/signal-sources`, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get user signal source configuration')
    return res.json()
  },

  async saveUserSignalSource(
    coinPoolUrl: string,
    oiTopUrl: string
  ): Promise<void> {
    const res = await fetch(`${API_BASE}/user/signal-sources`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({
        coin_pool_url: coinPoolUrl,
        oi_top_url: oiTopUrl,
      }),
    })
    if (!res.ok) throw new Error('Failed to save user signal source configuration')
  },

  // Get server IP (requires authentication, for whitelist configuration)
  async getServerIP(): Promise<{
    public_ip: string
    message: string
  }> {
    const res = await fetch(`${API_BASE}/server-ip`, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get server IP')
    return res.json()
  },
}
```

**Key Features:**
- Type-safe API calls
- Automatic JWT token injection
- Error handling
- Support for optional `trader_id` parameter

### State Management

**React Context:**
- `AuthContext` - Authentication state, user info
- `LanguageContext` - Internationalization

**Zustand Stores:**
- Lightweight state management for UI state
- Used for trader list, competition data caching

### Data Fetching

**SWR (Stale-While-Revalidate):**
- Automatic caching and revalidation
- 5-10 second polling intervals for real-time data
- Optimistic updates

---

## Data Flow Diagrams

### Request Flow: Frontend → Backend

```
┌─────────────┐
│   Browser   │
│  (React UI) │
└──────┬──────┘
       │ HTTP Request (JSON)
       │ Authorization: Bearer <JWT>
       ↓
┌──────────────────┐
│   API Server     │
│  (Gin Router)    │
│  - CORS          │
│  - Auth Middleware│
└──────┬───────────┘
       │
       ├─→ GET /api/traders
       │   └─→ TraderManager.GetAllTraders()
       │       └─→ Database.GetTraders()
       │
       ├─→ POST /api/traders/:id/start
       │   └─→ TraderManager.StartTrader()
       │       └─→ AutoTrader.Start()
       │
       └─→ GET /api/status?trader_id=xxx
           └─→ AutoTrader.GetStatus()
               └─→ Trader.GetBalance()
               └─→ Trader.GetPositions()
```

### Trading Cycle Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    AutoTrader.Run()                         │
│                  (Every 3-5 minutes)                        │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ↓
        ┌───────────────────────────────┐
        │  1. Fetch Account Status       │
        │     - Trader.GetBalance()      │
        │     - Trader.GetPositions()   │
        └───────────────┬───────────────┘
                        │
                        ↓
        ┌───────────────────────────────┐
        │  2. Get Candidate Coins        │
        │     - pool.GetCoinPool()       │
        │     - Filter by liquidity     │
        └───────────────┬───────────────┘
                        │
                        ↓
        ┌───────────────────────────────┐
        │  3. Fetch Market Data           │
        │     - market.FetchKlines()      │
        │     - Calculate indicators     │
        │       (EMA, MACD, RSI, ATR)    │
        └───────────────┬───────────────┘
                        │
                        ↓
        ┌───────────────────────────────┐
        │  4. Build Decision Context      │
        │     - Account info             │
        │     - Positions                │
        │     - Market data              │
        │     - Historical performance   │
        └───────────────┬───────────────┘
                        │
                        ↓
        ┌───────────────────────────────┐
        │  5. Call AI Decision Engine    │
        │     - decision.GetFullDecision()│
        │     - Build prompts            │
        │     - mcp.Client.CallWithMessages()│
        │     - Parse JSON response      │
        └───────────────┬───────────────┘
                        │
                        ↓
        ┌───────────────────────────────┐
        │  6. Validate & Execute         │
        │     - Risk checks              │
        │     - Position limits          │
        │     - Execute orders           │
        │       (close existing first)    │
        └───────────────┬───────────────┘
                        │
                        ↓
        ┌───────────────────────────────┐
        │  7. Log Decision               │
        │     - Save to JSON file        │
        │     - Update performance DB    │
        │     - Calculate metrics        │
        └───────────────────────────────┘
```

### Configuration Flow

```
┌─────────────┐
│ config.json  │ (Optional, synced at startup)
└──────┬───────┘
       │
       ↓ syncConfigToDatabase()
┌─────────────┐
│ config.db   │ (SQLite - source of truth)
│ - system_config│
│ - traders    │
│ - ai_models  │
│ - exchanges  │
└──────┬───────┘
       │
       ├─→ API Endpoints
       │   GET/PUT /api/models
       │   GET/PUT /api/exchanges
       │   GET/PUT /api/traders
       │
       └─→ Frontend
           - Web UI configuration
           - Real-time updates
```

---

## Key Design Patterns

### 1. Interface-Based Abstraction

The trader system uses a unified `Trader` interface to support multiple exchanges:

```3:53:trader/interface.go
// Trader is a unified trading interface
// Supports multiple trading platforms (Binance, Hyperliquid, etc.)
type Trader interface {
	// GetBalance gets the account balance
	GetBalance() (map[string]interface{}, error)

	// GetPositions gets all positions
	GetPositions() ([]map[string]interface{}, error)

	// OpenLong opens a long position
	OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error)

	// OpenShort opens a short position
	OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error)

	// CloseLong closes a long position (quantity=0 means close all)
	CloseLong(symbol string, quantity float64) (map[string]interface{}, error)

	// CloseShort closes a short position (quantity=0 means close all)
	CloseShort(symbol string, quantity float64) (map[string]interface{}, error)

	// SetLeverage sets the leverage
	SetLeverage(symbol string, leverage int) error

	// SetMarginMode sets the margin mode (true=cross margin, false=isolated margin)
	SetMarginMode(symbol string, isCrossMargin bool) error

	// GetMarketPrice gets the market price
	GetMarketPrice(symbol string) (float64, error)

	// SetStopLoss sets a stop loss order
	SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error

	// SetTakeProfit sets a take profit order
	SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error

	// CancelStopLossOrders cancels only stop loss orders (fixes BUG: doesn't delete take profit when adjusting stop loss)
	CancelStopLossOrders(symbol string) error

	// CancelTakeProfitOrders cancels only take profit orders (fixes BUG: doesn't delete stop loss when adjusting take profit)
	CancelTakeProfitOrders(symbol string) error

	// CancelAllOrders cancels all pending orders for this symbol
	CancelAllOrders(symbol string) error

	// CancelStopOrders cancels stop loss/take profit orders for this symbol (used when adjusting stop loss/take profit levels)
	CancelStopOrders(symbol string) error

	// FormatQuantity formats the quantity to the correct precision
	FormatQuantity(symbol string, quantity float64) (string, error)
}
```

**Benefits:**
- Easy to add new exchanges
- Consistent API across platforms
- Testable with mock implementations

### 2. Strategy Pattern

Exchange implementations (`binance_futures.go`, `hyperliquid_trader.go`, `aster_trader.go`) are concrete strategies implementing the `Trader` interface.

**Example:**
- `BinanceFutures` implements `Trader` for Binance
- `HyperliquidTrader` implements `Trader` for Hyperliquid
- `AsterTrader` implements `Trader` for Aster DEX

### 3. Manager Pattern

`TraderManager` orchestrates multiple trader instances:

- Centralized lifecycle management
- Resource coordination
- Competition data aggregation

### 4. Repository Pattern

The `config.Database` acts as a repository for all data access:

- Encapsulates SQL queries
- Provides type-safe methods
- Handles migrations and schema

---

## Technology Stack Details

### Backend Dependencies

| Package | Purpose | Version |
|---------|---------|---------|
| `github.com/gin-gonic/gin` | HTTP web framework | v1.11+ |
| `github.com/adshao/go-binance/v2` | Binance API client | v2.8+ |
| `github.com/ethereum/go-ethereum` | Ethereum client (for Hyperliquid/Aster) | v1.16+ |
| `github.com/sonirico/go-hyperliquid` | Hyperliquid DEX client | v0.17+ |
| `github.com/golang-jwt/jwt/v5` | JWT authentication | v5.2+ |
| `github.com/pquerna/otp` | 2FA/TOTP support | v1.4+ |
| `golang.org/x/crypto` | Password hashing (bcrypt) | v0.42+ |
| `modernc.org/sqlite` | SQLite driver (pure Go) | v1.40+ |
| `github.com/gorilla/websocket` | WebSocket client | v1.5+ |
| `github.com/sirupsen/logrus` | Structured logging | v1.9+ |

### Frontend Dependencies

| Package | Purpose | Version |
|---------|---------|---------|
| `react` + `react-dom` | UI framework | 18.3+ |
| `typescript` | Type safety | 5.8+ |
| `vite` | Build tool & dev server | 6.0+ |
| `tailwindcss` | CSS framework | 3.4+ |
| `recharts` | Chart library | 2.15+ |
| `swr` | Data fetching & caching | 2.2+ |
| `zustand` | State management | 5.0+ |
| `lucide-react` | Icon library | Latest |
| `framer-motion` | Animation library | 12.23+ |

### Build Tools

**Backend:**
- Go 1.25+ compiler
- `go mod` for dependency management

**Frontend:**
- Vite 6 for bundling and dev server
- TypeScript compiler
- ESLint + Prettier for code quality
- PostCSS + Autoprefixer for CSS processing

---

## File Organization Patterns

### Naming Conventions

**Go (Backend):**
- Files: `snake_case.go` (e.g., `auto_trader.go`)
- Types: `PascalCase` (e.g., `AutoTrader`, `TraderManager`)
- Functions: `PascalCase` (exported) or `camelCase` (private)
- Interfaces: `PascalCase` (e.g., `Trader`, `Decision`)

**TypeScript (Frontend):**
- Files: `PascalCase.tsx` for components, `camelCase.ts` for utilities
- Components: `PascalCase` (e.g., `EquityChart.tsx`)
- Functions: `camelCase`
- Types/Interfaces: `PascalCase`

### Code Organization Principles

1. **Separation of Concerns:**
   - `trader/` - Trading logic
   - `decision/` - AI decision making
   - `market/` - Market data
   - `api/` - HTTP API

2. **Dependency Direction:**
   - Higher-level modules depend on lower-level modules
   - Interfaces define contracts between modules
   - No circular dependencies

3. **Module Boundaries:**
   - Each package has a clear responsibility
   - Minimal coupling between packages
   - Shared types in appropriate packages

### Directory Structure Patterns

**Backend:**
- One package per directory
- Related files grouped together
- Interface definitions separate from implementations

**Frontend:**
- Feature-based component organization
- Shared utilities in `lib/`
- Type definitions in `types/`
- Context providers in `contexts/`

---

## Related Documentation

- [Architecture Overview](README.md) - High-level architecture documentation
- [Getting Started](../getting-started/README.md) - Setup and deployment guide
- [Contributing](../../CONTRIBUTING.md) - Contribution guidelines

---

**Last Updated:** 2025-01-27

