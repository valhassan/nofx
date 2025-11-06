package decision

import (
	"encoding/json"
	"fmt"
	"log"
	"nofx/market"
	"nofx/mcp"
	"nofx/pool"
	"regexp"
	"strings"
	"time"
)

// Precompiled regex patterns (performance optimization: avoid recompiling on each call)
var (
	// ✅ Safe regex: precisely matches ```json code blocks
	// Use backticks + concatenation to avoid escaping issues
	reJSONFence      = regexp.MustCompile(`(?is)` + "```json\\s*(\\[\\s*\\{.*?\\}\\s*\\])\\s*```")
	reJSONArray      = regexp.MustCompile(`(?is)\[\s*\{.*?\}\s*\]`)
	reArrayHead      = regexp.MustCompile(`^\[\s*\{`)
	reArrayOpenSpace = regexp.MustCompile(`^\[\s+\{`)
	reInvisibleRunes = regexp.MustCompile("[\u200B\u200C\u200D\uFEFF]")
)

// PositionInfo represents position information
type PositionInfo struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"` // "long" or "short"
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	Quantity         float64 `json:"quantity"`
	Leverage         int     `json:"leverage"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct float64 `json:"unrealized_pnl_pct"`
	LiquidationPrice float64 `json:"liquidation_price"`
	MarginUsed       float64 `json:"margin_used"`
	UpdateTime       int64   `json:"update_time"` // Position update timestamp (milliseconds)
}

// AccountInfo represents account information
type AccountInfo struct {
	TotalEquity      float64 `json:"total_equity"`      // Account equity
	AvailableBalance float64 `json:"available_balance"` // Available balance
	TotalPnL         float64 `json:"total_pnl"`         // Total P&L
	TotalPnLPct      float64 `json:"total_pnl_pct"`     // Total P&L percentage
	MarginUsed       float64 `json:"margin_used"`       // Margin used
	MarginUsedPct    float64 `json:"margin_used_pct"`   // Margin usage rate
	PositionCount    int     `json:"position_count"`    // Number of positions
}

// CandidateCoin represents a candidate coin from the coin pool
type CandidateCoin struct {
	Symbol  string   `json:"symbol"`
	Sources []string `json:"sources"` // Sources: "ai500" and/or "oi_top"
}

// OITopData represents OI (Open Interest) growth top data (for AI decision reference)
type OITopData struct {
	Rank              int     // OI Top rank
	OIDeltaPercent    float64 // OI change percentage (1 hour)
	OIDeltaValue      float64 // OI change value
	PriceDeltaPercent float64 // Price change percentage
	NetLong           float64 // Net long position
	NetShort          float64 // Net short position
}

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

// Decision represents an AI trading decision
type Decision struct {
	Symbol string `json:"symbol"`
	Action string `json:"action"` // "open_long", "open_short", "close_long", "close_short", "update_stop_loss", "update_take_profit", "partial_close", "hold", "wait"

	// Opening position parameters
	Leverage        int     `json:"leverage,omitempty"`
	PositionSizeUSD float64 `json:"position_size_usd,omitempty"`
	StopLoss        float64 `json:"stop_loss,omitempty"`
	TakeProfit      float64 `json:"take_profit,omitempty"`

	// Adjustment parameters (newly added)
	NewStopLoss     float64 `json:"new_stop_loss,omitempty"`    // For update_stop_loss
	NewTakeProfit   float64 `json:"new_take_profit,omitempty"`  // For update_take_profit
	ClosePercentage float64 `json:"close_percentage,omitempty"` // For partial_close (0-100)

	// Common parameters
	Confidence int     `json:"confidence,omitempty"` // Confidence level (0-100)
	RiskUSD    float64 `json:"risk_usd,omitempty"`   // Maximum USD risk
	Reasoning  string  `json:"reasoning"`
}

// FullDecision represents a complete AI decision (including chain of thought)
type FullDecision struct {
	SystemPrompt string     `json:"system_prompt"` // System prompt (sent to AI)
	UserPrompt   string     `json:"user_prompt"`   // User prompt (sent to AI)
	CoTTrace     string     `json:"cot_trace"`     // Chain of thought analysis (AI output)
	Decisions    []Decision `json:"decisions"`     // List of specific decisions
	Timestamp    time.Time  `json:"timestamp"`
}

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

// fetchMarketDataForContext fetches market data and OI data for all coins in the context
func fetchMarketDataForContext(ctx *Context) error {
	ctx.MarketDataMap = make(map[string]*market.Data)
	ctx.OITopDataMap = make(map[string]*OITopData)

	// Collect all coins that need data
	symbolSet := make(map[string]bool)

	// 1. Prioritize fetching data for position coins (required)
	for _, pos := range ctx.Positions {
		symbolSet[pos.Symbol] = true
	}

	// 2. Candidate coin count dynamically adjusted based on account status
	maxCandidates := calculateMaxCandidates(ctx)
	for i, coin := range ctx.CandidateCoins {
		if i >= maxCandidates {
			break
		}
		symbolSet[coin.Symbol] = true
	}

	// Fetch market data concurrently
	// Position symbol set (used to determine whether to skip OI check)
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		positionSymbols[pos.Symbol] = true
	}

	for symbol := range symbolSet {
		data, err := market.Get(symbol)
		if err != nil {
			// Single coin failure doesn't affect overall, only log error
			continue
		}

		// ⚠️ Liquidity filter: skip coins with OI value below threshold (both long and short)
		// OI value = open interest × current price
		// But existing positions must be kept (need to decide whether to close)
		// 💡 OI threshold config: users can adjust based on risk preference
		const minOIThresholdMillions = 15.0 // Adjustable: 15M(conservative) / 10M(balanced) / 8M(loose) / 5M(aggressive)

		isExistingPosition := positionSymbols[symbol]
		if !isExistingPosition && data.OpenInterest != nil && data.CurrentPrice > 0 {
			// Calculate OI value (USD) = open interest × current price
			oiValue := data.OpenInterest.Latest * data.CurrentPrice
			oiValueInMillions := oiValue / 1_000_000 // Convert to millions USD
			if oiValueInMillions < minOIThresholdMillions {
				log.Printf("⚠️  %s OI value too low (%.2fM USD < %.1fM), skipping this coin [OI:%.0f × Price:%.4f]",
					symbol, oiValueInMillions, minOIThresholdMillions, data.OpenInterest.Latest, data.CurrentPrice)
				continue
			}
		}

		ctx.MarketDataMap[symbol] = data
	}

	// Load OI Top data (doesn't affect main flow)
	oiPositions, err := pool.GetOITopPositions()
	if err == nil {
		for _, pos := range oiPositions {
			// Normalize symbol matching
			symbol := pos.Symbol
			ctx.OITopDataMap[symbol] = &OITopData{
				Rank:              pos.Rank,
				OIDeltaPercent:    pos.OIDeltaPercent,
				OIDeltaValue:      pos.OIDeltaValue,
				PriceDeltaPercent: pos.PriceDeltaPercent,
				NetLong:           pos.NetLong,
				NetShort:          pos.NetShort,
			}
		}
	}

	return nil
}

// calculateMaxCandidates calculates the number of candidate coins to analyze based on account status
func calculateMaxCandidates(ctx *Context) int {
	// ⚠️ Important: limit candidate coin count to avoid prompt being too large
	// Dynamically adjust based on position count: fewer positions means more candidate coins can be analyzed
	const (
		maxCandidatesWhenEmpty    = 30 // Max 30 candidates when no positions
		maxCandidatesWhenHolding1 = 25 // Max 25 candidates when holding 1 position
		maxCandidatesWhenHolding2 = 20 // Max 20 candidates when holding 2 positions
		maxCandidatesWhenHolding3 = 15 // Max 15 candidates when holding 3 positions (avoid prompt being too large)
	)

	positionCount := len(ctx.Positions)
	var maxCandidates int

	switch positionCount {
	case 0:
		maxCandidates = maxCandidatesWhenEmpty
	case 1:
		maxCandidates = maxCandidatesWhenHolding1
	case 2:
		maxCandidates = maxCandidatesWhenHolding2
	default: // 3+ positions
		maxCandidates = maxCandidatesWhenHolding3
	}

	// Return the smaller value between actual candidate count and limit
	return min(len(ctx.CandidateCoins), maxCandidates)
}

// buildSystemPromptWithCustom builds System Prompt with custom content
func buildSystemPromptWithCustom(accountEquity float64, btcEthLeverage, altcoinLeverage int, customPrompt string, overrideBase bool, templateName string) string {
	// If override base prompt and custom prompt exists, use only custom prompt
	if overrideBase && customPrompt != "" {
		return customPrompt
	}

	// Get base prompt (using specified template)
	basePrompt := buildSystemPrompt(accountEquity, btcEthLeverage, altcoinLeverage, templateName)

	// If no custom prompt, return base prompt directly
	if customPrompt == "" {
		return basePrompt
	}

	// Add custom prompt section to base prompt
	var sb strings.Builder
	sb.WriteString(basePrompt)
	sb.WriteString("\n\n")
	sb.WriteString("# 📌 Personalized Trading Strategy\n\n")
	sb.WriteString(customPrompt)
	sb.WriteString("\n\n")
	sb.WriteString("Note: The above personalized strategy supplements the base rules and must not violate basic risk control principles.\n")

	return sb.String()
}

// buildSystemPrompt builds System Prompt (using template + dynamic parts)
func buildSystemPrompt(accountEquity float64, btcEthLeverage, altcoinLeverage int, templateName string) string {
	var sb strings.Builder

	// 1. Load prompt template (core trading strategy part)
	if templateName == "" {
		templateName = "default" // Default to "default" template
	}

	template, err := GetPromptTemplate(templateName)
	if err != nil {
		// If template doesn't exist, log error and use default
		log.Printf("⚠️  Prompt template '%s' does not exist, using default: %v", templateName, err)
		template, err = GetPromptTemplate("default")
		if err != nil {
			// If even default doesn't exist, use built-in simplified version
			log.Printf("❌ Unable to load any prompt templates, using built-in simplified version")
			sb.WriteString("You are a professional cryptocurrency trading AI. Please make trading decisions based on market data.\n\n")
		} else {
			sb.WriteString(template.Content)
			sb.WriteString("\n\n")
		}
	} else {
		sb.WriteString(template.Content)
		sb.WriteString("\n\n")
	}

	// 2. Hard constraints (risk control) - dynamically generated
	sb.WriteString("# Hard Constraints (Risk Control)\n\n")
	sb.WriteString("1. Risk-reward ratio: Must be ≥ 1:3 (risk 1%, earn 3%+ return)\n")
	sb.WriteString("2. Maximum positions: 3 coins (quality > quantity)\n")
	sb.WriteString(fmt.Sprintf("3. Single coin position: Altcoins %.0f-%.0f U | BTC/ETH %.0f-%.0f U\n",
		accountEquity*0.8, accountEquity*1.5, accountEquity*5, accountEquity*10))
	sb.WriteString(fmt.Sprintf("4. Leverage limit: **Altcoins max %dx leverage** | **BTC/ETH max %dx leverage** (⚠️ Strictly enforced, cannot exceed)\n", altcoinLeverage, btcEthLeverage))
	sb.WriteString("5. Margin: Total usage ≤ 90%\n")
	sb.WriteString("6. Opening amount: Recommended **≥12 USDT** (exchange minimum notional value 10 USDT + safety margin)\n\n")

	// 3. Output format - dynamically generated
	sb.WriteString("# Output Format\n\n")
	sb.WriteString("Step 1: Chain of Thought (plain text)\n")
	sb.WriteString("Briefly analyze your thinking process\n\n")
	sb.WriteString("Step 2: JSON Decision Array\n\n")
	sb.WriteString("```json\n[\n")
	sb.WriteString(fmt.Sprintf("  {\"symbol\": \"BTCUSDT\", \"action\": \"open_short\", \"leverage\": %d, \"position_size_usd\": %.0f, \"stop_loss\": 97000, \"take_profit\": 91000, \"confidence\": 85, \"risk_usd\": 300, \"reasoning\": \"Downtrend + MACD death cross\"},\n", btcEthLeverage, accountEquity*5))
	sb.WriteString("  {\"symbol\": \"ETHUSDT\", \"action\": \"close_long\", \"reasoning\": \"Take profit exit\"}\n")
	sb.WriteString("]\n```\n\n")
	sb.WriteString("Field descriptions:\n")
	sb.WriteString("- `action`: open_long | open_short | close_long | close_short | hold | wait\n")
	sb.WriteString("- `confidence`: 0-100 (opening positions recommended ≥75)\n")
	sb.WriteString("- Required when opening: leverage, position_size_usd, stop_loss, take_profit, confidence, risk_usd, reasoning\n\n")

	return sb.String()
}

// buildUserPrompt builds User Prompt (dynamic data)
func buildUserPrompt(ctx *Context) string {
	var sb strings.Builder

	// System status
	sb.WriteString(fmt.Sprintf("Time: %s | Cycle: #%d | Runtime: %d minutes\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	// BTC market
	if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
		sb.WriteString(fmt.Sprintf("BTC: %.2f (1h: %+.2f%%, 4h: %+.2f%%) | MACD: %.4f | RSI: %.2f\n\n",
			btcData.CurrentPrice, btcData.PriceChange1h, btcData.PriceChange4h,
			btcData.CurrentMACD, btcData.CurrentRSI7))
	}

	// Account
	sb.WriteString(fmt.Sprintf("Account: Equity %.2f | Balance %.2f (%.1f%%) | P&L %+.2f%% | Margin %.1f%% | Positions %d\n\n",
		ctx.Account.TotalEquity,
		ctx.Account.AvailableBalance,
		(ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100,
		ctx.Account.TotalPnLPct,
		ctx.Account.MarginUsedPct,
		ctx.Account.PositionCount))

	// Positions (complete market data)
	if len(ctx.Positions) > 0 {
		sb.WriteString("## Current Positions\n")
		for i, pos := range ctx.Positions {
			// Calculate holding duration
			holdingDuration := ""
			if pos.UpdateTime > 0 {
				durationMs := time.Now().UnixMilli() - pos.UpdateTime
				durationMin := durationMs / (1000 * 60) // Convert to minutes
				if durationMin < 60 {
					holdingDuration = fmt.Sprintf(" | Holding duration %d minutes", durationMin)
				} else {
					durationHour := durationMin / 60
					durationMinRemainder := durationMin % 60
					holdingDuration = fmt.Sprintf(" | Holding duration %d hours %d minutes", durationHour, durationMinRemainder)
				}
			}

			sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %.4f Current %.4f | P&L %+.2f%% | Leverage %dx | Margin %.0f | Liquidation %.4f%s\n\n",
				i+1, pos.Symbol, strings.ToUpper(pos.Side),
				pos.EntryPrice, pos.MarkPrice, pos.UnrealizedPnLPct,
				pos.Leverage, pos.MarginUsed, pos.LiquidationPrice, holdingDuration))

			// Use FormatMarketData to output complete market data
			if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
				sb.WriteString(market.Format(marketData))
				sb.WriteString("\n")
			}
		}
	} else {
		sb.WriteString("Current positions: None\n\n")
	}

	// Candidate coins (complete market data)
	sb.WriteString(fmt.Sprintf("## Candidate Coins (%d)\n\n", len(ctx.MarketDataMap)))
	displayedCount := 0
	for _, coin := range ctx.CandidateCoins {
		marketData, hasData := ctx.MarketDataMap[coin.Symbol]
		if !hasData {
			continue
		}
		displayedCount++

		sourceTags := ""
		if len(coin.Sources) > 1 {
			sourceTags = " (AI500+OI_Top dual signal)"
		} else if len(coin.Sources) == 1 && coin.Sources[0] == "oi_top" {
			sourceTags = " (OI_Top position growth)"
		}

		// Use FormatMarketData to output complete market data
		sb.WriteString(fmt.Sprintf("### %d. %s%s\n\n", displayedCount, coin.Symbol, sourceTags))
		sb.WriteString(market.Format(marketData))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// Sharpe ratio (direct value, no complex formatting)
	if ctx.Performance != nil {
		// Extract SharpeRatio directly from interface{}
		type PerformanceData struct {
			SharpeRatio float64 `json:"sharpe_ratio"`
		}
		var perfData PerformanceData
		if jsonData, err := json.Marshal(ctx.Performance); err == nil {
			if err := json.Unmarshal(jsonData, &perfData); err == nil {
				sb.WriteString(fmt.Sprintf("## 📊 Sharpe Ratio: %.2f\n\n", perfData.SharpeRatio))
			}
		}
	}

	sb.WriteString("---\n\n")
	sb.WriteString("Please analyze and output decisions (chain of thought + JSON)\n")

	return sb.String()
}

// parseFullDecisionResponse parses AI's complete decision response
func parseFullDecisionResponse(aiResponse string, accountEquity float64, btcEthLeverage, altcoinLeverage int) (*FullDecision, error) {
	// 1. Extract chain of thought
	cotTrace := extractCoTTrace(aiResponse)

	// 2. Extract JSON decision list
	decisions, err := extractDecisions(aiResponse)
	if err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: []Decision{},
		}, fmt.Errorf("failed to extract decisions: %w", err)
	}

	// 3. Validate decisions
	if err := validateDecisions(decisions, accountEquity, btcEthLeverage, altcoinLeverage); err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: decisions,
		}, fmt.Errorf("decision validation failed: %w", err)
	}

	return &FullDecision{
		CoTTrace:  cotTrace,
		Decisions: decisions,
	}, nil
}

// extractCoTTrace extracts chain of thought analysis
func extractCoTTrace(response string) string {
	// Find the start position of JSON array
	jsonStart := strings.Index(response, "[")

	if jsonStart > 0 {
		// Chain of thought is the content before JSON array
		return strings.TrimSpace(response[:jsonStart])
	}

	// If JSON not found, entire response is chain of thought
	return strings.TrimSpace(response)
}

// extractDecisions extracts JSON decision list
func extractDecisions(response string) ([]Decision, error) {
	// Pre-clean: remove zero-width/BOM characters
	s := removeInvisibleRunes(response)
	s = strings.TrimSpace(s)

	// 🔧 Critical fix: fix full-width characters before regex matching!
	// Otherwise regex \[ cannot match full-width ［
	s = fixMissingQuotes(s)

	// 1) Priority: extract from ```json code block
	if m := reJSONFence.FindStringSubmatch(s); m != nil && len(m) > 1 {
		jsonContent := strings.TrimSpace(m[1])
		jsonContent = compactArrayOpen(jsonContent) // Normalize "[ {" to "[{"
		jsonContent = fixMissingQuotes(jsonContent) // Second fix (prevent remaining full-width after regex extraction)
		if err := validateJSONFormat(jsonContent); err != nil {
			return nil, fmt.Errorf("JSON format validation failed: %w\nJSON content: %s\nFull response:\n%s", err, jsonContent, response)
		}
		var decisions []Decision
		if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
			return nil, fmt.Errorf("JSON parsing failed: %w\nJSON content: %s", err, jsonContent)
		}
		return decisions, nil
	}

	// 2) Fallback: search entire text for first object array
	// Note: s has already been processed by fixMissingQuotes(), full-width characters converted to half-width
	jsonContent := strings.TrimSpace(reJSONArray.FindString(s))
	if jsonContent == "" {
		// 🔧 Safe fallback: when AI only outputs chain of thought without JSON, generate fallback decision (avoid system crash)
		log.Printf("⚠️  [SafeFallback] AI did not output JSON decision, entering safe wait mode (AI response without JSON, entering safe wait mode)")

		// Extract chain of thought summary (max 240 characters)
		cotSummary := s
		if len(cotSummary) > 240 {
			cotSummary = cotSummary[:240] + "..."
		}

		// Generate fallback decision: all coins enter wait state
		fallbackDecision := Decision{
			Symbol:    "ALL",
			Action:    "wait",
			Reasoning: fmt.Sprintf("Model did not output structured JSON decision, entering safe wait; summary: %s", cotSummary),
		}

		return []Decision{fallbackDecision}, nil
	}

	// 🔧 Normalize format (full-width characters already fixed earlier)
	jsonContent = compactArrayOpen(jsonContent)
	jsonContent = fixMissingQuotes(jsonContent) // Second fix (prevent remaining full-width after regex extraction)

	// 🔧 Validate JSON format (detect common errors)
	if err := validateJSONFormat(jsonContent); err != nil {
		return nil, fmt.Errorf("JSON format validation failed: %w\nJSON content: %s\nFull response:\n%s", err, jsonContent, response)
	}

	// Parse JSON
	var decisions []Decision
	if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
		return nil, fmt.Errorf("JSON parsing failed: %w\nJSON content: %s", err, jsonContent)
	}

	return decisions, nil
}

// fixMissingQuotes replaces Chinese quotes and full-width characters with English quotes and half-width characters (avoid AI outputting full-width JSON characters causing parsing failure)
func fixMissingQuotes(jsonStr string) string {
	// Replace Chinese quotes
	jsonStr = strings.ReplaceAll(jsonStr, "\u201c", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u201d", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u2018", "'")  // '
	jsonStr = strings.ReplaceAll(jsonStr, "\u2019", "'")  // '

	// ⚠️ Replace full-width brackets, colons, commas (prevent AI from outputting full-width JSON characters)
	jsonStr = strings.ReplaceAll(jsonStr, "［", "[") // U+FF3B Full-width left square bracket
	jsonStr = strings.ReplaceAll(jsonStr, "］", "]") // U+FF3D Full-width right square bracket
	jsonStr = strings.ReplaceAll(jsonStr, "｛", "{") // U+FF5B Full-width left curly bracket
	jsonStr = strings.ReplaceAll(jsonStr, "｝", "}") // U+FF5D Full-width right curly bracket
	jsonStr = strings.ReplaceAll(jsonStr, "：", ":") // U+FF1A Full-width colon
	jsonStr = strings.ReplaceAll(jsonStr, "，", ",") // U+FF0C Full-width comma

	// ⚠️ Replace CJK punctuation marks (AI may output these in Chinese context)
	jsonStr = strings.ReplaceAll(jsonStr, "【", "[") // CJK left corner bracket U+3010
	jsonStr = strings.ReplaceAll(jsonStr, "】", "]") // CJK right corner bracket U+3011
	jsonStr = strings.ReplaceAll(jsonStr, "〔", "[") // CJK left tortoise shell bracket U+3014
	jsonStr = strings.ReplaceAll(jsonStr, "〕", "]") // CJK right tortoise shell bracket U+3015
	jsonStr = strings.ReplaceAll(jsonStr, "、", ",") // CJK ideographic comma U+3001

	// ⚠️ Replace full-width space with half-width space (JSON should not have full-width spaces)
	jsonStr = strings.ReplaceAll(jsonStr, "　", " ") // U+3000 Full-width space

	return jsonStr
}

// validateJSONFormat validates JSON format, detects common errors
func validateJSONFormat(jsonStr string) error {
	trimmed := strings.TrimSpace(jsonStr)

	// Allow any whitespace (including zero-width) between [ and {
	if !reArrayHead.MatchString(trimmed) {
		// Check if it's a pure number/range array (common error)
		if strings.HasPrefix(trimmed, "[") && !strings.Contains(trimmed[:min(20, len(trimmed))], "{") {
			return fmt.Errorf("not a valid decision array (must contain objects {}), actual content: %s", trimmed[:min(50, len(trimmed))])
		}
		return fmt.Errorf("JSON must start with [{ (whitespace allowed), actual: %s", trimmed[:min(20, len(trimmed))])
	}

	// Check if it contains range symbol ~ (LLM common error)
	if strings.Contains(jsonStr, "~") {
		return fmt.Errorf("JSON cannot contain range symbol ~, all numbers must be exact single values")
	}

	// Check if it contains thousand separators (e.g., 98,000)
	// Use simple pattern matching: digit+comma+3 digits
	for i := 0; i < len(jsonStr)-4; i++ {
		if jsonStr[i] >= '0' && jsonStr[i] <= '9' &&
			jsonStr[i+1] == ',' &&
			jsonStr[i+2] >= '0' && jsonStr[i+2] <= '9' &&
			jsonStr[i+3] >= '0' && jsonStr[i+3] <= '9' &&
			jsonStr[i+4] >= '0' && jsonStr[i+4] <= '9' {
			return fmt.Errorf("JSON numbers cannot contain thousand separator commas, found: %s", jsonStr[i:min(i+10, len(jsonStr))])
		}
	}

	return nil
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// removeInvisibleRunes removes zero-width characters and BOM, avoiding invisible prefixes that break validation
func removeInvisibleRunes(s string) string {
	return reInvisibleRunes.ReplaceAllString(s, "")
}

// compactArrayOpen normalizes opening "[ {" → "[{"
func compactArrayOpen(s string) string {
	return reArrayOpenSpace.ReplaceAllString(strings.TrimSpace(s), "[{")
}

// validateDecisions validates all decisions (requires account info and leverage configuration)
func validateDecisions(decisions []Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int) error {
	for i, decision := range decisions {
		if err := validateDecision(&decision, accountEquity, btcEthLeverage, altcoinLeverage); err != nil {
			return fmt.Errorf("decision #%d validation failed: %w", i+1, err)
		}
	}
	return nil
}

// findMatchingBracket finds matching right bracket
func findMatchingBracket(s string, start int) int {
	if start >= len(s) || s[start] != '[' {
		return -1
	}

	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

// validateDecision validates a single decision's validity
func validateDecision(d *Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int) error {
	// Validate action
	validActions := map[string]bool{
		"open_long":          true,
		"open_short":         true,
		"close_long":         true,
		"close_short":        true,
		"update_stop_loss":   true,
		"update_take_profit": true,
		"partial_close":      true,
		"hold":               true,
		"wait":               true,
	}

	if !validActions[d.Action] {
		return fmt.Errorf("invalid action: %s", d.Action)
	}

	// Opening position operations must provide complete parameters
	if d.Action == "open_long" || d.Action == "open_short" {
		// Use configured leverage limit based on coin type
		maxLeverage := altcoinLeverage          // Altcoins use configured leverage
		maxPositionValue := accountEquity * 1.5 // Altcoins max 1.5x account equity
		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			maxLeverage = btcEthLeverage          // BTC and ETH use configured leverage
			maxPositionValue = accountEquity * 10 // BTC/ETH max 10x account equity
		}

		if d.Leverage <= 0 || d.Leverage > maxLeverage {
			return fmt.Errorf("leverage must be between 1-%d (%s, current config limit %dx): %d", maxLeverage, d.Symbol, maxLeverage, d.Leverage)
		}
		if d.PositionSizeUSD <= 0 {
			return fmt.Errorf("position size must be greater than 0: %.2f", d.PositionSizeUSD)
		}

		// ✅ Validate minimum opening amount (prevent quantity formatting to 0 error)
		// Binance minimum notional value 10 USDT + safety margin
		const minPositionSizeGeneral = 12.0 // 10 + 20% safety margin
		const minPositionSizeBTCETH = 60.0  // BTC/ETH need larger amount due to high price and precision limits (more flexible)

		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			if d.PositionSizeUSD < minPositionSizeBTCETH {
				return fmt.Errorf("%s opening amount too small (%.2f USDT), must be ≥%.2f USDT (due to high price and precision limits, avoid quantity rounding to 0)", d.Symbol, d.PositionSizeUSD, minPositionSizeBTCETH)
			}
		} else {
			if d.PositionSizeUSD < minPositionSizeGeneral {
				return fmt.Errorf("opening amount too small (%.2f USDT), must be ≥%.2f USDT (Binance minimum notional value requirement)", d.PositionSizeUSD, minPositionSizeGeneral)
			}
		}

		// Validate position value upper limit (add 1% tolerance to avoid floating point precision issues)
		tolerance := maxPositionValue * 0.01 // 1% tolerance
		if d.PositionSizeUSD > maxPositionValue+tolerance {
			if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
				return fmt.Errorf("BTC/ETH single coin position value cannot exceed %.0f USDT (10x account equity), actual: %.0f", maxPositionValue, d.PositionSizeUSD)
			} else {
				return fmt.Errorf("altcoin single coin position value cannot exceed %.0f USDT (1.5x account equity), actual: %.0f", maxPositionValue, d.PositionSizeUSD)
			}
		}
		if d.StopLoss <= 0 || d.TakeProfit <= 0 {
			return fmt.Errorf("stop loss and take profit must be greater than 0")
		}

		// Validate stop loss and take profit reasonableness
		if d.Action == "open_long" {
			if d.StopLoss >= d.TakeProfit {
				return fmt.Errorf("when going long, stop loss price must be less than take profit price")
			}
		} else {
			if d.StopLoss <= d.TakeProfit {
				return fmt.Errorf("when going short, stop loss price must be greater than take profit price")
			}
		}

		// Validate risk-reward ratio (must be ≥1:3)
		// Calculate entry price (assume current market price)
		var entryPrice float64
		if d.Action == "open_long" {
			// Long: entry price between stop loss and take profit
			entryPrice = d.StopLoss + (d.TakeProfit-d.StopLoss)*0.2 // Assume entry at 20% position
		} else {
			// Short: entry price between stop loss and take profit
			entryPrice = d.StopLoss - (d.StopLoss-d.TakeProfit)*0.2 // Assume entry at 20% position
		}

		var riskPercent, rewardPercent, riskRewardRatio float64
		if d.Action == "open_long" {
			riskPercent = (entryPrice - d.StopLoss) / entryPrice * 100
			rewardPercent = (d.TakeProfit - entryPrice) / entryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		} else {
			riskPercent = (d.StopLoss - entryPrice) / entryPrice * 100
			rewardPercent = (entryPrice - d.TakeProfit) / entryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		}

		// Hard constraint: risk-reward ratio must be ≥3.0
		if riskRewardRatio < 3.0 {
			return fmt.Errorf("risk-reward ratio too low (%.2f:1), must be ≥3.0:1 [risk:%.2f%% reward:%.2f%%] [stop loss:%.2f take profit:%.2f]",
				riskRewardRatio, riskPercent, rewardPercent, d.StopLoss, d.TakeProfit)
		}
	}

	// Dynamic stop loss adjustment validation
	if d.Action == "update_stop_loss" {
		if d.NewStopLoss <= 0 {
			return fmt.Errorf("new stop loss price must be greater than 0: %.2f", d.NewStopLoss)
		}
	}

	// Dynamic take profit adjustment validation
	if d.Action == "update_take_profit" {
		if d.NewTakeProfit <= 0 {
			return fmt.Errorf("new take profit price must be greater than 0: %.2f", d.NewTakeProfit)
		}
	}

	// Partial close validation
	if d.Action == "partial_close" {
		if d.ClosePercentage <= 0 || d.ClosePercentage > 100 {
			return fmt.Errorf("close percentage must be between 0-100: %.1f", d.ClosePercentage)
		}
	}

	return nil
}
