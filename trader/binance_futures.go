package trader

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)

// FuturesTrader Binance futures trader
type FuturesTrader struct {
	client *futures.Client

	// Balance cache
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// Position cache
	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex

	// Cache validity duration (15 seconds)
	cacheDuration time.Duration
}

// NewFuturesTrader Create futures trader
func NewFuturesTrader(apiKey, secretKey string) *FuturesTrader {
	client := futures.NewClient(apiKey, secretKey)
	// Sync time to avoid "Timestamp ahead" error
	syncBinanceServerTime(client)
	trader := &FuturesTrader{
		client:        client,
		cacheDuration: 15 * time.Second, // 15 second cache
	}

	// Set dual side position mode (Hedge Mode)
	// This is required because the code uses PositionSide (LONG/SHORT)
	if err := trader.setDualSidePosition(); err != nil {
		log.Printf("⚠️ Failed to set dual side position mode: %v (ignore this warning if already in dual mode)", err)
	}

	return trader
}

// setDualSidePosition Set dual side position mode (called during initialization)
func (t *FuturesTrader) setDualSidePosition() error {
	// Try to set dual side position mode
	err := t.client.NewChangePositionModeService().
		DualSide(true). // true = dual side position (Hedge Mode)
		Do(context.Background())

	if err != nil {
		// If error message contains "No need to change", it means already in dual side position mode
		if strings.Contains(err.Error(), "No need to change position side") {
			log.Printf("  ✓ Account is already in dual side position mode (Hedge Mode)")
			return nil
		}
		// Other errors are returned (but won't interrupt initialization at caller)
		return err
	}

	log.Printf("  ✓ Account switched to dual side position mode (Hedge Mode)")
	log.Printf("  ℹ️  Dual side position mode allows holding both long and short positions simultaneously")
	return nil
}

// syncBinanceServerTime Sync Binance server time to ensure request timestamp is valid
func syncBinanceServerTime(client *futures.Client) {
	serverTime, err := client.NewServerTimeService().Do(context.Background())
	if err != nil {
		log.Printf("⚠️ Failed to sync Binance server time: %v", err)
		return
	}

	now := time.Now().UnixMilli()
	offset := now - serverTime
	client.TimeOffset = offset
	log.Printf("⏱ Binance server time synced, offset %dms", offset)
}

// GetBalance Get account balance (with cache)
func (t *FuturesTrader) GetBalance() (map[string]interface{}, error) {
	// First check if cache is valid
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		cacheAge := time.Since(t.balanceCacheTime)
		t.balanceCacheMutex.RUnlock()
		log.Printf("✓ Using cached account balance (cached %.1f seconds ago)", cacheAge.Seconds())
		return t.cachedBalance, nil
	}
	t.balanceCacheMutex.RUnlock()

	// Cache expired or doesn't exist, call API
	log.Printf("🔄 Cache expired, calling Binance API to get account balance...")
	account, err := t.client.NewGetAccountService().Do(context.Background())
	if err != nil {
		log.Printf("❌ Binance API call failed: %v", err)
		return nil, fmt.Errorf("failed to get account information: %w", err)
	}

	result := make(map[string]interface{})
	result["totalWalletBalance"], _ = strconv.ParseFloat(account.TotalWalletBalance, 64)
	result["availableBalance"], _ = strconv.ParseFloat(account.AvailableBalance, 64)
	result["totalUnrealizedProfit"], _ = strconv.ParseFloat(account.TotalUnrealizedProfit, 64)

	log.Printf("✓ Binance API returned: Total balance=%s, Available=%s, Unrealized PnL=%s",
		account.TotalWalletBalance,
		account.AvailableBalance,
		account.TotalUnrealizedProfit)

	// Update cache
	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return result, nil
}

// GetPositions Get all positions (with cache)
func (t *FuturesTrader) GetPositions() ([]map[string]interface{}, error) {
	// First check if cache is valid
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		cacheAge := time.Since(t.positionsCacheTime)
		t.positionsCacheMutex.RUnlock()
		log.Printf("✓ Using cached position information (cached %.1f seconds ago)", cacheAge.Seconds())
		return t.cachedPositions, nil
	}
	t.positionsCacheMutex.RUnlock()

	// Cache expired or doesn't exist, call API
	log.Printf("🔄 Cache expired, calling Binance API to get position information...")
	positions, err := t.client.NewGetPositionRiskService().Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		posAmt, _ := strconv.ParseFloat(pos.PositionAmt, 64)
		if posAmt == 0 {
			continue // Skip positions with no amount
		}

		posMap := make(map[string]interface{})
		posMap["symbol"] = pos.Symbol
		posMap["positionAmt"], _ = strconv.ParseFloat(pos.PositionAmt, 64)
		posMap["entryPrice"], _ = strconv.ParseFloat(pos.EntryPrice, 64)
		posMap["markPrice"], _ = strconv.ParseFloat(pos.MarkPrice, 64)
		posMap["unRealizedProfit"], _ = strconv.ParseFloat(pos.UnRealizedProfit, 64)
		posMap["leverage"], _ = strconv.ParseFloat(pos.Leverage, 64)
		posMap["liquidationPrice"], _ = strconv.ParseFloat(pos.LiquidationPrice, 64)

		// Determine direction
		if posAmt > 0 {
			posMap["side"] = "long"
		} else {
			posMap["side"] = "short"
		}

		result = append(result, posMap)
	}

	// Update cache
	t.positionsCacheMutex.Lock()
	t.cachedPositions = result
	t.positionsCacheTime = time.Now()
	t.positionsCacheMutex.Unlock()

	return result, nil
}

// SetMarginMode Set margin mode
func (t *FuturesTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	var marginType futures.MarginType
	if isCrossMargin {
		marginType = futures.MarginTypeCrossed
	} else {
		marginType = futures.MarginTypeIsolated
	}

	// Try to set margin mode
	err := t.client.NewChangeMarginTypeService().
		Symbol(symbol).
		MarginType(marginType).
		Do(context.Background())

	marginModeStr := "Cross Margin"
	if !isCrossMargin {
		marginModeStr = "Isolated Margin"
	}

	if err != nil {
		// If error message contains "No need to change", it means margin mode is already the target value
		if contains(err.Error(), "No need to change margin type") {
			log.Printf("  ✓ %s margin mode is already %s", symbol, marginModeStr)
			return nil
		}
		// If there are positions, cannot change margin mode, but doesn't affect trading
		if contains(err.Error(), "Margin type cannot be changed if there exists position") {
			log.Printf("  ⚠️ %s has positions, cannot change margin mode, continuing with current mode", symbol)
			return nil
		}
		// Detect multi-asset mode (error code -4168)
		if contains(err.Error(), "Multi-Assets mode") || contains(err.Error(), "-4168") || contains(err.Error(), "4168") {
			log.Printf("  ⚠️ %s detected multi-asset mode, forcing cross margin mode", symbol)
			log.Printf("  💡 Tip: To use isolated margin mode, please disable multi-asset mode on Binance")
			return nil
		}
		// Detect unified account API (Portfolio Margin)
		if contains(err.Error(), "unified") || contains(err.Error(), "portfolio") || contains(err.Error(), "Portfolio") {
			log.Printf("  ❌ %s detected unified account API, cannot perform futures trading", symbol)
			return fmt.Errorf("please use 'Spot & Futures Trading' API permissions, do not use 'Unified Account API'")
		}
		log.Printf("  ⚠️ Failed to set margin mode: %v", err)
		// Don't return error, let trading continue
		return nil
	}

	log.Printf("  ✓ %s margin mode set to %s", symbol, marginModeStr)
	return nil
}

// SetLeverage Set leverage (smart detection + cooldown period)
func (t *FuturesTrader) SetLeverage(symbol string, leverage int) error {
	// First try to get current leverage (from position information)
	currentLeverage := 0
	positions, err := t.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == symbol {
				if lev, ok := pos["leverage"].(float64); ok {
					currentLeverage = int(lev)
					break
				}
			}
		}
	}

	// If current leverage is already the target leverage, skip
	if currentLeverage == leverage && currentLeverage > 0 {
		log.Printf("  ✓ %s leverage is already %dx, no need to switch", symbol, leverage)
		return nil
	}

	// Switch leverage
	_, err = t.client.NewChangeLeverageService().
		Symbol(symbol).
		Leverage(leverage).
		Do(context.Background())

	if err != nil {
		// If error message contains "No need to change", it means leverage is already the target value
		if contains(err.Error(), "No need to change") {
			log.Printf("  ✓ %s leverage is already %dx", symbol, leverage)
			return nil
		}
		return fmt.Errorf("failed to set leverage: %w", err)
	}

	log.Printf("  ✓ %s leverage switched to %dx", symbol, leverage)

	// Wait 5 seconds after switching leverage (avoid cooldown error)
	log.Printf("  ⏱ Waiting 5 seconds cooldown period...")
	time.Sleep(5 * time.Second)

	return nil
}

// OpenLong Open long position
func (t *FuturesTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// First cancel all pending orders for this symbol (clean up old stop loss/take profit orders)
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ Failed to cancel old pending orders (may have no orders): %v", err)
	}

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// Note: Margin mode should be set by caller (AutoTrader) before opening position via SetMarginMode

	// Format quantity to correct precision
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// ✅ Check if formatted quantity is 0 (prevent errors from rounding)
	quantityFloat, parseErr := strconv.ParseFloat(quantityStr, 64)
	if parseErr != nil || quantityFloat <= 0 {
		return nil, fmt.Errorf("opening quantity too small, formatted to 0 (original: %.8f → formatted: %s). Suggest increasing opening amount or choosing a coin with lower price", quantity, quantityStr)
	}

	// ✅ Check minimum notional value (Binance requires at least 10 USDT)
	if err := t.CheckMinNotional(symbol, quantityFloat); err != nil {
		return nil, err
	}

	// Create market buy order
	order, err := t.client.NewCreateOrderService().
		Symbol(symbol).
		Side(futures.SideTypeBuy).
		PositionSide(futures.PositionSideTypeLong).
		Type(futures.OrderTypeMarket).
		Quantity(quantityStr).
		Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("failed to open long position: %w", err)
	}

	log.Printf("✓ Long position opened successfully: %s quantity: %s", symbol, quantityStr)
	log.Printf("  Order ID: %d", order.OrderID)

	result := make(map[string]interface{})
	result["orderId"] = order.OrderID
	result["symbol"] = order.Symbol
	result["status"] = order.Status
	return result, nil
}

// OpenShort Open short position
func (t *FuturesTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// First cancel all pending orders for this symbol (clean up old stop loss/take profit orders)
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ Failed to cancel old pending orders (may have no orders): %v", err)
	}

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// Note: Margin mode should be set by caller (AutoTrader) before opening position via SetMarginMode

	// Format quantity to correct precision
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// ✅ Check if formatted quantity is 0 (prevent errors from rounding)
	quantityFloat, parseErr := strconv.ParseFloat(quantityStr, 64)
	if parseErr != nil || quantityFloat <= 0 {
		return nil, fmt.Errorf("opening quantity too small, formatted to 0 (original: %.8f → formatted: %s). Suggest increasing opening amount or choosing a coin with lower price", quantity, quantityStr)
	}

	// ✅ Check minimum notional value (Binance requires at least 10 USDT)
	if err := t.CheckMinNotional(symbol, quantityFloat); err != nil {
		return nil, err
	}

	// Create market sell order
	order, err := t.client.NewCreateOrderService().
		Symbol(symbol).
		Side(futures.SideTypeSell).
		PositionSide(futures.PositionSideTypeShort).
		Type(futures.OrderTypeMarket).
		Quantity(quantityStr).
		Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("failed to open short position: %w", err)
	}

	log.Printf("✓ Short position opened successfully: %s quantity: %s", symbol, quantityStr)
	log.Printf("  Order ID: %d", order.OrderID)

	result := make(map[string]interface{})
	result["orderId"] = order.OrderID
	result["symbol"] = order.Symbol
	result["status"] = order.Status
	return result, nil
}

// CloseLong Close long position
func (t *FuturesTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
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
			return nil, fmt.Errorf("long position not found for %s", symbol)
		}
	}

	// Format quantity
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Create market sell order (close long)
	order, err := t.client.NewCreateOrderService().
		Symbol(symbol).
		Side(futures.SideTypeSell).
		PositionSide(futures.PositionSideTypeLong).
		Type(futures.OrderTypeMarket).
		Quantity(quantityStr).
		Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("failed to close long position: %w", err)
	}

	log.Printf("✓ Long position closed successfully: %s quantity: %s", symbol, quantityStr)

	// Cancel all pending orders for this symbol after closing (stop loss/take profit orders)
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ Failed to cancel pending orders: %v", err)
	}

	result := make(map[string]interface{})
	result["orderId"] = order.OrderID
	result["symbol"] = order.Symbol
	result["status"] = order.Status
	return result, nil
}

// CloseShort Close short position
func (t *FuturesTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// If quantity is 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "short" {
				quantity = -pos["positionAmt"].(float64) // Short position quantity is negative, take absolute value
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("short position not found for %s", symbol)
		}
	}

	// Format quantity
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Create market buy order (close short)
	order, err := t.client.NewCreateOrderService().
		Symbol(symbol).
		Side(futures.SideTypeBuy).
		PositionSide(futures.PositionSideTypeShort).
		Type(futures.OrderTypeMarket).
		Quantity(quantityStr).
		Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("failed to close short position: %w", err)
	}

	log.Printf("✓ Short position closed successfully: %s quantity: %s", symbol, quantityStr)

	// Cancel all pending orders for this symbol after closing (stop loss/take profit orders)
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ Failed to cancel pending orders: %v", err)
	}

	result := make(map[string]interface{})
	result["orderId"] = order.OrderID
	result["symbol"] = order.Symbol
	result["status"] = order.Status
	return result, nil
}

// CancelStopLossOrders Cancel stop loss orders only (does not affect take profit orders)
func (t *FuturesTrader) CancelStopLossOrders(symbol string) error {
	// Get all open orders for this symbol
	orders, err := t.client.NewListOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err != nil {
		return fmt.Errorf("failed to get open orders: %w", err)
	}

	// Filter out stop loss orders and cancel them
	canceledCount := 0
	for _, order := range orders {
		orderType := order.Type

		// Only cancel stop loss orders (don't cancel take profit orders)
		if orderType == futures.OrderTypeStopMarket || orderType == futures.OrderTypeStop {
			_, err := t.client.NewCancelOrderService().
				Symbol(symbol).
				OrderID(order.OrderID).
				Do(context.Background())

			if err != nil {
				log.Printf("  ⚠ Failed to cancel stop loss order %d: %v", order.OrderID, err)
				continue
			}

			canceledCount++
			log.Printf("  ✓ Canceled stop loss order (Order ID: %d, Type: %s)", order.OrderID, orderType)
		}
	}

	if canceledCount == 0 {
		log.Printf("  ℹ %s has no stop loss orders to cancel", symbol)
	} else {
		log.Printf("  ✓ Canceled %d stop loss orders for %s", canceledCount, symbol)
	}

	return nil
}

// CancelTakeProfitOrders Cancel take profit orders only (does not affect stop loss orders)
func (t *FuturesTrader) CancelTakeProfitOrders(symbol string) error {
	// Get all open orders for this symbol
	orders, err := t.client.NewListOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err != nil {
		return fmt.Errorf("failed to get open orders: %w", err)
	}

	// Filter out take profit orders and cancel them
	canceledCount := 0
	for _, order := range orders {
		orderType := order.Type

		// Only cancel take profit orders (don't cancel stop loss orders)
		if orderType == futures.OrderTypeTakeProfitMarket || orderType == futures.OrderTypeTakeProfit {
			_, err := t.client.NewCancelOrderService().
				Symbol(symbol).
				OrderID(order.OrderID).
				Do(context.Background())

			if err != nil {
				log.Printf("  ⚠ Failed to cancel take profit order %d: %v", order.OrderID, err)
				continue
			}

			canceledCount++
			log.Printf("  ✓ Canceled take profit order (Order ID: %d, Type: %s)", order.OrderID, orderType)
		}
	}

	if canceledCount == 0 {
		log.Printf("  ℹ %s has no take profit orders to cancel", symbol)
	} else {
		log.Printf("  ✓ Canceled %d take profit orders for %s", canceledCount, symbol)
	}

	return nil
}

// CancelAllOrders Cancel all pending orders for this symbol
func (t *FuturesTrader) CancelAllOrders(symbol string) error {
	err := t.client.NewCancelAllOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err != nil {
		return fmt.Errorf("failed to cancel pending orders: %w", err)
	}

	log.Printf("  ✓ Canceled all pending orders for %s", symbol)
	return nil
}

// CancelStopOrders Cancel stop loss/take profit orders for this symbol (used to adjust stop loss/take profit positions)
func (t *FuturesTrader) CancelStopOrders(symbol string) error {
	// Get all open orders for this symbol
	orders, err := t.client.NewListOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err != nil {
		return fmt.Errorf("failed to get open orders: %w", err)
	}

	// Filter out stop loss/take profit orders and cancel them
	canceledCount := 0
	for _, order := range orders {
		orderType := order.Type

		// Only cancel stop loss and take profit orders
		if orderType == futures.OrderTypeStopMarket ||
			orderType == futures.OrderTypeTakeProfitMarket ||
			orderType == futures.OrderTypeStop ||
			orderType == futures.OrderTypeTakeProfit {

			_, err := t.client.NewCancelOrderService().
				Symbol(symbol).
				OrderID(order.OrderID).
				Do(context.Background())

			if err != nil {
				log.Printf("  ⚠ Failed to cancel order %d: %v", order.OrderID, err)
				continue
			}

			canceledCount++
			log.Printf("  ✓ Canceled stop loss/take profit order for %s (Order ID: %d, Type: %s)",
				symbol, order.OrderID, orderType)
		}
	}

	if canceledCount == 0 {
		log.Printf("  ℹ %s has no stop loss/take profit orders to cancel", symbol)
	} else {
		log.Printf("  ✓ Canceled %d stop loss/take profit orders for %s", canceledCount, symbol)
	}

	return nil
}

// GetMarketPrice Get market price
func (t *FuturesTrader) GetMarketPrice(symbol string) (float64, error) {
	prices, err := t.client.NewListPricesService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return 0, fmt.Errorf("failed to get price: %w", err)
	}

	if len(prices) == 0 {
		return 0, fmt.Errorf("price not found")
	}

	price, err := strconv.ParseFloat(prices[0].Price, 64)
	if err != nil {
		return 0, err
	}

	return price, nil
}

// CalculatePositionSize Calculate position size
func (t *FuturesTrader) CalculatePositionSize(balance, riskPercent, price float64, leverage int) float64 {
	riskAmount := balance * (riskPercent / 100.0)
	positionValue := riskAmount * float64(leverage)
	quantity := positionValue / price
	return quantity
}

// SetStopLoss Set stop loss order
func (t *FuturesTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	var side futures.SideType
	var posSide futures.PositionSideType

	if positionSide == "LONG" {
		side = futures.SideTypeSell
		posSide = futures.PositionSideTypeLong
	} else {
		side = futures.SideTypeBuy
		posSide = futures.PositionSideTypeShort
	}

	// Format quantity
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return err
	}

	_, err = t.client.NewCreateOrderService().
		Symbol(symbol).
		Side(side).
		PositionSide(posSide).
		Type(futures.OrderTypeStopMarket).
		StopPrice(fmt.Sprintf("%.8f", stopPrice)).
		Quantity(quantityStr).
		WorkingType(futures.WorkingTypeContractPrice).
		ClosePosition(true).
		Do(context.Background())

	if err != nil {
		return fmt.Errorf("failed to set stop loss: %w", err)
	}

	log.Printf("  Stop loss price set: %.4f", stopPrice)
	return nil
}

// SetTakeProfit Set take profit order
func (t *FuturesTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	var side futures.SideType
	var posSide futures.PositionSideType

	if positionSide == "LONG" {
		side = futures.SideTypeSell
		posSide = futures.PositionSideTypeLong
	} else {
		side = futures.SideTypeBuy
		posSide = futures.PositionSideTypeShort
	}

	// Format quantity
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return err
	}

	_, err = t.client.NewCreateOrderService().
		Symbol(symbol).
		Side(side).
		PositionSide(posSide).
		Type(futures.OrderTypeTakeProfitMarket).
		StopPrice(fmt.Sprintf("%.8f", takeProfitPrice)).
		Quantity(quantityStr).
		WorkingType(futures.WorkingTypeContractPrice).
		ClosePosition(true).
		Do(context.Background())

	if err != nil {
		return fmt.Errorf("failed to set take profit: %w", err)
	}

	log.Printf("  Take profit price set: %.4f", takeProfitPrice)
	return nil
}

// GetMinNotional Get minimum notional value (Binance requirement)
func (t *FuturesTrader) GetMinNotional(symbol string) float64 {
	// Use conservative default value of 10 USDT to ensure orders pass exchange validation
	return 10.0
}

// CheckMinNotional Check if order meets minimum notional value requirement
func (t *FuturesTrader) CheckMinNotional(symbol string, quantity float64) error {
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return fmt.Errorf("failed to get market price: %w", err)
	}

	notionalValue := quantity * price
	minNotional := t.GetMinNotional(symbol)

	if notionalValue < minNotional {
		return fmt.Errorf(
			"order amount %.2f USDT is below minimum requirement %.2f USDT (quantity: %.4f, price: %.4f)",
			notionalValue, minNotional, quantity, price,
		)
	}

	return nil
}

// GetSymbolPrecision Get quantity precision for trading pair
func (t *FuturesTrader) GetSymbolPrecision(symbol string) (int, error) {
	exchangeInfo, err := t.client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		return 0, fmt.Errorf("failed to get trading rules: %w", err)
	}

	for _, s := range exchangeInfo.Symbols {
		if s.Symbol == symbol {
			// Get precision from LOT_SIZE filter
			for _, filter := range s.Filters {
				if filter["filterType"] == "LOT_SIZE" {
					stepSize := filter["stepSize"].(string)
					precision := calculatePrecision(stepSize)
					log.Printf("  %s quantity precision: %d (stepSize: %s)", symbol, precision, stepSize)
					return precision, nil
				}
			}
		}
	}

	log.Printf("  ⚠ %s precision information not found, using default precision 3", symbol)
	return 3, nil // Default precision is 3
}

// calculatePrecision Calculate precision from stepSize
func calculatePrecision(stepSize string) int {
	// Remove trailing zeros
	stepSize = trimTrailingZeros(stepSize)

	// Find decimal point
	dotIndex := -1
	for i := 0; i < len(stepSize); i++ {
		if stepSize[i] == '.' {
			dotIndex = i
			break
		}
	}

	// If no decimal point or decimal point is at the end, precision is 0
	if dotIndex == -1 || dotIndex == len(stepSize)-1 {
		return 0
	}

	// Return number of digits after decimal point
	return len(stepSize) - dotIndex - 1
}

// trimTrailingZeros Remove trailing zeros
func trimTrailingZeros(s string) string {
	// If no decimal point, return directly
	if !stringContains(s, ".") {
		return s
	}

	// Traverse from back to front, remove trailing zeros
	for len(s) > 0 && s[len(s)-1] == '0' {
		s = s[:len(s)-1]
	}

	// If last character is decimal point, also remove it
	if len(s) > 0 && s[len(s)-1] == '.' {
		s = s[:len(s)-1]
	}

	return s
}

// FormatQuantity Format quantity to correct precision
func (t *FuturesTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	precision, err := t.GetSymbolPrecision(symbol)
	if err != nil {
		// If getting precision fails, use default format
		return fmt.Sprintf("%.3f", quantity), nil
	}

	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, quantity), nil
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
