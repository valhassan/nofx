package trader

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
