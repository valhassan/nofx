package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"nofx/config"
	"nofx/trader"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CompetitionCache caches competition data
type CompetitionCache struct {
	data      map[string]interface{}
	timestamp time.Time
	mu        sync.RWMutex
}

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

// LoadTradersFromDatabase loads all traders from database to memory
func (tm *TraderManager) LoadTradersFromDatabase(database *config.Database) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Get all users
	userIDs, err := database.GetAllUsers()
	if err != nil {
		return fmt.Errorf("failed to get user list: %w", err)
	}

	log.Printf("📋 Found %d users, starting to load all trader configs...", len(userIDs))

	var allTraders []*config.TraderRecord
	for _, userID := range userIDs {
		// Get traders for each user
		traders, err := database.GetTraders(userID)
		if err != nil {
			log.Printf("⚠️ Failed to get traders for user %s: %v", userID, err)
			continue
		}
		log.Printf("📋 User %s: %d traders", userID, len(traders))
		allTraders = append(allTraders, traders...)
	}

	log.Printf("📋 Total loaded %d trader configs", len(allTraders))

	// Get system config (excluding signal sources, signal sources are now user-level)
	maxDailyLossStr, _ := database.GetSystemConfig("max_daily_loss")
	maxDrawdownStr, _ := database.GetSystemConfig("max_drawdown")
	stopTradingMinutesStr, _ := database.GetSystemConfig("stop_trading_minutes")
	defaultCoinsStr, _ := database.GetSystemConfig("default_coins")

	// Parse config
	maxDailyLoss := 10.0 // default value
	if val, err := strconv.ParseFloat(maxDailyLossStr, 64); err == nil {
		maxDailyLoss = val
	}

	maxDrawdown := 20.0 // default value
	if val, err := strconv.ParseFloat(maxDrawdownStr, 64); err == nil {
		maxDrawdown = val
	}

	stopTradingMinutes := 60 // default value
	if val, err := strconv.Atoi(stopTradingMinutesStr); err == nil {
		stopTradingMinutes = val
	}

	// Parse default coin list
	var defaultCoins []string
	if defaultCoinsStr != "" {
		if err := json.Unmarshal([]byte(defaultCoinsStr), &defaultCoins); err != nil {
			log.Printf("⚠️ Failed to parse default coins config: %v, using empty list", err)
			defaultCoins = []string{}
		}
	}

	// Get AI model and exchange config for each trader
	for _, traderCfg := range allTraders {
		// Get AI model config (using the trader's user ID)
		aiModels, err := database.GetAIModels(traderCfg.UserID)
		if err != nil {
			log.Printf("⚠️ Failed to get AI model config: %v", err)
			continue
		}

		var aiModelCfg *config.AIModelConfig
		// Prioritize exact match by model.ID (new logic)
		for _, model := range aiModels {
			if model.ID == traderCfg.AIModelID {
				aiModelCfg = model
				break
			}
		}
		// If no exact match, try matching by provider (compatibility with old data)
		if aiModelCfg == nil {
			for _, model := range aiModels {
				if model.Provider == traderCfg.AIModelID {
					aiModelCfg = model
					log.Printf("⚠️ Trader %s using legacy provider match: %s -> %s", traderCfg.Name, traderCfg.AIModelID, model.ID)
					break
				}
			}
		}

		if aiModelCfg == nil {
			log.Printf("⚠️ Trader %s's AI model %s does not exist, skipping", traderCfg.Name, traderCfg.AIModelID)
			continue
		}

		if !aiModelCfg.Enabled {
			log.Printf("⚠️ Trader %s's AI model %s is not enabled, skipping", traderCfg.Name, traderCfg.AIModelID)
			continue
		}

		// Get exchange config (using the trader's user ID)
		exchanges, err := database.GetExchanges(traderCfg.UserID)
		if err != nil {
			log.Printf("⚠️ Failed to get exchange config: %v", err)
			continue
		}

		var exchangeCfg *config.ExchangeConfig
		for _, exchange := range exchanges {
			if exchange.ID == traderCfg.ExchangeID {
				exchangeCfg = exchange
				break
			}
		}

		if exchangeCfg == nil {
			log.Printf("⚠️ Trader %s's exchange %s does not exist, skipping", traderCfg.Name, traderCfg.ExchangeID)
			continue
		}

		if !exchangeCfg.Enabled {
			log.Printf("⚠️ Trader %s's exchange %s is not enabled, skipping", traderCfg.Name, traderCfg.ExchangeID)
			continue
		}

		// Get user signal source config
		var coinPoolURL, oiTopURL string
		if userSignalSource, err := database.GetUserSignalSource(traderCfg.UserID); err == nil {
			coinPoolURL = userSignalSource.CoinPoolURL
			oiTopURL = userSignalSource.OITopURL
		} else {
			// If user has no signal source configured, use empty string
			log.Printf("🔍 User %s has no signal source configured", traderCfg.UserID)
		}

		// Add to TraderManager
		err = tm.addTraderFromDB(traderCfg, aiModelCfg, exchangeCfg, coinPoolURL, oiTopURL, maxDailyLoss, maxDrawdown, stopTradingMinutes, defaultCoins, database, traderCfg.UserID)
		if err != nil {
			log.Printf("❌ Failed to add trader %s: %v", traderCfg.Name, err)
			continue
		}
	}

	log.Printf("✓ Successfully loaded %d traders to memory", len(tm.traders))
	return nil
}

// addTraderFromDB internal method: add trader from config (no lock, caller already locked)
func (tm *TraderManager) addTraderFromDB(traderCfg *config.TraderRecord, aiModelCfg *config.AIModelConfig, exchangeCfg *config.ExchangeConfig, coinPoolURL, oiTopURL string, maxDailyLoss, maxDrawdown float64, stopTradingMinutes int, defaultCoins []string, database *config.Database, userID string) error {
	if _, exists := tm.traders[traderCfg.ID]; exists {
		return fmt.Errorf("trader ID '%s' already exists", traderCfg.ID)
	}

	// Process trading coin list
	var tradingCoins []string
	if traderCfg.TradingSymbols != "" {
		// Parse comma-separated trading coin list
		symbols := strings.Split(traderCfg.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" {
				tradingCoins = append(tradingCoins, symbol)
			}
		}
	}

	// If no trading coins specified, use default coins
	if len(tradingCoins) == 0 {
		tradingCoins = defaultCoins
	}

	// Determine whether to use signal source based on trader config
	var effectiveCoinPoolURL string
	if traderCfg.UseCoinPool && coinPoolURL != "" {
		effectiveCoinPoolURL = coinPoolURL
		log.Printf("✓ Trader %s enabled COIN POOL signal source: %s", traderCfg.Name, coinPoolURL)
	}

	// Build AutoTraderConfig
	traderConfig := trader.AutoTraderConfig{
		ID:                    traderCfg.ID,
		Name:                  traderCfg.Name,
		AIModel:               aiModelCfg.Provider, // Use provider as model identifier
		Exchange:              exchangeCfg.ID,      // Use exchange ID
		BinanceAPIKey:         "",
		BinanceSecretKey:      "",
		HyperliquidPrivateKey: "",
		HyperliquidTestnet:    exchangeCfg.Testnet,
		CoinPoolAPIURL:        effectiveCoinPoolURL,
		UseQwen:               aiModelCfg.Provider == "qwen",
		DeepSeekKey:           "",
		QwenKey:               "",
		CustomAPIURL:          aiModelCfg.CustomAPIURL,    // Custom API URL
		CustomModelName:       aiModelCfg.CustomModelName, // Custom model name
		ScanInterval:          time.Duration(traderCfg.ScanIntervalMinutes) * time.Minute,
		InitialBalance:        traderCfg.InitialBalance,
		BTCETHLeverage:        traderCfg.BTCETHLeverage,
		AltcoinLeverage:       traderCfg.AltcoinLeverage,
		MaxDailyLoss:          maxDailyLoss,
		MaxDrawdown:           maxDrawdown,
		StopTradingTime:       time.Duration(stopTradingMinutes) * time.Minute,
		IsCrossMargin:         traderCfg.IsCrossMargin,
		DefaultCoins:          defaultCoins,
		TradingCoins:          tradingCoins,
		SystemPromptTemplate:  traderCfg.SystemPromptTemplate, // System prompt template
	}

	// Set API keys based on exchange type
	if exchangeCfg.ID == "binance" {
		traderConfig.BinanceAPIKey = exchangeCfg.APIKey
		traderConfig.BinanceSecretKey = exchangeCfg.SecretKey
	} else if exchangeCfg.ID == "hyperliquid" {
		traderConfig.HyperliquidPrivateKey = exchangeCfg.APIKey // hyperliquid uses APIKey to store private key
		traderConfig.HyperliquidWalletAddr = exchangeCfg.HyperliquidWalletAddr
	} else if exchangeCfg.ID == "aster" {
		traderConfig.AsterUser = exchangeCfg.AsterUser
		traderConfig.AsterSigner = exchangeCfg.AsterSigner
		traderConfig.AsterPrivateKey = exchangeCfg.AsterPrivateKey
	}

	// Set API keys based on AI model
	if aiModelCfg.Provider == "qwen" {
		traderConfig.QwenKey = aiModelCfg.APIKey
	} else if aiModelCfg.Provider == "deepseek" {
		traderConfig.DeepSeekKey = aiModelCfg.APIKey
	}

	// Create trader instance
	at, err := trader.NewAutoTrader(traderConfig, database, userID)
	if err != nil {
		return fmt.Errorf("failed to create trader: %w", err)
	}

	// Set custom prompt (if any)
	if traderCfg.CustomPrompt != "" {
		at.SetCustomPrompt(traderCfg.CustomPrompt)
		at.SetOverrideBasePrompt(traderCfg.OverrideBasePrompt)
		if traderCfg.OverrideBasePrompt {
			log.Printf("✓ Custom trading strategy prompt set (overriding base prompt)")
		} else {
			log.Printf("✓ Custom trading strategy prompt set (supplementing base prompt)")
		}
	}

	tm.traders[traderCfg.ID] = at
	log.Printf("✓ Trader '%s' (%s + %s) loaded to memory", traderCfg.Name, aiModelCfg.Provider, exchangeCfg.ID)
	return nil
}

// AddTraderFromDB adds trader from database config
func (tm *TraderManager) AddTraderFromDB(traderCfg *config.TraderRecord, aiModelCfg *config.AIModelConfig, exchangeCfg *config.ExchangeConfig, coinPoolURL, oiTopURL string, maxDailyLoss, maxDrawdown float64, stopTradingMinutes int, defaultCoins []string, database *config.Database, userID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.traders[traderCfg.ID]; exists {
		return fmt.Errorf("trader ID '%s' already exists", traderCfg.ID)
	}

	// Process trading coin list
	var tradingCoins []string
	if traderCfg.TradingSymbols != "" {
		// Parse comma-separated trading coin list
		symbols := strings.Split(traderCfg.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" {
				tradingCoins = append(tradingCoins, symbol)
			}
		}
	}

	// If no trading coins specified, use default coins
	if len(tradingCoins) == 0 {
		tradingCoins = defaultCoins
	}

	// Determine whether to use signal source based on trader config
	var effectiveCoinPoolURL string
	if traderCfg.UseCoinPool && coinPoolURL != "" {
		effectiveCoinPoolURL = coinPoolURL
		log.Printf("✓ Trader %s enabled COIN POOL signal source: %s", traderCfg.Name, coinPoolURL)
	}

	// Build AutoTraderConfig
	traderConfig := trader.AutoTraderConfig{
		ID:                    traderCfg.ID,
		Name:                  traderCfg.Name,
		AIModel:               aiModelCfg.Provider, // Use provider as model identifier
		Exchange:              exchangeCfg.ID,      // Use exchange ID
		BinanceAPIKey:         "",
		BinanceSecretKey:      "",
		HyperliquidPrivateKey: "",
		HyperliquidTestnet:    exchangeCfg.Testnet,
		CoinPoolAPIURL:        effectiveCoinPoolURL,
		UseQwen:               aiModelCfg.Provider == "qwen",
		DeepSeekKey:           "",
		QwenKey:               "",
		CustomAPIURL:          aiModelCfg.CustomAPIURL,    // Custom API URL
		CustomModelName:       aiModelCfg.CustomModelName, // Custom model name
		ScanInterval:          time.Duration(traderCfg.ScanIntervalMinutes) * time.Minute,
		InitialBalance:        traderCfg.InitialBalance,
		BTCETHLeverage:        traderCfg.BTCETHLeverage,
		AltcoinLeverage:       traderCfg.AltcoinLeverage,
		MaxDailyLoss:          maxDailyLoss,
		MaxDrawdown:           maxDrawdown,
		StopTradingTime:       time.Duration(stopTradingMinutes) * time.Minute,
		IsCrossMargin:         traderCfg.IsCrossMargin,
		DefaultCoins:          defaultCoins,
		TradingCoins:          tradingCoins,
	}

	// Set API keys based on exchange type
	if exchangeCfg.ID == "binance" {
		traderConfig.BinanceAPIKey = exchangeCfg.APIKey
		traderConfig.BinanceSecretKey = exchangeCfg.SecretKey
	} else if exchangeCfg.ID == "hyperliquid" {
		traderConfig.HyperliquidPrivateKey = exchangeCfg.APIKey // hyperliquid uses APIKey to store private key
		traderConfig.HyperliquidWalletAddr = exchangeCfg.HyperliquidWalletAddr
	} else if exchangeCfg.ID == "aster" {
		traderConfig.AsterUser = exchangeCfg.AsterUser
		traderConfig.AsterSigner = exchangeCfg.AsterSigner
		traderConfig.AsterPrivateKey = exchangeCfg.AsterPrivateKey
	}

	// Set API keys based on AI model
	if aiModelCfg.Provider == "qwen" {
		traderConfig.QwenKey = aiModelCfg.APIKey
	} else if aiModelCfg.Provider == "deepseek" {
		traderConfig.DeepSeekKey = aiModelCfg.APIKey
	}

	// Create trader instance
	at, err := trader.NewAutoTrader(traderConfig, database, userID)
	if err != nil {
		return fmt.Errorf("failed to create trader: %w", err)
	}

	// Set custom prompt (if any)
	if traderCfg.CustomPrompt != "" {
		at.SetCustomPrompt(traderCfg.CustomPrompt)
		at.SetOverrideBasePrompt(traderCfg.OverrideBasePrompt)
		if traderCfg.OverrideBasePrompt {
			log.Printf("✓ Custom trading strategy prompt set (overriding base prompt)")
		} else {
			log.Printf("✓ Custom trading strategy prompt set (supplementing base prompt)")
		}
	}

	tm.traders[traderCfg.ID] = at
	log.Printf("✓ Trader '%s' (%s + %s) added", traderCfg.Name, aiModelCfg.Provider, exchangeCfg.ID)
	return nil
}

// GetTrader gets trader by ID
func (tm *TraderManager) GetTrader(id string) (*trader.AutoTrader, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	t, exists := tm.traders[id]
	if !exists {
		return nil, fmt.Errorf("trader ID '%s' does not exist", id)
	}
	return t, nil
}

// GetAllTraders gets all traders
func (tm *TraderManager) GetAllTraders() map[string]*trader.AutoTrader {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make(map[string]*trader.AutoTrader)
	for id, t := range tm.traders {
		result[id] = t
	}
	return result
}

// GetTraderIDs gets all trader ID list
func (tm *TraderManager) GetTraderIDs() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	ids := make([]string, 0, len(tm.traders))
	for id := range tm.traders {
		ids = append(ids, id)
	}
	return ids
}

// StartAll starts all traders
func (tm *TraderManager) StartAll() {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	log.Println("🚀 Starting all Traders...")
	for id, t := range tm.traders {
		go func(traderID string, at *trader.AutoTrader) {
			log.Printf("▶️  Starting %s...", at.GetName())
			if err := at.Run(); err != nil {
				log.Printf("❌ %s runtime error: %v", at.GetName(), err)
			}
		}(id, t)
	}
}

// StopAll stops all traders
func (tm *TraderManager) StopAll() {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	log.Println("⏹  Stopping all Traders...")
	for _, t := range tm.traders {
		t.Stop()
	}
}

// GetComparisonData gets comparison data
func (tm *TraderManager) GetComparisonData() (map[string]interface{}, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	comparison := make(map[string]interface{})
	traders := make([]map[string]interface{}, 0, len(tm.traders))

	for _, t := range tm.traders {
		account, err := t.GetAccountInfo()
		if err != nil {
			continue
		}

		status := t.GetStatus()

		traders = append(traders, map[string]interface{}{
			"trader_id":       t.GetID(),
			"trader_name":     t.GetName(),
			"ai_model":        t.GetAIModel(),
			"exchange":        t.GetExchange(),
			"total_equity":    account["total_equity"],
			"total_pnl":       account["total_pnl"],
			"total_pnl_pct":   account["total_pnl_pct"],
			"position_count":  account["position_count"],
			"margin_used_pct": account["margin_used_pct"],
			"call_count":      status["call_count"],
			"is_running":      status["is_running"],
		})
	}

	comparison["traders"] = traders
	comparison["count"] = len(traders)

	return comparison, nil
}

// GetCompetitionData gets competition data (all traders across the platform)
func (tm *TraderManager) GetCompetitionData() (map[string]interface{}, error) {
	// Check if cache is valid (within 30 seconds)
	tm.competitionCache.mu.RLock()
	if time.Since(tm.competitionCache.timestamp) < 30*time.Second && len(tm.competitionCache.data) > 0 {
		// Return cached data
		cachedData := make(map[string]interface{})
		for k, v := range tm.competitionCache.data {
			cachedData[k] = v
		}
		tm.competitionCache.mu.RUnlock()
		log.Printf("📋 Returning competition data cache (cache age: %.1fs)", time.Since(tm.competitionCache.timestamp).Seconds())
		return cachedData, nil
	}
	tm.competitionCache.mu.RUnlock()

	tm.mu.RLock()

	// Get all trader list
	allTraders := make([]*trader.AutoTrader, 0, len(tm.traders))
	for _, t := range tm.traders {
		allTraders = append(allTraders, t)
	}
	tm.mu.RUnlock()

	log.Printf("🔄 Refreshing competition data, trader count: %d", len(allTraders))

	// Concurrently get trader data
	traders := tm.getConcurrentTraderData(allTraders)

	// Sort by return rate (descending)
	sort.Slice(traders, func(i, j int) bool {
		pnlPctI, okI := traders[i]["total_pnl_pct"].(float64)
		pnlPctJ, okJ := traders[j]["total_pnl_pct"].(float64)
		if !okI {
			pnlPctI = 0
		}
		if !okJ {
			pnlPctJ = 0
		}
		return pnlPctI > pnlPctJ
	})

	// Limit to top 50
	totalCount := len(traders)
	limit := 50
	if len(traders) > limit {
		traders = traders[:limit]
	}

	comparison := make(map[string]interface{})
	comparison["traders"] = traders
	comparison["count"] = len(traders)
	comparison["total_count"] = totalCount // Total trader count

	// Update cache
	tm.competitionCache.mu.Lock()
	tm.competitionCache.data = comparison
	tm.competitionCache.timestamp = time.Now()
	tm.competitionCache.mu.Unlock()

	return comparison, nil
}

// getConcurrentTraderData concurrently gets data for multiple traders
func (tm *TraderManager) getConcurrentTraderData(traders []*trader.AutoTrader) []map[string]interface{} {
	type traderResult struct {
		index int
		data  map[string]interface{}
	}

	// Create result channel
	resultChan := make(chan traderResult, len(traders))

	// Concurrently get data for each trader
	for i, t := range traders {
		go func(index int, trader *trader.AutoTrader) {
			// Set timeout for single trader to 3 seconds
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			// Use channels to implement timeout control
			accountChan := make(chan map[string]interface{}, 1)
			errorChan := make(chan error, 1)

			go func() {
				account, err := trader.GetAccountInfo()
				if err != nil {
					errorChan <- err
				} else {
					accountChan <- account
				}
			}()

			status := trader.GetStatus()
			var traderData map[string]interface{}

			select {
			case account := <-accountChan:
				// Successfully got account info
				traderData = map[string]interface{}{
					"trader_id":       trader.GetID(),
					"trader_name":     trader.GetName(),
					"ai_model":        trader.GetAIModel(),
					"exchange":        trader.GetExchange(),
					"total_equity":    account["total_equity"],
					"total_pnl":       account["total_pnl"],
					"total_pnl_pct":   account["total_pnl_pct"],
					"position_count":  account["position_count"],
					"margin_used_pct": account["margin_used_pct"],
					"is_running":      status["is_running"],
				}
			case err := <-errorChan:
				// Failed to get account info
				log.Printf("⚠️ Failed to get account info for trader %s: %v", trader.GetID(), err)
				traderData = map[string]interface{}{
					"trader_id":       trader.GetID(),
					"trader_name":     trader.GetName(),
					"ai_model":        trader.GetAIModel(),
					"exchange":        trader.GetExchange(),
					"total_equity":    0.0,
					"total_pnl":       0.0,
					"total_pnl_pct":   0.0,
					"position_count":  0,
					"margin_used_pct": 0.0,
					"is_running":      status["is_running"],
					"error":           "Failed to get account data",
				}
			case <-ctx.Done():
				// Timeout
				log.Printf("⏰ Timeout getting account info for trader %s", trader.GetID())
				traderData = map[string]interface{}{
					"trader_id":       trader.GetID(),
					"trader_name":     trader.GetName(),
					"ai_model":        trader.GetAIModel(),
					"exchange":        trader.GetExchange(),
					"total_equity":    0.0,
					"total_pnl":       0.0,
					"total_pnl_pct":   0.0,
					"position_count":  0,
					"margin_used_pct": 0.0,
					"is_running":      status["is_running"],
					"error":           "Request timeout",
				}
			}

			resultChan <- traderResult{index: index, data: traderData}
		}(i, t)
	}

	// Collect all results
	results := make([]map[string]interface{}, len(traders))
	for i := 0; i < len(traders); i++ {
		result := <-resultChan
		results[result.index] = result.data
	}

	return results
}

// GetTopTradersData gets top 5 trader data (for performance comparison)
func (tm *TraderManager) GetTopTradersData() (map[string]interface{}, error) {
	// Reuse competition data cache, since top 5 is filtered from all data
	competitionData, err := tm.GetCompetitionData()
	if err != nil {
		return nil, err
	}

	// Extract top 5 from competition data
	allTraders, ok := competitionData["traders"].([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("competition data format error")
	}

	// Limit to top 5
	limit := 5
	topTraders := allTraders
	if len(allTraders) > limit {
		topTraders = allTraders[:limit]
	}

	result := map[string]interface{}{
		"traders": topTraders,
		"count":   len(topTraders),
	}

	return result, nil
}

// isUserTrader checks if trader belongs to specified user
func isUserTrader(traderID, userID string) bool {
	// trader ID format: userID_traderName or randomUUID_modelName
	// For compatibility, we check the prefix
	if len(traderID) >= len(userID) && traderID[:len(userID)] == userID {
		return true
	}
	// For legacy default user, all without explicit user prefix belong to default
	if userID == "default" && !containsUserPrefix(traderID) {
		return true
	}
	return false
}

// containsUserPrefix checks if trader ID contains user prefix
func containsUserPrefix(traderID string) bool {
	// Check if contains email format prefix (user@example.com_traderName)
	for i, ch := range traderID {
		if ch == '@' {
			// Found @ symbol, likely email prefix
			return true
		}
		if ch == '_' && i > 0 {
			// Found underscore but no @ before, might be UUID or other format
			break
		}
	}
	return false
}

// LoadUserTraders loads traders for a specific user to memory
func (tm *TraderManager) LoadUserTraders(database *config.Database, userID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Get all traders for specified user
	traders, err := database.GetTraders(userID)
	if err != nil {
		return fmt.Errorf("failed to get trader list for user %s: %w", userID, err)
	}

	log.Printf("📋 Loading trader configs for user %s: %d traders", userID, len(traders))

	// Get system config (excluding signal sources, signal sources are now user-level)
	maxDailyLossStr, _ := database.GetSystemConfig("max_daily_loss")
	maxDrawdownStr, _ := database.GetSystemConfig("max_drawdown")
	stopTradingMinutesStr, _ := database.GetSystemConfig("stop_trading_minutes")
	defaultCoinsStr, _ := database.GetSystemConfig("default_coins")

	// Get user signal source config
	var coinPoolURL, oiTopURL string
	if userSignalSource, err := database.GetUserSignalSource(userID); err == nil {
		coinPoolURL = userSignalSource.CoinPoolURL
		oiTopURL = userSignalSource.OITopURL
		log.Printf("📡 Loading signal source config for user %s: COIN POOL=%s, OI TOP=%s", userID, coinPoolURL, oiTopURL)
	} else {
		log.Printf("🔍 User %s has no signal source configured", userID)
	}

	// Parse config
	maxDailyLoss := 10.0 // default value
	if val, err := strconv.ParseFloat(maxDailyLossStr, 64); err == nil {
		maxDailyLoss = val
	}

	maxDrawdown := 20.0 // default value
	if val, err := strconv.ParseFloat(maxDrawdownStr, 64); err == nil {
		maxDrawdown = val
	}

	stopTradingMinutes := 60 // default value
	if val, err := strconv.Atoi(stopTradingMinutesStr); err == nil {
		stopTradingMinutes = val
	}

	// Parse default coin list
	var defaultCoins []string
	if defaultCoinsStr != "" {
		if err := json.Unmarshal([]byte(defaultCoinsStr), &defaultCoins); err != nil {
			log.Printf("⚠️ Failed to parse default coins config: %v, using empty list", err)
			defaultCoins = []string{}
		}
	}

	// Get AI model and exchange config for each trader
	for _, traderCfg := range traders {
		// Check if trader is already loaded
		if _, exists := tm.traders[traderCfg.ID]; exists {
			log.Printf("⚠️ Trader %s already loaded, skipping", traderCfg.Name)
			continue
		}

		// Get AI model config (using this user's config)
		aiModels, err := database.GetAIModels(userID)
		if err != nil {
			log.Printf("⚠️ Failed to get AI model config for user %s: %v", userID, err)
			continue
		}

		var aiModelCfg *config.AIModelConfig
		// Prioritize exact match by model.ID (new logic)
		for _, model := range aiModels {
			if model.ID == traderCfg.AIModelID {
				aiModelCfg = model
				break
			}
		}
		// If no exact match, try matching by provider (compatibility with old data)
		if aiModelCfg == nil {
			for _, model := range aiModels {
				if model.Provider == traderCfg.AIModelID {
					aiModelCfg = model
					log.Printf("⚠️ Trader %s using legacy provider match: %s -> %s", traderCfg.Name, traderCfg.AIModelID, model.ID)
					break
				}
			}
		}

		if aiModelCfg == nil {
			log.Printf("⚠️ Trader %s's AI model %s does not exist, skipping", traderCfg.Name, traderCfg.AIModelID)
			continue
		}

		if !aiModelCfg.Enabled {
			log.Printf("⚠️ Trader %s's AI model %s is not enabled, skipping", traderCfg.Name, traderCfg.AIModelID)
			continue
		}

		// Get exchange config (using this user's config)
		exchanges, err := database.GetExchanges(userID)
		if err != nil {
			log.Printf("⚠️ Failed to get exchange config for user %s: %v", userID, err)
			continue
		}

		var exchangeCfg *config.ExchangeConfig
		for _, exchange := range exchanges {
			if exchange.ID == traderCfg.ExchangeID {
				exchangeCfg = exchange
				break
			}
		}

		if exchangeCfg == nil {
			log.Printf("⚠️ Trader %s's exchange %s does not exist, skipping", traderCfg.Name, traderCfg.ExchangeID)
			continue
		}

		if !exchangeCfg.Enabled {
			log.Printf("⚠️ Trader %s's exchange %s is not enabled, skipping", traderCfg.Name, traderCfg.ExchangeID)
			continue
		}

		// Use existing method to load trader
		err = tm.loadSingleTrader(traderCfg, aiModelCfg, exchangeCfg, coinPoolURL, oiTopURL, maxDailyLoss, maxDrawdown, stopTradingMinutes, defaultCoins, database, userID)
		if err != nil {
			log.Printf("⚠️ Failed to load trader %s: %v", traderCfg.Name, err)
		}
	}

	return nil
}

// loadSingleTrader loads a single trader (common logic extracted from existing code)
func (tm *TraderManager) loadSingleTrader(traderCfg *config.TraderRecord, aiModelCfg *config.AIModelConfig, exchangeCfg *config.ExchangeConfig, coinPoolURL, oiTopURL string, maxDailyLoss, maxDrawdown float64, stopTradingMinutes int, defaultCoins []string, database *config.Database, userID string) error {
	// Process trading coin list
	var tradingCoins []string
	if traderCfg.TradingSymbols != "" {
		// Parse comma-separated trading coin list
		symbols := strings.Split(traderCfg.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" {
				tradingCoins = append(tradingCoins, symbol)
			}
		}
	}

	// If no trading coins specified, use default coins
	if len(tradingCoins) == 0 {
		tradingCoins = defaultCoins
	}

	// Determine whether to use signal source based on trader config
	var effectiveCoinPoolURL string
	if traderCfg.UseCoinPool && coinPoolURL != "" {
		effectiveCoinPoolURL = coinPoolURL
		log.Printf("✓ Trader %s enabled COIN POOL signal source: %s", traderCfg.Name, coinPoolURL)
	}

	// Build AutoTraderConfig
	traderConfig := trader.AutoTraderConfig{
		ID:                   traderCfg.ID,
		Name:                 traderCfg.Name,
		AIModel:              aiModelCfg.Provider, // Use provider as model identifier
		Exchange:             exchangeCfg.ID,      // Use exchange ID
		InitialBalance:       traderCfg.InitialBalance,
		BTCETHLeverage:       traderCfg.BTCETHLeverage,
		AltcoinLeverage:      traderCfg.AltcoinLeverage,
		ScanInterval:         time.Duration(traderCfg.ScanIntervalMinutes) * time.Minute,
		CoinPoolAPIURL:       effectiveCoinPoolURL,
		CustomAPIURL:         aiModelCfg.CustomAPIURL,    // Custom API URL
		CustomModelName:      aiModelCfg.CustomModelName, // Custom model name
		UseQwen:              aiModelCfg.Provider == "qwen",
		MaxDailyLoss:         maxDailyLoss,
		MaxDrawdown:          maxDrawdown,
		StopTradingTime:      time.Duration(stopTradingMinutes) * time.Minute,
		IsCrossMargin:        traderCfg.IsCrossMargin,
		DefaultCoins:         defaultCoins,
		TradingCoins:         tradingCoins,
		SystemPromptTemplate: traderCfg.SystemPromptTemplate, // System prompt template
		HyperliquidTestnet:   exchangeCfg.Testnet,            // Hyperliquid testnet
	}

	// Set API keys based on exchange type
	if exchangeCfg.ID == "binance" {
		traderConfig.BinanceAPIKey = exchangeCfg.APIKey
		traderConfig.BinanceSecretKey = exchangeCfg.SecretKey
	} else if exchangeCfg.ID == "hyperliquid" {
		traderConfig.HyperliquidPrivateKey = exchangeCfg.APIKey // hyperliquid uses APIKey to store private key
		traderConfig.HyperliquidWalletAddr = exchangeCfg.HyperliquidWalletAddr
	} else if exchangeCfg.ID == "aster" {
		traderConfig.AsterUser = exchangeCfg.AsterUser
		traderConfig.AsterSigner = exchangeCfg.AsterSigner
		traderConfig.AsterPrivateKey = exchangeCfg.AsterPrivateKey
	}

	// Set API keys based on AI model
	if aiModelCfg.Provider == "qwen" {
		traderConfig.QwenKey = aiModelCfg.APIKey
	} else if aiModelCfg.Provider == "deepseek" {
		traderConfig.DeepSeekKey = aiModelCfg.APIKey
	}

	// Create trader instance
	at, err := trader.NewAutoTrader(traderConfig, database, userID)
	if err != nil {
		return fmt.Errorf("failed to create trader: %w", err)
	}

	// Set custom prompt (if any)
	if traderCfg.CustomPrompt != "" {
		at.SetCustomPrompt(traderCfg.CustomPrompt)
		at.SetOverrideBasePrompt(traderCfg.OverrideBasePrompt)
		if traderCfg.OverrideBasePrompt {
			log.Printf("✓ Custom trading strategy prompt set (overriding base prompt)")
		} else {
			log.Printf("✓ Custom trading strategy prompt set (supplementing base prompt)")
		}
	}

	tm.traders[traderCfg.ID] = at
	log.Printf("✓ Trader '%s' (%s + %s) loaded to memory for user", traderCfg.Name, aiModelCfg.Provider, exchangeCfg.ID)
	return nil
}
