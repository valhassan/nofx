package trader

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// AsterTrader Aster trading platform implementation
type AsterTrader struct {
	ctx        context.Context
	user       string            // Main wallet address (ERC20)
	signer     string            // API wallet address
	privateKey *ecdsa.PrivateKey // API wallet private key
	client     *http.Client
	baseURL    string

	// Cache symbol precision information
	symbolPrecision map[string]SymbolPrecision
	mu              sync.RWMutex
}

// SymbolPrecision Symbol precision information
type SymbolPrecision struct {
	PricePrecision    int
	QuantityPrecision int
	TickSize          float64 // Price step size
	StepSize          float64 // Quantity step size
}

// NewAsterTrader Create Aster trader
// user: Main wallet address (login address)
// signer: API wallet address (obtained from https://www.asterdex.com/en/api-wallet)
// privateKey: API wallet private key (obtained from https://www.asterdex.com/en/api-wallet)
func NewAsterTrader(user, signer, privateKeyHex string) (*AsterTrader, error) {
	// Parse private key
	privKey, err := crypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return &AsterTrader{
		ctx:             context.Background(),
		user:            user,
		signer:          signer,
		privateKey:      privKey,
		symbolPrecision: make(map[string]SymbolPrecision),
		client: &http.Client{
			Timeout: 30 * time.Second, // Increased to 30 seconds
			Transport: &http.Transport{
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 10 * time.Second,
				IdleConnTimeout:       90 * time.Second,
			},
		},
		baseURL: "https://fapi.asterdex.com",
	}, nil
}

// genNonce Generate microsecond timestamp
func (t *AsterTrader) genNonce() uint64 {
	return uint64(time.Now().UnixMicro())
}

// getPrecision Get symbol precision information
func (t *AsterTrader) getPrecision(symbol string) (SymbolPrecision, error) {
	t.mu.RLock()
	if prec, ok := t.symbolPrecision[symbol]; ok {
		t.mu.RUnlock()
		return prec, nil
	}
	t.mu.RUnlock()

	// Get exchange information
	resp, err := t.client.Get(t.baseURL + "/fapi/v3/exchangeInfo")
	if err != nil {
		return SymbolPrecision{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var info struct {
		Symbols []struct {
			Symbol            string                   `json:"symbol"`
			PricePrecision    int                      `json:"pricePrecision"`
			QuantityPrecision int                      `json:"quantityPrecision"`
			Filters           []map[string]interface{} `json:"filters"`
		} `json:"symbols"`
	}

	if err := json.Unmarshal(body, &info); err != nil {
		return SymbolPrecision{}, err
	}

	// Cache precision for all symbols
	t.mu.Lock()
	for _, s := range info.Symbols {
		prec := SymbolPrecision{
			PricePrecision:    s.PricePrecision,
			QuantityPrecision: s.QuantityPrecision,
		}

		// Parse filters to get tickSize and stepSize
		for _, filter := range s.Filters {
			filterType, _ := filter["filterType"].(string)
			switch filterType {
			case "PRICE_FILTER":
				if tickSizeStr, ok := filter["tickSize"].(string); ok {
					prec.TickSize, _ = strconv.ParseFloat(tickSizeStr, 64)
				}
			case "LOT_SIZE":
				if stepSizeStr, ok := filter["stepSize"].(string); ok {
					prec.StepSize, _ = strconv.ParseFloat(stepSizeStr, 64)
				}
			}
		}

		t.symbolPrecision[s.Symbol] = prec
	}
	t.mu.Unlock()

	if prec, ok := t.symbolPrecision[symbol]; ok {
		return prec, nil
	}

	return SymbolPrecision{}, fmt.Errorf("precision information not found for symbol %s", symbol)
}

// roundToTickSize Round price/quantity to integer multiple of tick size/step size
func roundToTickSize(value float64, tickSize float64) float64 {
	if tickSize <= 0 {
		return value
	}
	// Calculate how many tick sizes
	steps := value / tickSize
	// Round to nearest integer
	roundedSteps := math.Round(steps)
	// Multiply back by tick size
	return roundedSteps * tickSize
}

// formatPrice Format price to correct precision and tick size
func (t *AsterTrader) formatPrice(symbol string, price float64) (float64, error) {
	prec, err := t.getPrecision(symbol)
	if err != nil {
		return 0, err
	}

	// Prefer tick size to ensure price is an integer multiple of tick size
	if prec.TickSize > 0 {
		return roundToTickSize(price, prec.TickSize), nil
	}

	// If no tick size, round by precision
	multiplier := math.Pow10(prec.PricePrecision)
	return math.Round(price*multiplier) / multiplier, nil
}

// formatQuantity Format quantity to correct precision and step size
func (t *AsterTrader) formatQuantity(symbol string, quantity float64) (float64, error) {
	prec, err := t.getPrecision(symbol)
	if err != nil {
		return 0, err
	}

	// Prefer step size to ensure quantity is an integer multiple of step size
	if prec.StepSize > 0 {
		return roundToTickSize(quantity, prec.StepSize), nil
	}

	// If no step size, round by precision
	multiplier := math.Pow10(prec.QuantityPrecision)
	return math.Round(quantity*multiplier) / multiplier, nil
}

// formatFloatWithPrecision Format float to string with specified precision (remove trailing zeros)
func (t *AsterTrader) formatFloatWithPrecision(value float64, precision int) string {
	// Format with specified precision
	formatted := strconv.FormatFloat(value, 'f', precision, 64)

	// Remove trailing zeros and decimal point (if any)
	formatted = strings.TrimRight(formatted, "0")
	formatted = strings.TrimRight(formatted, ".")

	return formatted
}

// normalizeAndStringify Normalize parameters and serialize to JSON string (sorted by key)
func (t *AsterTrader) normalizeAndStringify(params map[string]interface{}) (string, error) {
	normalized, err := t.normalize(params)
	if err != nil {
		return "", err
	}
	bs, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

// normalize Recursively normalize parameters (sorted by key, all values converted to strings)
func (t *AsterTrader) normalize(v interface{}) (interface{}, error) {
	switch val := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		newMap := make(map[string]interface{}, len(keys))
		for _, k := range keys {
			nv, err := t.normalize(val[k])
			if err != nil {
				return nil, err
			}
			newMap[k] = nv
		}
		return newMap, nil
	case []interface{}:
		out := make([]interface{}, 0, len(val))
		for _, it := range val {
			nv, err := t.normalize(it)
			if err != nil {
				return nil, err
			}
			out = append(out, nv)
		}
		return out, nil
	case string:
		return val, nil
	case int:
		return fmt.Sprintf("%d", val), nil
	case int64:
		return fmt.Sprintf("%d", val), nil
	case float64:
		return fmt.Sprintf("%v", val), nil
	case bool:
		return fmt.Sprintf("%v", val), nil
	default:
		// Convert other types to string
		return fmt.Sprintf("%v", val), nil
	}
}

// sign Sign request parameters
func (t *AsterTrader) sign(params map[string]interface{}, nonce uint64) error {
	// Add timestamp and receive window
	params["recvWindow"] = "50000"
	params["timestamp"] = strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10)

	// Normalize parameters to JSON string
	jsonStr, err := t.normalizeAndStringify(params)
	if err != nil {
		return err
	}

	// ABI encoding: (string, address, address, uint256)
	addrUser := common.HexToAddress(t.user)
	addrSigner := common.HexToAddress(t.signer)
	nonceBig := new(big.Int).SetUint64(nonce)

	tString, _ := abi.NewType("string", "", nil)
	tAddress, _ := abi.NewType("address", "", nil)
	tUint256, _ := abi.NewType("uint256", "", nil)

	arguments := abi.Arguments{
		{Type: tString},
		{Type: tAddress},
		{Type: tAddress},
		{Type: tUint256},
	}

	packed, err := arguments.Pack(jsonStr, addrUser, addrSigner, nonceBig)
	if err != nil {
		return fmt.Errorf("ABI encoding failed: %w", err)
	}

	// Keccak256 hash
	hash := crypto.Keccak256(packed)

	// Ethereum signed message prefix
	prefixedMsg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(hash), hash)
	msgHash := crypto.Keccak256Hash([]byte(prefixedMsg))

	// ECDSA signature
	sig, err := crypto.Sign(msgHash.Bytes(), t.privateKey)
	if err != nil {
		return fmt.Errorf("signing failed: %w", err)
	}

	// Convert v from 0/1 to 27/28
	if len(sig) != 65 {
		return fmt.Errorf("invalid signature length: %d", len(sig))
	}
	sig[64] += 27

	// Add signature parameters
	params["user"] = t.user
	params["signer"] = t.signer
	params["signature"] = "0x" + hex.EncodeToString(sig)
	params["nonce"] = nonce

	return nil
}

// request Send HTTP request (with retry mechanism)
func (t *AsterTrader) request(method, endpoint string, params map[string]interface{}) ([]byte, error) {
	const maxRetries = 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Generate new nonce and signature for each retry
		nonce := t.genNonce()
		paramsCopy := make(map[string]interface{})
		for k, v := range params {
			paramsCopy[k] = v
		}

		// Sign
		if err := t.sign(paramsCopy, nonce); err != nil {
			return nil, err
		}

		body, err := t.doRequest(method, endpoint, paramsCopy)
		if err == nil {
			return body, nil
		}

		lastErr = err

		// Retry on network timeout or temporary errors
		if strings.Contains(err.Error(), "timeout") ||
			strings.Contains(err.Error(), "connection reset") ||
			strings.Contains(err.Error(), "EOF") {
			if attempt < maxRetries {
				waitTime := time.Duration(attempt) * time.Second
				time.Sleep(waitTime)
				continue
			}
		}

		// Other errors (like 400/401) don't retry
		return nil, err
	}

	return nil, fmt.Errorf("request failed (retried %d times): %w", maxRetries, lastErr)
}

// doRequest Execute actual HTTP request
func (t *AsterTrader) doRequest(method, endpoint string, params map[string]interface{}) ([]byte, error) {
	fullURL := t.baseURL + endpoint
	method = strings.ToUpper(method)

	switch method {
	case "POST":
		// POST request: parameters in form body
		form := url.Values{}
		for k, v := range params {
			form.Set(k, fmt.Sprintf("%v", v))
		}
		req, err := http.NewRequest("POST", fullURL, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := t.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}
		return body, nil

	case "GET", "DELETE":
		// GET/DELETE request: parameters in querystring
		q := url.Values{}
		for k, v := range params {
			q.Set(k, fmt.Sprintf("%v", v))
		}
		u, _ := url.Parse(fullURL)
		u.RawQuery = q.Encode()

		req, err := http.NewRequest(method, u.String(), nil)
		if err != nil {
			return nil, err
		}

		resp, err := t.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}
		return body, nil

	default:
		return nil, fmt.Errorf("unsupported HTTP method: %s", method)
	}
}

// GetBalance Get account balance
func (t *AsterTrader) GetBalance() (map[string]interface{}, error) {
	params := make(map[string]interface{})
	body, err := t.request("GET", "/fapi/v3/balance", params)
	if err != nil {
		return nil, err
	}

	var balances []map[string]interface{}
	if err := json.Unmarshal(body, &balances); err != nil {
		return nil, err
	}

	// 🔍 Debug: print raw API response
	log.Printf("🔍 Aster API raw response: %s", string(body))

	// Find USDT balance
	totalBalance := 0.0
	availableBalance := 0.0
	crossUnPnl := 0.0

	for _, bal := range balances {
		// 🔍 Debug: print each balance record
		log.Printf("🔍 Balance record: %+v", bal)

		if asset, ok := bal["asset"].(string); ok && asset == "USDT" {
			// 🔍 Debug: print USDT balance details
			log.Printf("🔍 USDT balance details: balance=%v, availableBalance=%v, crossUnPnl=%v",
				bal["balance"], bal["availableBalance"], bal["crossUnPnl"])

			if wb, ok := bal["balance"].(string); ok {
				totalBalance, _ = strconv.ParseFloat(wb, 64)
			}
			if avail, ok := bal["availableBalance"].(string); ok {
				availableBalance, _ = strconv.ParseFloat(avail, 64)
			}
			if unpnl, ok := bal["crossUnPnl"].(string); ok {
				crossUnPnl, _ = strconv.ParseFloat(unpnl, 64)
			}
			break
		}
	}

	// ✅ Aster API is fully compatible with Binance API format
	// balance field = wallet balance (excluding unrealized profit/loss)
	// crossUnPnl = unrealized profit (unrealized profit/loss)
	// crossWalletBalance = balance + crossUnPnl (cross margin wallet balance, including profit/loss)
	//
	// Reference Binance official documentation:
	// - Account Information V2: marginBalance = walletBalance + unrealizedProfit
	// - Balance V3: crossWalletBalance = balance + crossUnPnl

	log.Printf("✓ Aster API returned: wallet balance=%.2f, unrealized PnL=%.2f, available balance=%.2f",
		totalBalance,
		crossUnPnl,
		availableBalance)

	// Return same field names as Binance to ensure AutoTrader can parse correctly
	return map[string]interface{}{
		"totalWalletBalance":    totalBalance, // Wallet balance (excluding unrealized profit/loss)
		"availableBalance":      availableBalance,
		"totalUnrealizedProfit": crossUnPnl, // Unrealized profit/loss
	}, nil
}

// GetPositions Get position information
func (t *AsterTrader) GetPositions() ([]map[string]interface{}, error) {
	params := make(map[string]interface{})
	body, err := t.request("GET", "/fapi/v3/positionRisk", params)
	if err != nil {
		return nil, err
	}

	var positions []map[string]interface{}
	if err := json.Unmarshal(body, &positions); err != nil {
		return nil, err
	}

	result := []map[string]interface{}{}
	for _, pos := range positions {
		posAmtStr, ok := pos["positionAmt"].(string)
		if !ok {
			continue
		}

		posAmt, _ := strconv.ParseFloat(posAmtStr, 64)
		if posAmt == 0 {
			continue // Skip empty positions
		}

		entryPrice, _ := strconv.ParseFloat(pos["entryPrice"].(string), 64)
		markPrice, _ := strconv.ParseFloat(pos["markPrice"].(string), 64)
		unRealizedProfit, _ := strconv.ParseFloat(pos["unRealizedProfit"].(string), 64)
		leverageVal, _ := strconv.ParseFloat(pos["leverage"].(string), 64)
		liquidationPrice, _ := strconv.ParseFloat(pos["liquidationPrice"].(string), 64)

		// Determine direction (consistent with Binance)
		side := "long"
		if posAmt < 0 {
			side = "short"
			posAmt = -posAmt
		}

		// Return same field names as Binance
		result = append(result, map[string]interface{}{
			"symbol":           pos["symbol"],
			"side":             side,
			"positionAmt":      posAmt,
			"entryPrice":       entryPrice,
			"markPrice":        markPrice,
			"unRealizedProfit": unRealizedProfit,
			"leverage":         leverageVal,
			"liquidationPrice": liquidationPrice,
		})
	}

	return result, nil
}

// OpenLong Open long position
func (t *AsterTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// Cancel all pending orders before opening position to prevent position stacking from residual orders
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ Failed to cancel orders (continuing to open): %v", err)
	}

	// Set leverage first
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, fmt.Errorf("failed to set leverage: %w", err)
	}

	// Get current price
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// Use limit order to simulate market order (set price slightly higher to ensure execution)
	limitPrice := price * 1.01

	// Format price and quantity to correct precision
	formattedPrice, err := t.formatPrice(symbol, limitPrice)
	if err != nil {
		return nil, err
	}
	formattedQty, err := t.formatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Get precision information
	prec, err := t.getPrecision(symbol)
	if err != nil {
		return nil, err
	}

	// Convert to string with correct precision format
	priceStr := t.formatFloatWithPrecision(formattedPrice, prec.PricePrecision)
	qtyStr := t.formatFloatWithPrecision(formattedQty, prec.QuantityPrecision)

	log.Printf("  📏 Precision handling: price %.8f -> %s (precision=%d), quantity %.8f -> %s (precision=%d)",
		limitPrice, priceStr, prec.PricePrecision, quantity, qtyStr, prec.QuantityPrecision)

	params := map[string]interface{}{
		"symbol":       symbol,
		"positionSide": "BOTH",
		"type":         "LIMIT",
		"side":         "BUY",
		"timeInForce":  "GTC",
		"quantity":     qtyStr,
		"price":        priceStr,
	}

	body, err := t.request("POST", "/fapi/v3/order", params)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// OpenShort Open short position
func (t *AsterTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// Cancel all pending orders before opening position to prevent position stacking from residual orders
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ Failed to cancel orders (continuing to open): %v", err)
	}

	// Set leverage first
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, fmt.Errorf("failed to set leverage: %w", err)
	}

	// Get current price
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// Use limit order to simulate market order (set price slightly lower to ensure execution)
	limitPrice := price * 0.99

	// Format price and quantity to correct precision
	formattedPrice, err := t.formatPrice(symbol, limitPrice)
	if err != nil {
		return nil, err
	}
	formattedQty, err := t.formatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Get precision information
	prec, err := t.getPrecision(symbol)
	if err != nil {
		return nil, err
	}

	// Convert to string with correct precision format
	priceStr := t.formatFloatWithPrecision(formattedPrice, prec.PricePrecision)
	qtyStr := t.formatFloatWithPrecision(formattedQty, prec.QuantityPrecision)

	log.Printf("  📏 Precision handling: price %.8f -> %s (precision=%d), quantity %.8f -> %s (precision=%d)",
		limitPrice, priceStr, prec.PricePrecision, quantity, qtyStr, prec.QuantityPrecision)

	params := map[string]interface{}{
		"symbol":       symbol,
		"positionSide": "BOTH",
		"type":         "LIMIT",
		"side":         "SELL",
		"timeInForce":  "GTC",
		"quantity":     qtyStr,
		"price":        priceStr,
	}

	body, err := t.request("POST", "/fapi/v3/order", params)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// CloseLong Close long position
func (t *AsterTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
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
		log.Printf("  📊 Retrieved long position quantity: %.8f", quantity)
	}

	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	limitPrice := price * 0.99

	// Format price and quantity to correct precision
	formattedPrice, err := t.formatPrice(symbol, limitPrice)
	if err != nil {
		return nil, err
	}
	formattedQty, err := t.formatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Get precision information
	prec, err := t.getPrecision(symbol)
	if err != nil {
		return nil, err
	}

	// Convert to string with correct precision format
	priceStr := t.formatFloatWithPrecision(formattedPrice, prec.PricePrecision)
	qtyStr := t.formatFloatWithPrecision(formattedQty, prec.QuantityPrecision)

	log.Printf("  📏 Precision handling: price %.8f -> %s (precision=%d), quantity %.8f -> %s (precision=%d)",
		limitPrice, priceStr, prec.PricePrecision, quantity, qtyStr, prec.QuantityPrecision)

	params := map[string]interface{}{
		"symbol":       symbol,
		"positionSide": "BOTH",
		"type":         "LIMIT",
		"side":         "SELL",
		"timeInForce":  "GTC",
		"quantity":     qtyStr,
		"price":        priceStr,
	}

	body, err := t.request("POST", "/fapi/v3/order", params)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	log.Printf("✓ Successfully closed long position: %s quantity: %s", symbol, qtyStr)

	// Cancel all pending orders for this symbol after closing (stop loss/take profit orders)
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ Failed to cancel orders: %v", err)
	}

	return result, nil
}

// CloseShort Close short position
func (t *AsterTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// If quantity is 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "short" {
				// Aster's GetPositions already converts short position quantity to positive, use directly
				quantity = pos["positionAmt"].(float64)
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("short position not found for %s", symbol)
		}
		log.Printf("  📊 Retrieved short position quantity: %.8f", quantity)
	}

	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	limitPrice := price * 1.01

	// Format price and quantity to correct precision
	formattedPrice, err := t.formatPrice(symbol, limitPrice)
	if err != nil {
		return nil, err
	}
	formattedQty, err := t.formatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Get precision information
	prec, err := t.getPrecision(symbol)
	if err != nil {
		return nil, err
	}

	// Convert to string with correct precision format
	priceStr := t.formatFloatWithPrecision(formattedPrice, prec.PricePrecision)
	qtyStr := t.formatFloatWithPrecision(formattedQty, prec.QuantityPrecision)

	log.Printf("  📏 Precision handling: price %.8f -> %s (precision=%d), quantity %.8f -> %s (precision=%d)",
		limitPrice, priceStr, prec.PricePrecision, quantity, qtyStr, prec.QuantityPrecision)

	params := map[string]interface{}{
		"symbol":       symbol,
		"positionSide": "BOTH",
		"type":         "LIMIT",
		"side":         "BUY",
		"timeInForce":  "GTC",
		"quantity":     qtyStr,
		"price":        priceStr,
	}

	body, err := t.request("POST", "/fapi/v3/order", params)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	log.Printf("✓ Successfully closed short position: %s quantity: %s", symbol, qtyStr)

	// Cancel all pending orders for this symbol after closing (stop loss/take profit orders)
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ Failed to cancel orders: %v", err)
	}

	return result, nil
}

// SetMarginMode Set margin mode
func (t *AsterTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	// Aster supports margin mode setting
	// API format similar to Binance: CROSSED (cross margin) / ISOLATED (isolated margin)
	marginType := "CROSSED"
	if !isCrossMargin {
		marginType = "ISOLATED"
	}

	params := map[string]interface{}{
		"symbol":     symbol,
		"marginType": marginType,
	}

	// Use request method to call API
	_, err := t.request("POST", "/fapi/v3/marginType", params)
	if err != nil {
		// If error indicates no change needed, ignore error
		if strings.Contains(err.Error(), "No need to change") ||
			strings.Contains(err.Error(), "Margin type cannot be changed") {
			log.Printf("  ✓ %s margin mode is already %s or has positions that cannot be changed", symbol, marginType)
			return nil
		}
		// Detect multi-asset mode (error code -4168)
		if strings.Contains(err.Error(), "Multi-Assets mode") ||
			strings.Contains(err.Error(), "-4168") ||
			strings.Contains(err.Error(), "4168") {
			log.Printf("  ⚠️ %s detected multi-asset mode, forcing cross margin mode", symbol)
			log.Printf("  💡 Tip: To use isolated margin mode, please disable multi-asset mode on the exchange")
			return nil
		}
		// Detect unified account API
		if strings.Contains(err.Error(), "unified") ||
			strings.Contains(err.Error(), "portfolio") ||
			strings.Contains(err.Error(), "Portfolio") {
			log.Printf("  ❌ %s detected unified account API, cannot perform futures trading", symbol)
			return fmt.Errorf("please use 'Spot & Futures Trading' API permissions, do not use 'Unified Account API'")
		}
		log.Printf("  ⚠️ Failed to set margin mode: %v", err)
		// Don't return error, let trading continue
		return nil
	}

	log.Printf("  ✓ %s margin mode set to %s", symbol, marginType)
	return nil
}

// SetLeverage Set leverage multiplier
func (t *AsterTrader) SetLeverage(symbol string, leverage int) error {
	params := map[string]interface{}{
		"symbol":   symbol,
		"leverage": leverage,
	}

	_, err := t.request("POST", "/fapi/v3/leverage", params)
	return err
}

// GetMarketPrice Get market price
func (t *AsterTrader) GetMarketPrice(symbol string) (float64, error) {
	// Use ticker interface to get current price
	resp, err := t.client.Get(fmt.Sprintf("%s/fapi/v3/ticker/price?symbol=%s", t.baseURL, symbol))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	priceStr, ok := result["price"].(string)
	if !ok {
		return 0, errors.New("unable to get price")
	}

	return strconv.ParseFloat(priceStr, 64)
}

// SetStopLoss Set stop loss
func (t *AsterTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	side := "SELL"
	if positionSide == "SHORT" {
		side = "BUY"
	}

	// Format price and quantity to correct precision
	formattedPrice, err := t.formatPrice(symbol, stopPrice)
	if err != nil {
		return err
	}
	formattedQty, err := t.formatQuantity(symbol, quantity)
	if err != nil {
		return err
	}

	// Get precision information
	prec, err := t.getPrecision(symbol)
	if err != nil {
		return err
	}

	// Convert to string with correct precision format
	priceStr := t.formatFloatWithPrecision(formattedPrice, prec.PricePrecision)
	qtyStr := t.formatFloatWithPrecision(formattedQty, prec.QuantityPrecision)

	params := map[string]interface{}{
		"symbol":       symbol,
		"positionSide": "BOTH",
		"type":         "STOP_MARKET",
		"side":         side,
		"stopPrice":    priceStr,
		"quantity":     qtyStr,
		"timeInForce":  "GTC",
	}

	_, err = t.request("POST", "/fapi/v3/order", params)
	return err
}

// SetTakeProfit Set take profit
func (t *AsterTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	side := "SELL"
	if positionSide == "SHORT" {
		side = "BUY"
	}

	// Format price and quantity to correct precision
	formattedPrice, err := t.formatPrice(symbol, takeProfitPrice)
	if err != nil {
		return err
	}
	formattedQty, err := t.formatQuantity(symbol, quantity)
	if err != nil {
		return err
	}

	// Get precision information
	prec, err := t.getPrecision(symbol)
	if err != nil {
		return err
	}

	// Convert to string with correct precision format
	priceStr := t.formatFloatWithPrecision(formattedPrice, prec.PricePrecision)
	qtyStr := t.formatFloatWithPrecision(formattedQty, prec.QuantityPrecision)

	params := map[string]interface{}{
		"symbol":       symbol,
		"positionSide": "BOTH",
		"type":         "TAKE_PROFIT_MARKET",
		"side":         side,
		"stopPrice":    priceStr,
		"quantity":     qtyStr,
		"timeInForce":  "GTC",
	}

	_, err = t.request("POST", "/fapi/v3/order", params)
	return err
}

// CancelStopLossOrders Cancel stop loss orders only (does not affect take profit orders)
func (t *AsterTrader) CancelStopLossOrders(symbol string) error {
	// Get all open orders for this symbol
	params := map[string]interface{}{
		"symbol": symbol,
	}

	body, err := t.request("GET", "/fapi/v3/openOrders", params)
	if err != nil {
		return fmt.Errorf("failed to get open orders: %w", err)
	}

	var orders []map[string]interface{}
	if err := json.Unmarshal(body, &orders); err != nil {
		return fmt.Errorf("failed to parse order data: %w", err)
	}

	// Filter out stop loss orders and cancel them
	canceledCount := 0
	for _, order := range orders {
		orderType, _ := order["type"].(string)

		// Only cancel stop loss orders (don't cancel take profit orders)
		if orderType == "STOP_MARKET" || orderType == "STOP" {
			orderID, _ := order["orderId"].(float64)
			cancelParams := map[string]interface{}{
				"symbol":  symbol,
				"orderId": int64(orderID),
			}

			_, err := t.request("DELETE", "/fapi/v1/order", cancelParams)
			if err != nil {
				log.Printf("  ⚠ Failed to cancel stop loss order %d: %v", int64(orderID), err)
				continue
			}

			canceledCount++
			log.Printf("  ✓ Canceled stop loss order (Order ID: %d, Type: %s)", int64(orderID), orderType)
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
func (t *AsterTrader) CancelTakeProfitOrders(symbol string) error {
	// Get all open orders for this symbol
	params := map[string]interface{}{
		"symbol": symbol,
	}

	body, err := t.request("GET", "/fapi/v3/openOrders", params)
	if err != nil {
		return fmt.Errorf("failed to get open orders: %w", err)
	}

	var orders []map[string]interface{}
	if err := json.Unmarshal(body, &orders); err != nil {
		return fmt.Errorf("failed to parse order data: %w", err)
	}

	// Filter out take profit orders and cancel them
	canceledCount := 0
	for _, order := range orders {
		orderType, _ := order["type"].(string)

		// Only cancel take profit orders (don't cancel stop loss orders)
		if orderType == "TAKE_PROFIT_MARKET" || orderType == "TAKE_PROFIT" {
			orderID, _ := order["orderId"].(float64)
			cancelParams := map[string]interface{}{
				"symbol":  symbol,
				"orderId": int64(orderID),
			}

			_, err := t.request("DELETE", "/fapi/v1/order", cancelParams)
			if err != nil {
				log.Printf("  ⚠ Failed to cancel take profit order %d: %v", int64(orderID), err)
				continue
			}

			canceledCount++
			log.Printf("  ✓ Canceled take profit order (Order ID: %d, Type: %s)", int64(orderID), orderType)
		}
	}

	if canceledCount == 0 {
		log.Printf("  ℹ %s has no take profit orders to cancel", symbol)
	} else {
		log.Printf("  ✓ Canceled %d take profit orders for %s", canceledCount, symbol)
	}

	return nil
}

// CancelAllOrders Cancel all orders
func (t *AsterTrader) CancelAllOrders(symbol string) error {
	params := map[string]interface{}{
		"symbol": symbol,
	}

	_, err := t.request("DELETE", "/fapi/v3/allOpenOrders", params)
	return err
}

// CancelStopOrders Cancel stop loss/take profit orders for this symbol (used to adjust stop loss/take profit positions)
func (t *AsterTrader) CancelStopOrders(symbol string) error {
	// Get all open orders for this symbol
	params := map[string]interface{}{
		"symbol": symbol,
	}

	body, err := t.request("GET", "/fapi/v3/openOrders", params)
	if err != nil {
		return fmt.Errorf("failed to get open orders: %w", err)
	}

	var orders []map[string]interface{}
	if err := json.Unmarshal(body, &orders); err != nil {
		return fmt.Errorf("failed to parse order data: %w", err)
	}

	// Filter out stop loss/take profit orders and cancel them
	canceledCount := 0
	for _, order := range orders {
		orderType, _ := order["type"].(string)

		// Only cancel stop loss and take profit orders
		if orderType == "STOP_MARKET" ||
			orderType == "TAKE_PROFIT_MARKET" ||
			orderType == "STOP" ||
			orderType == "TAKE_PROFIT" {

			orderID, _ := order["orderId"].(float64)
			cancelParams := map[string]interface{}{
				"symbol":  symbol,
				"orderId": int64(orderID),
			}

			_, err := t.request("DELETE", "/fapi/v3/order", cancelParams)
			if err != nil {
				log.Printf("  ⚠ Failed to cancel order %d: %v", int64(orderID), err)
				continue
			}

			canceledCount++
			log.Printf("  ✓ Canceled stop loss/take profit order for %s (Order ID: %d, Type: %s)",
				symbol, int64(orderID), orderType)
		}
	}

	if canceledCount == 0 {
		log.Printf("  ℹ %s has no stop loss/take profit orders to cancel", symbol)
	} else {
		log.Printf("  ✓ Canceled %d stop loss/take profit orders for %s", canceledCount, symbol)
	}

	return nil
}

// FormatQuantity Format quantity (implements Trader interface)
func (t *AsterTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	formatted, err := t.formatQuantity(symbol, quantity)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", formatted), nil
}
