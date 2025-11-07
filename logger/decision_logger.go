package logger

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"time"
)

// DecisionRecord decision record
type DecisionRecord struct {
	Timestamp      time.Time          `json:"timestamp"`       // Decision time
	CycleNumber    int                `json:"cycle_number"`    // Cycle number
	SystemPrompt   string             `json:"system_prompt"`   // System prompt (system prompt sent to AI)
	InputPrompt    string             `json:"input_prompt"`    // Input prompt sent to AI
	CoTTrace       string             `json:"cot_trace"`       // AI chain of thought (output)
	DecisionJSON   string             `json:"decision_json"`   // Decision JSON
	AccountState   AccountSnapshot    `json:"account_state"`   // Account state snapshot
	Positions      []PositionSnapshot `json:"positions"`       // Position snapshot
	CandidateCoins []string           `json:"candidate_coins"` // Candidate coin list
	Decisions      []DecisionAction   `json:"decisions"`       // Executed decisions
	ExecutionLog   []string           `json:"execution_log"`   // Execution log
	Success        bool               `json:"success"`         // Whether successful
	ErrorMessage   string             `json:"error_message"`   // Error message (if any)
}

// AccountSnapshot account state snapshot
type AccountSnapshot struct {
	TotalBalance          float64 `json:"total_balance"`
	AvailableBalance      float64 `json:"available_balance"`
	TotalUnrealizedProfit float64 `json:"total_unrealized_profit"`
	PositionCount         int     `json:"position_count"`
	MarginUsedPct         float64 `json:"margin_used_pct"`
}

// PositionSnapshot position snapshot
type PositionSnapshot struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"`
	PositionAmt      float64 `json:"position_amt"`
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	UnrealizedProfit float64 `json:"unrealized_profit"`
	Leverage         float64 `json:"leverage"`
	LiquidationPrice float64 `json:"liquidation_price"`
}

// DecisionAction decision action
type DecisionAction struct {
	Action    string    `json:"action"`    // open_long, open_short, close_long, close_short, update_stop_loss, update_take_profit, partial_close
	Symbol    string    `json:"symbol"`    // Symbol
	Quantity  float64   `json:"quantity"`  // Quantity (used for partial close)
	Leverage  int       `json:"leverage"`  // Leverage (when opening position)
	Price     float64   `json:"price"`     // Execution price
	OrderID   int64     `json:"order_id"`  // Order ID
	Timestamp time.Time `json:"timestamp"` // Execution time
	Success   bool      `json:"success"`   // Whether successful
	Error     string    `json:"error"`     // Error message
}

// DecisionLogger decision log recorder
type DecisionLogger struct {
	logDir      string
	cycleNumber int
}

// NewDecisionLogger creates a decision log recorder
func NewDecisionLogger(logDir string) *DecisionLogger {
	if logDir == "" {
		logDir = "decision_logs"
	}

	// Ensure log directory exists
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("⚠ Failed to create log directory: %v\n", err)
	}

	return &DecisionLogger{
		logDir:      logDir,
		cycleNumber: 0,
	}
}

// LogDecision records a decision
func (l *DecisionLogger) LogDecision(record *DecisionRecord) error {
	l.cycleNumber++
	record.CycleNumber = l.cycleNumber
	record.Timestamp = time.Now()

	// Generate filename: decision_YYYYMMDD_HHMMSS_cycleN.json
	filename := fmt.Sprintf("decision_%s_cycle%d.json",
		record.Timestamp.Format("20060102_150405"),
		record.CycleNumber)

	filepath := filepath.Join(l.logDir, filename)

	// Serialize to JSON (with indentation for readability)
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize decision record: %w", err)
	}

	// Write to file
	if err := ioutil.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write decision record: %w", err)
	}

	fmt.Printf("📝 Decision record saved: %s\n", filename)
	return nil
}

// GetLatestRecords gets the latest N records (in chronological order: from old to new)
func (l *DecisionLogger) GetLatestRecords(n int) ([]*DecisionRecord, error) {
	files, err := ioutil.ReadDir(l.logDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read log directory: %w", err)
	}

	// First collect in reverse order by modification time (newest first)
	var records []*DecisionRecord
	count := 0
	for i := len(files) - 1; i >= 0 && count < n; i-- {
		file := files[i]
		if file.IsDir() {
			continue
		}

		filepath := filepath.Join(l.logDir, file.Name())
		data, err := ioutil.ReadFile(filepath)
		if err != nil {
			continue
		}

		var record DecisionRecord
		if err := json.Unmarshal(data, &record); err != nil {
			continue
		}

		records = append(records, &record)
		count++
	}

	// Reverse array to arrange from old to new (for chart display)
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	return records, nil
}

// GetRecordByDate gets all records for the specified date
func (l *DecisionLogger) GetRecordByDate(date time.Time) ([]*DecisionRecord, error) {
	dateStr := date.Format("20060102")
	pattern := filepath.Join(l.logDir, fmt.Sprintf("decision_%s_*.json", dateStr))

	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to find log files: %w", err)
	}

	var records []*DecisionRecord
	for _, filepath := range files {
		data, err := ioutil.ReadFile(filepath)
		if err != nil {
			continue
		}

		var record DecisionRecord
		if err := json.Unmarshal(data, &record); err != nil {
			continue
		}

		records = append(records, &record)
	}

	return records, nil
}

// CleanOldRecords cleans old records from N days ago
func (l *DecisionLogger) CleanOldRecords(days int) error {
	cutoffTime := time.Now().AddDate(0, 0, -days)

	files, err := ioutil.ReadDir(l.logDir)
	if err != nil {
		return fmt.Errorf("failed to read log directory: %w", err)
	}

	removedCount := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if file.ModTime().Before(cutoffTime) {
			filepath := filepath.Join(l.logDir, file.Name())
			if err := os.Remove(filepath); err != nil {
				fmt.Printf("⚠ Failed to delete old record %s: %v\n", file.Name(), err)
				continue
			}
			removedCount++
		}
	}

	if removedCount > 0 {
		fmt.Printf("🗑️ Cleaned %d old records (%d days ago)\n", removedCount, days)
	}

	return nil
}

// GetStatistics gets statistics
func (l *DecisionLogger) GetStatistics() (*Statistics, error) {
	files, err := ioutil.ReadDir(l.logDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read log directory: %w", err)
	}

	stats := &Statistics{}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filepath := filepath.Join(l.logDir, file.Name())
		data, err := ioutil.ReadFile(filepath)
		if err != nil {
			continue
		}

		var record DecisionRecord
		if err := json.Unmarshal(data, &record); err != nil {
			continue
		}

		stats.TotalCycles++

		for _, action := range record.Decisions {
			if action.Success {
				switch action.Action {
				case "open_long", "open_short":
					stats.TotalOpenPositions++
				case "close_long", "close_short", "auto_close_long", "auto_close_short":
					stats.TotalClosePositions++
					// 🔧 BUG FIX: partial_close is not counted in TotalClosePositions to avoid double counting
					// case "partial_close": // Not counted, as only full close counts as one
					// update_stop_loss and update_take_profit are not counted in statistics
				}
			}
		}

		if record.Success {
			stats.SuccessfulCycles++
		} else {
			stats.FailedCycles++
		}
	}

	return stats, nil
}

// Statistics statistical information
type Statistics struct {
	TotalCycles         int `json:"total_cycles"`
	SuccessfulCycles    int `json:"successful_cycles"`
	FailedCycles        int `json:"failed_cycles"`
	TotalOpenPositions  int `json:"total_open_positions"`
	TotalClosePositions int `json:"total_close_positions"`
}

// TradeOutcome single trade result
type TradeOutcome struct {
	Symbol        string    `json:"symbol"`         // Symbol
	Side          string    `json:"side"`           // long/short
	Quantity      float64   `json:"quantity"`       // Position quantity
	Leverage      int       `json:"leverage"`       // Leverage multiplier
	OpenPrice     float64   `json:"open_price"`     // Open price
	ClosePrice    float64   `json:"close_price"`    // Close price
	PositionValue float64   `json:"position_value"` // Position value (quantity × openPrice)
	MarginUsed    float64   `json:"margin_used"`    // Margin used (positionValue / leverage)
	PnL           float64   `json:"pn_l"`           // Profit/Loss (USDT)
	PnLPct        float64   `json:"pn_l_pct"`       // Profit/Loss percentage (relative to margin)
	Duration      string    `json:"duration"`       // Position duration
	OpenTime      time.Time `json:"open_time"`      // Open time
	CloseTime     time.Time `json:"close_time"`     // Close time
	WasStopLoss   bool      `json:"was_stop_loss"`  // Whether stop loss was triggered
}

// PerformanceAnalysis trading performance analysis
type PerformanceAnalysis struct {
	TotalTrades   int                           `json:"total_trades"`   // Total number of trades
	WinningTrades int                           `json:"winning_trades"` // Number of winning trades
	LosingTrades  int                           `json:"losing_trades"`  // Number of losing trades
	WinRate       float64                       `json:"win_rate"`       // Win rate
	AvgWin        float64                       `json:"avg_win"`        // Average win
	AvgLoss       float64                       `json:"avg_loss"`       // Average loss
	ProfitFactor  float64                       `json:"profit_factor"`  // Profit factor
	SharpeRatio   float64                       `json:"sharpe_ratio"`   // Sharpe ratio (risk-adjusted return)
	RecentTrades  []TradeOutcome                `json:"recent_trades"`  // Recent N trades
	SymbolStats   map[string]*SymbolPerformance `json:"symbol_stats"`   // Performance by symbol
	BestSymbol    string                        `json:"best_symbol"`    // Best performing symbol
	WorstSymbol   string                        `json:"worst_symbol"`   // Worst performing symbol
}

// SymbolPerformance symbol performance statistics
type SymbolPerformance struct {
	Symbol        string  `json:"symbol"`         // Symbol
	TotalTrades   int     `json:"total_trades"`   // Number of trades
	WinningTrades int     `json:"winning_trades"` // Number of wins
	LosingTrades  int     `json:"losing_trades"`  // Number of losses
	WinRate       float64 `json:"win_rate"`       // Win rate
	TotalPnL      float64 `json:"total_pn_l"`     // Total profit/loss
	AvgPnL        float64 `json:"avg_pn_l"`       // Average profit/loss
}

// AnalyzePerformance analyzes trading performance of the last N cycles
func (l *DecisionLogger) AnalyzePerformance(lookbackCycles int) (*PerformanceAnalysis, error) {
	records, err := l.GetLatestRecords(lookbackCycles)
	if err != nil {
		return nil, fmt.Errorf("failed to read historical records: %w", err)
	}

	if len(records) == 0 {
		return &PerformanceAnalysis{
			RecentTrades: []TradeOutcome{},
			SymbolStats:  make(map[string]*SymbolPerformance),
		}, nil
	}

	analysis := &PerformanceAnalysis{
		RecentTrades: []TradeOutcome{},
		SymbolStats:  make(map[string]*SymbolPerformance),
	}

	// Track position state: symbol_side -> {side, openPrice, openTime, quantity, leverage}
	openPositions := make(map[string]map[string]interface{})

	// To avoid matching failures when open records are outside the window, first find all unclosed positions from all historical records
	// Get more historical records to build complete position state (use larger window)
	allRecords, err := l.GetLatestRecords(lookbackCycles * 3) // Expand window by 3x
	if err == nil && len(allRecords) > len(records) {
		// First collect all open records from the expanded window
		for _, record := range allRecords {
			for _, action := range record.Decisions {
				if !action.Success {
					continue
				}

				symbol := action.Symbol
				side := ""
				if action.Action == "open_long" || action.Action == "close_long" || action.Action == "partial_close" || action.Action == "auto_close_long" {
					side = "long"
				} else if action.Action == "open_short" || action.Action == "close_short" || action.Action == "auto_close_short" {
					side = "short"
				}

				// partial_close needs to determine direction based on position
				if action.Action == "partial_close" && side == "" {
					for key, pos := range openPositions {
						if posSymbol, _ := pos["side"].(string); key == symbol+"_"+posSymbol {
							side = posSymbol
							break
						}
					}
				}

				posKey := symbol + "_" + side

				switch action.Action {
				case "open_long", "open_short":
					// Record open position
					openPositions[posKey] = map[string]interface{}{
						"side":      side,
						"openPrice": action.Price,
						"openTime":  action.Timestamp,
						"quantity":  action.Quantity,
						"leverage":  action.Leverage,
					}
				case "close_long", "close_short", "auto_close_long", "auto_close_short":
					// Remove closed position record
					delete(openPositions, posKey)
					// partial_close not processed, keep position record
				}
			}
		}
	}

	// Iterate through records in analysis window to generate trade results
	for _, record := range records {
		for _, action := range record.Decisions {
			if !action.Success {
				continue
			}

			symbol := action.Symbol
			side := ""
			if action.Action == "open_long" || action.Action == "close_long" || action.Action == "partial_close" || action.Action == "auto_close_long" {
				side = "long"
			} else if action.Action == "open_short" || action.Action == "close_short" || action.Action == "auto_close_short" {
				side = "short"
			}

			// partial_close needs to determine direction based on position
			if action.Action == "partial_close" {
				// Find position direction from openPositions
				for key, pos := range openPositions {
					if posSymbol, _ := pos["side"].(string); key == symbol+"_"+posSymbol {
						side = posSymbol
						break
					}
				}
			}

			posKey := symbol + "_" + side // Use symbol_side as key to distinguish long/short positions

			switch action.Action {
			case "open_long", "open_short":
				// Update open position record (may have been recorded during pre-fill)
				openPositions[posKey] = map[string]interface{}{
					"side":               side,
					"openPrice":          action.Price,
					"openTime":           action.Timestamp,
					"quantity":           action.Quantity,
					"leverage":           action.Leverage,
					"remainingQuantity":  action.Quantity, // 🔧 BUG FIX: track remaining quantity
					"accumulatedPnL":     0.0,             // 🔧 BUG FIX: accumulate partial close PnL
					"partialCloseCount":  0,               // 🔧 BUG FIX: partial close count
					"partialCloseVolume": 0.0,             // 🔧 BUG FIX: partial close total volume
				}

			case "close_long", "close_short", "partial_close", "auto_close_long", "auto_close_short":
				// Find corresponding open position record (may come from pre-fill or current window)
				if openPos, exists := openPositions[posKey]; exists {
					openPrice := openPos["openPrice"].(float64)
					openTime := openPos["openTime"].(time.Time)
					side := openPos["side"].(string)
					quantity := openPos["quantity"].(float64)
					leverage := openPos["leverage"].(int)

					// 🔧 BUG FIX: get tracking fields (initialize if not exists)
					remainingQty, _ := openPos["remainingQuantity"].(float64)
					if remainingQty == 0 {
						remainingQty = quantity // Compatible with old data (no remainingQuantity field)
					}
					accumulatedPnL, _ := openPos["accumulatedPnL"].(float64)
					partialCloseCount, _ := openPos["partialCloseCount"].(int)
					partialCloseVolume, _ := openPos["partialCloseVolume"].(float64)

					// For partial_close, use actual close quantity; otherwise use remaining position quantity
					actualQuantity := remainingQty
					if action.Action == "partial_close" {
						actualQuantity = action.Quantity
					}

					// Calculate PnL for this close (USDT)
					var pnl float64
					if side == "long" {
						pnl = actualQuantity * (action.Price - openPrice)
					} else {
						pnl = actualQuantity * (openPrice - action.Price)
					}

					// 🔧 BUG FIX: handle partial_close aggregation logic
					if action.Action == "partial_close" {
						// Accumulate PnL and quantity
						accumulatedPnL += pnl
						remainingQty -= actualQuantity
						partialCloseCount++
						partialCloseVolume += actualQuantity

						// Update openPositions (keep position record, but update tracking data)
						openPos["remainingQuantity"] = remainingQty
						openPos["accumulatedPnL"] = accumulatedPnL
						openPos["partialCloseCount"] = partialCloseCount
						openPos["partialCloseVolume"] = partialCloseVolume

						// Check if fully closed
						if remainingQty <= 0.0001 { // Use small threshold to avoid floating point errors
							// ✅ Fully closed: record as one complete trade
							positionValue := quantity * openPrice
							marginUsed := positionValue / float64(leverage)
							pnlPct := 0.0
							if marginUsed > 0 {
								pnlPct = (accumulatedPnL / marginUsed) * 100
							}

							outcome := TradeOutcome{
								Symbol:        symbol,
								Side:          side,
								Quantity:      quantity, // Use original total quantity
								Leverage:      leverage,
								OpenPrice:     openPrice,
								ClosePrice:    action.Price, // Last close price
								PositionValue: positionValue,
								MarginUsed:    marginUsed,
								PnL:           accumulatedPnL, // 🔧 Use accumulated PnL
								PnLPct:        pnlPct,
								Duration:      action.Timestamp.Sub(openTime).String(),
								OpenTime:      openTime,
								CloseTime:     action.Timestamp,
							}

							analysis.RecentTrades = append(analysis.RecentTrades, outcome)
							analysis.TotalTrades++ // 🔧 Only count when fully closed

							// Classify trade
							if accumulatedPnL > 0 {
								analysis.WinningTrades++
								analysis.AvgWin += accumulatedPnL
							} else if accumulatedPnL < 0 {
								analysis.LosingTrades++
								analysis.AvgLoss += accumulatedPnL
							}

							// Update symbol statistics
							if _, exists := analysis.SymbolStats[symbol]; !exists {
								analysis.SymbolStats[symbol] = &SymbolPerformance{
									Symbol: symbol,
								}
							}
							stats := analysis.SymbolStats[symbol]
							stats.TotalTrades++
							stats.TotalPnL += accumulatedPnL
							if accumulatedPnL > 0 {
								stats.WinningTrades++
							} else if accumulatedPnL < 0 {
								stats.LosingTrades++
							}

							// Delete position record
							delete(openPositions, posKey)
						}
						// ⚠️ Otherwise do nothing (wait for subsequent partial_close or full close)

					} else {
						// 🔧 Full close (close_long/close_short/auto_close)
						// If there were partial closes before, need to add accumulated PnL
						totalPnL := accumulatedPnL + pnl

						positionValue := quantity * openPrice
						marginUsed := positionValue / float64(leverage)
						pnlPct := 0.0
						if marginUsed > 0 {
							pnlPct = (totalPnL / marginUsed) * 100
						}

						outcome := TradeOutcome{
							Symbol:        symbol,
							Side:          side,
							Quantity:      quantity, // Use original total quantity
							Leverage:      leverage,
							OpenPrice:     openPrice,
							ClosePrice:    action.Price,
							PositionValue: positionValue,
							MarginUsed:    marginUsed,
							PnL:           totalPnL, // 🔧 Include PnL from previous partial closes
							PnLPct:        pnlPct,
							Duration:      action.Timestamp.Sub(openTime).String(),
							OpenTime:      openTime,
							CloseTime:     action.Timestamp,
						}

						analysis.RecentTrades = append(analysis.RecentTrades, outcome)
						analysis.TotalTrades++

						// Classify trade
						if totalPnL > 0 {
							analysis.WinningTrades++
							analysis.AvgWin += totalPnL
						} else if totalPnL < 0 {
							analysis.LosingTrades++
							analysis.AvgLoss += totalPnL
						}

						// Update symbol statistics
						if _, exists := analysis.SymbolStats[symbol]; !exists {
							analysis.SymbolStats[symbol] = &SymbolPerformance{
								Symbol: symbol,
							}
						}
						stats := analysis.SymbolStats[symbol]
						stats.TotalTrades++
						stats.TotalPnL += totalPnL
						if totalPnL > 0 {
							stats.WinningTrades++
						} else if totalPnL < 0 {
							stats.LosingTrades++
						}

						// Delete position record
						delete(openPositions, posKey)
					}
				}
			}
		}
	}

	// Calculate statistical indicators
	if analysis.TotalTrades > 0 {
		analysis.WinRate = (float64(analysis.WinningTrades) / float64(analysis.TotalTrades)) * 100

		// Calculate total profit and total loss
		totalWinAmount := analysis.AvgWin   // Currently accumulated sum
		totalLossAmount := analysis.AvgLoss // Currently accumulated sum (negative)

		if analysis.WinningTrades > 0 {
			analysis.AvgWin /= float64(analysis.WinningTrades)
		}
		if analysis.LosingTrades > 0 {
			analysis.AvgLoss /= float64(analysis.LosingTrades)
		}

		// Profit Factor = Total Profit / Total Loss (absolute value)
		// Note: totalLossAmount is negative, so take negative to get absolute value
		if totalLossAmount != 0 {
			analysis.ProfitFactor = totalWinAmount / (-totalLossAmount)
		} else if totalWinAmount > 0 {
			// Only profit no loss case, set to a large value to indicate perfect strategy
			analysis.ProfitFactor = 999.0
		}
	}

	// Calculate win rate and average PnL for each symbol
	bestPnL := -999999.0
	worstPnL := 999999.0
	for symbol, stats := range analysis.SymbolStats {
		if stats.TotalTrades > 0 {
			stats.WinRate = (float64(stats.WinningTrades) / float64(stats.TotalTrades)) * 100
			stats.AvgPnL = stats.TotalPnL / float64(stats.TotalTrades)

			if stats.TotalPnL > bestPnL {
				bestPnL = stats.TotalPnL
				analysis.BestSymbol = symbol
			}
			if stats.TotalPnL < worstPnL {
				worstPnL = stats.TotalPnL
				analysis.WorstSymbol = symbol
			}
		}
	}

	// Only keep recent trades (reverse order: newest first)
	if len(analysis.RecentTrades) > 10 {
		// Reverse array to put newest first
		for i, j := 0, len(analysis.RecentTrades)-1; i < j; i, j = i+1, j-1 {
			analysis.RecentTrades[i], analysis.RecentTrades[j] = analysis.RecentTrades[j], analysis.RecentTrades[i]
		}
		analysis.RecentTrades = analysis.RecentTrades[:10]
	} else if len(analysis.RecentTrades) > 0 {
		// Reverse array
		for i, j := 0, len(analysis.RecentTrades)-1; i < j; i, j = i+1, j-1 {
			analysis.RecentTrades[i], analysis.RecentTrades[j] = analysis.RecentTrades[j], analysis.RecentTrades[i]
		}
	}

	// Calculate Sharpe ratio (requires at least 2 data points)
	analysis.SharpeRatio = l.calculateSharpeRatio(records)

	return analysis, nil
}

// calculateSharpeRatio calculates Sharpe ratio
// Calculates risk-adjusted return based on account equity changes
func (l *DecisionLogger) calculateSharpeRatio(records []*DecisionRecord) float64 {
	if len(records) < 2 {
		return 0.0
	}

	// Extract account equity for each cycle
	// Note: TotalBalance field actually stores TotalEquity (total account equity)
	// TotalUnrealizedProfit field actually stores TotalPnL (profit/loss relative to initial balance)
	var equities []float64
	for _, record := range records {
		// Directly use TotalBalance as it is already the complete account equity
		equity := record.AccountState.TotalBalance
		if equity > 0 {
			equities = append(equities, equity)
		}
	}

	if len(equities) < 2 {
		return 0.0
	}

	// Calculate period returns
	var returns []float64
	for i := 1; i < len(equities); i++ {
		if equities[i-1] > 0 {
			periodReturn := (equities[i] - equities[i-1]) / equities[i-1]
			returns = append(returns, periodReturn)
		}
	}

	if len(returns) == 0 {
		return 0.0
	}

	// Calculate average return
	sumReturns := 0.0
	for _, r := range returns {
		sumReturns += r
	}
	meanReturn := sumReturns / float64(len(returns))

	// Calculate return standard deviation
	sumSquaredDiff := 0.0
	for _, r := range returns {
		diff := r - meanReturn
		sumSquaredDiff += diff * diff
	}
	variance := sumSquaredDiff / float64(len(returns))
	stdDev := math.Sqrt(variance)

	// Avoid division by zero
	if stdDev == 0 {
		if meanReturn > 0 {
			return 999.0 // Positive return with no volatility
		} else if meanReturn < 0 {
			return -999.0 // Negative return with no volatility
		}
		return 0.0
	}

	// Calculate Sharpe ratio (assuming risk-free rate is 0)
	// Note: directly returns cycle-level Sharpe ratio (not annualized), normal range -2 to +2
	sharpeRatio := meanReturn / stdDev
	return sharpeRatio
}
