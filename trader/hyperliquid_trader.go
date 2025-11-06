package trader

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sonirico/go-hyperliquid"
)

// HyperliquidTrader Hyperliquid trader
type HyperliquidTrader struct {
	exchange      *hyperliquid.Exchange
	ctx           context.Context
	walletAddr    string
	meta          *hyperliquid.Meta // Cached meta information (including precision, etc.)
	isCrossMargin bool              // Whether cross margin mode is enabled
}

// NewHyperliquidTrader creates a Hyperliquid trader
func NewHyperliquidTrader(privateKeyHex string, walletAddr string, testnet bool) (*HyperliquidTrader, error) {
	// Remove 0x prefix from private key (if present, case-insensitive)
	privateKeyHex = strings.TrimPrefix(strings.ToLower(privateKeyHex), "0x")

	// Parse private key
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Select API URL
	apiURL := hyperliquid.MainnetAPIURL
	if testnet {
		apiURL = hyperliquid.TestnetAPIURL
	}

	// Generate wallet address from private key (if not provided)
	if walletAddr == "" {
		pubKey := privateKey.Public()
		publicKeyECDSA, ok := pubKey.(*ecdsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("unable to convert public key")
		}
		walletAddr = crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
		log.Printf("✓ Auto-generated wallet address from private key: %s", walletAddr)
	} else {
		log.Printf("✓ Using provided wallet address: %s", walletAddr)
	}

	ctx := context.Background()

	// Create Exchange client (Exchange includes Info functionality)
	exchange := hyperliquid.NewExchange(
		ctx,
		privateKey,
		apiURL,
		nil,        // Meta will be fetched automatically
		"",         // vault address (empty for personal account)
		walletAddr, // wallet address
		nil,        // SpotMeta will be fetched automatically
	)

	log.Printf("✓ Hyperliquid trader initialized successfully (testnet=%v, wallet=%s)", testnet, walletAddr)

	// Get meta information (including precision and other configurations)
	meta, err := exchange.Info().Meta(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get meta information: %w", err)
	}

	return &HyperliquidTrader{
		exchange:      exchange,
		ctx:           ctx,
		walletAddr:    walletAddr,
		meta:          meta,
		isCrossMargin: true, // Default to cross margin mode
	}, nil
}

// GetBalance gets account balance
func (t *HyperliquidTrader) GetBalance() (map[string]interface{}, error) {
	log.Printf("🔄 Calling Hyperliquid API to get account balance...")

	// ✅ Step 1: Query Spot account balance
	spotState, err := t.exchange.Info().SpotUserState(t.ctx, t.walletAddr)
	var spotUSDCBalance float64 = 0.0
	if err != nil {
		log.Printf("⚠️ Failed to query Spot balance (may have no spot assets): %v", err)
	} else if spotState != nil && len(spotState.Balances) > 0 {
		for _, balance := range spotState.Balances {
			if balance.Coin == "USDC" {
				spotUSDCBalance, _ = strconv.ParseFloat(balance.Total, 64)
				log.Printf("✓ Found Spot balance: %.2f USDC", spotUSDCBalance)
				break
			}
		}
	}

	// ✅ Step 2: Query Perpetuals account state
	accountState, err := t.exchange.Info().UserState(t.ctx, t.walletAddr)
	if err != nil {
		log.Printf("❌ Hyperliquid Perpetuals API call failed: %v", err)
		return nil, fmt.Errorf("failed to get account information: %w", err)
	}

	// Parse balance information (MarginSummary fields are all strings)
	result := make(map[string]interface{})

	// ✅ Step 3: Dynamically select correct summary based on margin mode (CrossMarginSummary or MarginSummary)
	var accountValue, totalMarginUsed float64
	var summaryType string
	var summary interface{}

	if t.isCrossMargin {
		// Cross margin mode: use CrossMarginSummary
		accountValue, _ = strconv.ParseFloat(accountState.CrossMarginSummary.AccountValue, 64)
		totalMarginUsed, _ = strconv.ParseFloat(accountState.CrossMarginSummary.TotalMarginUsed, 64)
		summaryType = "CrossMarginSummary (Cross)"
		summary = accountState.CrossMarginSummary
	} else {
		// Isolated margin mode: use MarginSummary
		accountValue, _ = strconv.ParseFloat(accountState.MarginSummary.AccountValue, 64)
		totalMarginUsed, _ = strconv.ParseFloat(accountState.MarginSummary.TotalMarginUsed, 64)
		summaryType = "MarginSummary (Isolated)"
		summary = accountState.MarginSummary
	}

	// 🔍 Debug: print complete summary structure returned by API
	summaryJSON, _ := json.MarshalIndent(summary, "  ", "  ")
	log.Printf("🔍 [DEBUG] Hyperliquid API %s complete data:", summaryType)
	log.Printf("%s", string(summaryJSON))

	// ⚠️ Critical fix: accumulate true unrealized PnL from all positions
	totalUnrealizedPnl := 0.0
	for _, assetPos := range accountState.AssetPositions {
		unrealizedPnl, _ := strconv.ParseFloat(assetPos.Position.UnrealizedPnl, 64)
		totalUnrealizedPnl += unrealizedPnl
	}

	// ✅ Correct understanding of Hyperliquid fields:
	// AccountValue = Total account equity (includes idle funds + position value + unrealized PnL)
	// TotalMarginUsed = Margin used by positions (already included in AccountValue, for display only)
	//
	// To be compatible with auto_trader.go calculation logic (totalEquity = totalWalletBalance + totalUnrealizedProfit)
	// Need to return "wallet balance without unrealized PnL"
	walletBalanceWithoutUnrealized := accountValue - totalUnrealizedPnl

	// ✅ Step 4: Use Withdrawable field (PR #443)
	// Withdrawable is the official real withdrawable balance, more reliable than simple calculation
	availableBalance := 0.0
	if accountState.Withdrawable != "" {
		withdrawable, err := strconv.ParseFloat(accountState.Withdrawable, 64)
		if err == nil && withdrawable > 0 {
			availableBalance = withdrawable
			log.Printf("✓ Using Withdrawable as available balance: %.2f", availableBalance)
		}
	}

	// Fallback: if no Withdrawable, use simple calculation
	if availableBalance == 0 && accountState.Withdrawable == "" {
		availableBalance = accountValue - totalMarginUsed
		if availableBalance < 0 {
			log.Printf("⚠️ Calculated available balance is negative (%.2f), resetting to 0", availableBalance)
			availableBalance = 0
		}
	}

	// ✅ Step 5: Correctly handle Spot + Perpetuals balance
	// Important: Spot only adds to total assets, not to available balance
	//       Reason: Spot and Perpetuals are separate accounts, require manual ClassTransfer to transfer
	totalWalletBalance := walletBalanceWithoutUnrealized + spotUSDCBalance

	result["totalWalletBalance"] = totalWalletBalance    // Total assets (Perp + Spot)
	result["availableBalance"] = availableBalance        // Available balance (Perpetuals only, excluding Spot)
	result["totalUnrealizedProfit"] = totalUnrealizedPnl // Unrealized PnL (from Perpetuals only)
	result["spotBalance"] = spotUSDCBalance              // Spot balance (returned separately)

	log.Printf("✓ Hyperliquid complete account:")
	log.Printf("  • Spot balance: %.2f USDC (need manual transfer to Perpetuals to open positions)", spotUSDCBalance)
	log.Printf("  • Perpetuals contract equity: %.2f USDC (wallet %.2f + unrealized %.2f)",
		accountValue,
		walletBalanceWithoutUnrealized,
		totalUnrealizedPnl)
	log.Printf("  • Perpetuals available balance: %.2f USDC (can be used directly for opening positions)", availableBalance)
	log.Printf("  • Margin used: %.2f USDC", totalMarginUsed)
	log.Printf("  • Total assets (Perp+Spot): %.2f USDC", totalWalletBalance)
	log.Printf("  ⭐ Total assets: %.2f USDC | Perp available: %.2f USDC | Spot balance: %.2f USDC",
		totalWalletBalance, availableBalance, spotUSDCBalance)

	return result, nil
}

// GetPositions gets all positions
func (t *HyperliquidTrader) GetPositions() ([]map[string]interface{}, error) {
	// Get account state
	accountState, err := t.exchange.Info().UserState(t.ctx, t.walletAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var result []map[string]interface{}

	// Iterate through all positions
	for _, assetPos := range accountState.AssetPositions {
		position := assetPos.Position

		// Position amount (string type)
		posAmt, _ := strconv.ParseFloat(position.Szi, 64)

		if posAmt == 0 {
			continue // Skip positions with no amount
		}

		posMap := make(map[string]interface{})

		// Standardize symbol format (Hyperliquid uses "BTC", we convert to "BTCUSDT")
		symbol := position.Coin + "USDT"
		posMap["symbol"] = symbol

		// Position amount and direction
		if posAmt > 0 {
			posMap["side"] = "long"
			posMap["positionAmt"] = posAmt
		} else {
			posMap["side"] = "short"
			posMap["positionAmt"] = -posAmt // Convert to positive
		}

		// Price information (EntryPx and LiquidationPx are pointer types)
		var entryPrice, liquidationPx float64
		if position.EntryPx != nil {
			entryPrice, _ = strconv.ParseFloat(*position.EntryPx, 64)
		}
		if position.LiquidationPx != nil {
			liquidationPx, _ = strconv.ParseFloat(*position.LiquidationPx, 64)
		}

		positionValue, _ := strconv.ParseFloat(position.PositionValue, 64)
		unrealizedPnl, _ := strconv.ParseFloat(position.UnrealizedPnl, 64)

		// Calculate mark price (positionValue / abs(posAmt))
		var markPrice float64
		if posAmt != 0 {
			markPrice = positionValue / absFloat(posAmt)
		}

		posMap["entryPrice"] = entryPrice
		posMap["markPrice"] = markPrice
		posMap["unRealizedProfit"] = unrealizedPnl
		posMap["leverage"] = float64(position.Leverage.Value)
		posMap["liquidationPrice"] = liquidationPx

		result = append(result, posMap)
	}

	return result, nil
}

// SetMarginMode sets margin mode (set together with SetLeverage)
func (t *HyperliquidTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	// Hyperliquid margin mode is set in SetLeverage, here we only record it
	t.isCrossMargin = isCrossMargin
	marginModeStr := "Cross"
	if !isCrossMargin {
		marginModeStr = "Isolated"
	}
	log.Printf("  ✓ %s will use %s mode", symbol, marginModeStr)
	return nil
}

// SetLeverage sets leverage
func (t *HyperliquidTrader) SetLeverage(symbol string, leverage int) error {
	// Hyperliquid symbol format (remove USDT suffix)
	coin := convertSymbolToHyperliquid(symbol)

	// Call UpdateLeverage (leverage int, name string, isCross bool)
	// Third parameter: true=cross margin mode, false=isolated margin mode
	_, err := t.exchange.UpdateLeverage(t.ctx, leverage, coin, t.isCrossMargin)
	if err != nil {
		return fmt.Errorf("failed to set leverage: %w", err)
	}

	log.Printf("  ✓ %s leverage switched to %dx", symbol, leverage)
	return nil
}

// OpenLong opens a long position
func (t *HyperliquidTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// Cancel all orders for this symbol first
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ Failed to cancel old orders: %v", err)
	}

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// Hyperliquid symbol format
	coin := convertSymbolToHyperliquid(symbol)

	// Get current price (for market order)
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// ⚠️ Critical: round quantity according to coin precision requirements
	roundedQuantity := t.roundToSzDecimals(coin, quantity)
	log.Printf("  📏 Quantity precision handling: %.8f -> %.8f (szDecimals=%d)", quantity, roundedQuantity, t.getSzDecimals(coin))

	// ⚠️ Critical: price also needs to be processed to 5 significant figures
	aggressivePrice := t.roundPriceToSigfigs(price * 1.01)
	log.Printf("  💰 Price precision handling: %.8f -> %.8f (5 significant figures)", price*1.01, aggressivePrice)

	// Create market buy order (using IOC limit order with aggressive price)
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: true,
		Size:  roundedQuantity, // Use rounded quantity
		Price: aggressivePrice, // Use processed price
		OrderType: hyperliquid.OrderType{
			Limit: &hyperliquid.LimitOrderType{
				Tif: hyperliquid.TifIoc, // Immediate or Cancel (similar to market order)
			},
		},
		ReduceOnly: false,
	}

	_, err = t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open long position: %w", err)
	}

	log.Printf("✓ Successfully opened long position: %s quantity: %.4f", symbol, roundedQuantity)

	result := make(map[string]interface{})
	result["orderId"] = 0 // Hyperliquid doesn't return order ID
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// OpenShort opens a short position
func (t *HyperliquidTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// Cancel all orders for this symbol first
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ Failed to cancel old orders: %v", err)
	}

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// Hyperliquid symbol format
	coin := convertSymbolToHyperliquid(symbol)

	// Get current price
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// ⚠️ Critical: round quantity according to coin precision requirements
	roundedQuantity := t.roundToSzDecimals(coin, quantity)
	log.Printf("  📏 Quantity precision handling: %.8f -> %.8f (szDecimals=%d)", quantity, roundedQuantity, t.getSzDecimals(coin))

	// ⚠️ Critical: price also needs to be processed to 5 significant figures
	aggressivePrice := t.roundPriceToSigfigs(price * 0.99)
	log.Printf("  💰 Price precision handling: %.8f -> %.8f (5 significant figures)", price*0.99, aggressivePrice)

	// Create market sell order
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: false,
		Size:  roundedQuantity, // Use rounded quantity
		Price: aggressivePrice, // Use processed price
		OrderType: hyperliquid.OrderType{
			Limit: &hyperliquid.LimitOrderType{
				Tif: hyperliquid.TifIoc,
			},
		},
		ReduceOnly: false,
	}

	_, err = t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open short position: %w", err)
	}

	log.Printf("✓ Successfully opened short position: %s quantity: %.4f", symbol, roundedQuantity)

	result := make(map[string]interface{})
	result["orderId"] = 0
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// CloseLong closes a long position
func (t *HyperliquidTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	// If quantity is 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "long" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("no long position found for %s", symbol)
		}
	}

	// Hyperliquid symbol format
	coin := convertSymbolToHyperliquid(symbol)

	// Get current price
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// ⚠️ Critical: round quantity according to coin precision requirements
	roundedQuantity := t.roundToSzDecimals(coin, quantity)
	log.Printf("  📏 Quantity precision handling: %.8f -> %.8f (szDecimals=%d)", quantity, roundedQuantity, t.getSzDecimals(coin))

	// ⚠️ Critical: price also needs to be processed to 5 significant figures
	aggressivePrice := t.roundPriceToSigfigs(price * 0.99)
	log.Printf("  💰 Price precision handling: %.8f -> %.8f (5 significant figures)", price*0.99, aggressivePrice)

	// Create close position order (sell + ReduceOnly)
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: false,
		Size:  roundedQuantity, // Use rounded quantity
		Price: aggressivePrice, // Use processed price
		OrderType: hyperliquid.OrderType{
			Limit: &hyperliquid.LimitOrderType{
				Tif: hyperliquid.TifIoc,
			},
		},
		ReduceOnly: true, // Only close position, don't open new position
	}

	_, err = t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to close long position: %w", err)
	}

	log.Printf("✓ Successfully closed long position: %s quantity: %.4f", symbol, roundedQuantity)

	// Cancel all pending orders for this symbol after closing
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ Failed to cancel pending orders: %v", err)
	}

	result := make(map[string]interface{})
	result["orderId"] = 0
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// CloseShort closes a short position
func (t *HyperliquidTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// If quantity is 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "short" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("no short position found for %s", symbol)
		}
	}

	// Hyperliquid symbol format
	coin := convertSymbolToHyperliquid(symbol)

	// Get current price
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// ⚠️ Critical: round quantity according to coin precision requirements
	roundedQuantity := t.roundToSzDecimals(coin, quantity)
	log.Printf("  📏 Quantity precision handling: %.8f -> %.8f (szDecimals=%d)", quantity, roundedQuantity, t.getSzDecimals(coin))

	// ⚠️ Critical: price also needs to be processed to 5 significant figures
	aggressivePrice := t.roundPriceToSigfigs(price * 1.01)
	log.Printf("  💰 Price precision handling: %.8f -> %.8f (5 significant figures)", price*1.01, aggressivePrice)

	// Create close position order (buy + ReduceOnly)
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: true,
		Size:  roundedQuantity, // Use rounded quantity
		Price: aggressivePrice, // Use processed price
		OrderType: hyperliquid.OrderType{
			Limit: &hyperliquid.LimitOrderType{
				Tif: hyperliquid.TifIoc,
			},
		},
		ReduceOnly: true,
	}

	_, err = t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to close short position: %w", err)
	}

	log.Printf("✓ Successfully closed short position: %s quantity: %.4f", symbol, roundedQuantity)

	// Cancel all pending orders for this symbol after closing
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ Failed to cancel pending orders: %v", err)
	}

	result := make(map[string]interface{})
	result["orderId"] = 0
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// CancelStopLossOrders cancels stop loss orders only (Hyperliquid cannot distinguish stop loss from take profit, cancels all)
func (t *HyperliquidTrader) CancelStopLossOrders(symbol string) error {
	// Hyperliquid SDK's OpenOrder structure doesn't expose trigger field
	// Cannot distinguish stop loss from take profit orders, so cancel all pending orders for this symbol
	log.Printf("  ⚠️ Hyperliquid cannot distinguish stop loss/take profit orders, will cancel all pending orders")
	return t.CancelStopOrders(symbol)
}

// CancelTakeProfitOrders cancels take profit orders only (Hyperliquid cannot distinguish stop loss from take profit, cancels all)
func (t *HyperliquidTrader) CancelTakeProfitOrders(symbol string) error {
	// Hyperliquid SDK's OpenOrder structure doesn't expose trigger field
	// Cannot distinguish stop loss from take profit orders, so cancel all pending orders for this symbol
	log.Printf("  ⚠️ Hyperliquid cannot distinguish stop loss/take profit orders, will cancel all pending orders")
	return t.CancelStopOrders(symbol)
}

// CancelAllOrders cancels all pending orders for this symbol
func (t *HyperliquidTrader) CancelAllOrders(symbol string) error {
	coin := convertSymbolToHyperliquid(symbol)

	// Get all pending orders
	openOrders, err := t.exchange.Info().OpenOrders(t.ctx, t.walletAddr)
	if err != nil {
		return fmt.Errorf("failed to get pending orders: %w", err)
	}

	// Cancel all pending orders for this symbol
	for _, order := range openOrders {
		if order.Coin == coin {
			_, err := t.exchange.Cancel(t.ctx, coin, order.Oid)
			if err != nil {
				log.Printf("  ⚠ Failed to cancel order (oid=%d): %v", order.Oid, err)
			}
		}
	}

	log.Printf("  ✓ Cancelled all pending orders for %s", symbol)
	return nil
}

// CancelStopOrders cancels stop loss/take profit orders for this symbol (used when adjusting stop loss/take profit levels)
func (t *HyperliquidTrader) CancelStopOrders(symbol string) error {
	coin := convertSymbolToHyperliquid(symbol)

	// Get all pending orders
	openOrders, err := t.exchange.Info().OpenOrders(t.ctx, t.walletAddr)
	if err != nil {
		return fmt.Errorf("failed to get pending orders: %w", err)
	}

	// Note: Hyperliquid SDK's OpenOrder structure doesn't expose trigger field
	// Therefore, temporarily cancel all pending orders for this symbol (including stop loss/take profit orders)
	// This is safe because we should clean up all old orders before setting new stop loss/take profit
	canceledCount := 0
	for _, order := range openOrders {
		if order.Coin == coin {
			_, err := t.exchange.Cancel(t.ctx, coin, order.Oid)
			if err != nil {
				log.Printf("  ⚠ Failed to cancel order (oid=%d): %v", order.Oid, err)
				continue
			}
			canceledCount++
		}
	}

	if canceledCount == 0 {
		log.Printf("  ℹ %s has no pending orders to cancel", symbol)
	} else {
		log.Printf("  ✓ Cancelled %d pending orders for %s (including stop loss/take profit orders)", canceledCount, symbol)
	}

	return nil
}

// GetMarketPrice gets market price
func (t *HyperliquidTrader) GetMarketPrice(symbol string) (float64, error) {
	coin := convertSymbolToHyperliquid(symbol)

	// Get all market prices
	allMids, err := t.exchange.Info().AllMids(t.ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get price: %w", err)
	}

	// Find price for corresponding coin (allMids is map[string]string)
	if priceStr, ok := allMids[coin]; ok {
		priceFloat, err := strconv.ParseFloat(priceStr, 64)
		if err == nil {
			return priceFloat, nil
		}
		return 0, fmt.Errorf("invalid price format: %v", err)
	}

	return 0, fmt.Errorf("price not found for %s", symbol)
}

// SetStopLoss sets stop loss order
func (t *HyperliquidTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	coin := convertSymbolToHyperliquid(symbol)

	isBuy := positionSide == "SHORT" // Short position stop loss = buy, long position stop loss = sell

	// ⚠️ Critical: round quantity according to coin precision requirements
	roundedQuantity := t.roundToSzDecimals(coin, quantity)

	// ⚠️ Critical: price also needs to be processed to 5 significant figures
	roundedStopPrice := t.roundPriceToSigfigs(stopPrice)

	// Create stop loss order (Trigger Order)
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: isBuy,
		Size:  roundedQuantity,  // Use rounded quantity
		Price: roundedStopPrice, // Use processed price
		OrderType: hyperliquid.OrderType{
			Trigger: &hyperliquid.TriggerOrderType{
				TriggerPx: roundedStopPrice,
				IsMarket:  true,
				Tpsl:      "sl", // stop loss
			},
		},
		ReduceOnly: true,
	}

	_, err := t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return fmt.Errorf("failed to set stop loss: %w", err)
	}

	log.Printf("  Stop loss price set: %.4f", roundedStopPrice)
	return nil
}

// SetTakeProfit sets take profit order
func (t *HyperliquidTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	coin := convertSymbolToHyperliquid(symbol)

	isBuy := positionSide == "SHORT" // Short position take profit = buy, long position take profit = sell

	// ⚠️ Critical: round quantity according to coin precision requirements
	roundedQuantity := t.roundToSzDecimals(coin, quantity)

	// ⚠️ Critical: price also needs to be processed to 5 significant figures
	roundedTakeProfitPrice := t.roundPriceToSigfigs(takeProfitPrice)

	// Create take profit order (Trigger Order)
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: isBuy,
		Size:  roundedQuantity,        // Use rounded quantity
		Price: roundedTakeProfitPrice, // Use processed price
		OrderType: hyperliquid.OrderType{
			Trigger: &hyperliquid.TriggerOrderType{
				TriggerPx: roundedTakeProfitPrice,
				IsMarket:  true,
				Tpsl:      "tp", // take profit
			},
		},
		ReduceOnly: true,
	}

	_, err := t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return fmt.Errorf("failed to set take profit: %w", err)
	}

	log.Printf("  Take profit price set: %.4f", roundedTakeProfitPrice)
	return nil
}

// FormatQuantity formats quantity to correct precision
func (t *HyperliquidTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	coin := convertSymbolToHyperliquid(symbol)
	szDecimals := t.getSzDecimals(coin)

	// Format quantity using szDecimals
	formatStr := fmt.Sprintf("%%.%df", szDecimals)
	return fmt.Sprintf(formatStr, quantity), nil
}

// getSzDecimals gets the quantity precision for a coin
func (t *HyperliquidTrader) getSzDecimals(coin string) int {
	if t.meta == nil {
		log.Printf("⚠️  Meta information is empty, using default precision 4")
		return 4 // Default precision
	}

	// Find corresponding coin in meta.Universe
	for _, asset := range t.meta.Universe {
		if asset.Name == coin {
			return asset.SzDecimals
		}
	}

	log.Printf("⚠️  Precision information not found for %s, using default precision 4", coin)
	return 4 // Default precision
}

// roundToSzDecimals rounds quantity to correct precision
func (t *HyperliquidTrader) roundToSzDecimals(coin string, quantity float64) float64 {
	szDecimals := t.getSzDecimals(coin)

	// Calculate multiplier (10^szDecimals)
	multiplier := 1.0
	for i := 0; i < szDecimals; i++ {
		multiplier *= 10.0
	}

	// Round
	return float64(int(quantity*multiplier+0.5)) / multiplier
}

// roundPriceToSigfigs rounds price to 5 significant figures
// Hyperliquid requires prices to use 5 significant figures
func (t *HyperliquidTrader) roundPriceToSigfigs(price float64) float64 {
	if price == 0 {
		return 0
	}

	const sigfigs = 5 // Hyperliquid standard: 5 significant figures

	// Calculate price magnitude
	var magnitude float64
	if price < 0 {
		magnitude = -price
	} else {
		magnitude = price
	}

	// Calculate required multiplier
	multiplier := 1.0
	for magnitude >= 10 {
		magnitude /= 10
		multiplier /= 10
	}
	for magnitude < 1 {
		magnitude *= 10
		multiplier *= 10
	}

	// Apply significant figures precision
	for i := 0; i < sigfigs-1; i++ {
		multiplier *= 10
	}

	// Round
	rounded := float64(int(price*multiplier+0.5)) / multiplier
	return rounded
}

// convertSymbolToHyperliquid converts standard symbol to Hyperliquid format
// Example: "BTCUSDT" -> "BTC"
func convertSymbolToHyperliquid(symbol string) string {
	// Remove USDT suffix
	if len(symbol) > 4 && symbol[len(symbol)-4:] == "USDT" {
		return symbol[:len(symbol)-4]
	}
	return symbol
}

// absFloat returns the absolute value of a float
func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
