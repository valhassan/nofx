package logger

import (
	"fmt"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramSender Telegram message sender (async)
type TelegramSender struct {
	bot           *tgbotapi.BotAPI
	chatID        int64
	msgChan       chan string
	retryCount    int
	retryInterval time.Duration
	wg            sync.WaitGroup
	stopChan      chan struct{}
	once          sync.Once
}

// NewTelegramSender creates Telegram sender (using default parameters)
func NewTelegramSender(botToken string, chatID int64) (*TelegramSender, error) {
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}

	// Set to silent mode (don't print bot info)
	bot.Debug = false

	sender := &TelegramSender{
		bot:           bot,
		chatID:        chatID,
		msgChan:       make(chan string, 20),     // Fixed buffer size: 20
		retryCount:    3,                         // Fixed retry count: 3
		retryInterval: 3 * time.Second,          // Fixed retry interval: 3 seconds
		stopChan:      make(chan struct{}),
	}

	// Start async send goroutine
	sender.Start()

	return sender, nil
}

// Start starts async send goroutine
func (s *TelegramSender) Start() {
	s.wg.Add(1)
	go s.listenAndSend()
}

// SendAsync sends message asynchronously (non-blocking)
func (s *TelegramSender) SendAsync(message string) {
	select {
	case s.msgChan <- message:
		// Successfully written to buffer
	default:
		// Buffer full, discard message (don't block main flow)
		fmt.Printf("[Telegram] Message buffer full, message discarded\n")
	}
}

// listenAndSend listens to channel and sends messages
func (s *TelegramSender) listenAndSend() {
	defer s.wg.Done()

	for {
		select {
		case msg := <-s.msgChan:
			s.sendWithRetry(msg)
		case <-s.stopChan:
			// Clear buffer before exiting
			for len(s.msgChan) > 0 {
				msg := <-s.msgChan
				s.sendWithRetry(msg)
			}
			return
		}
	}
}

// sendWithRetry sends message (with retry)
func (s *TelegramSender) sendWithRetry(message string) {
	var err error
	for i := 0; i < s.retryCount; i++ {
		err = s.send(message)
		if err == nil {
			return // Send successful
		}

		// Wait before retry
		if i < s.retryCount-1 {
			time.Sleep(s.retryInterval)
		}
	}

	// All retries failed
	if err != nil {
		fmt.Printf("[Telegram] Failed to send message (retried %d times): %v\n", s.retryCount, err)
	}
}

// send sends a single message
func (s *TelegramSender) send(message string) error {
	msg := tgbotapi.NewMessage(s.chatID, message)
	msg.ParseMode = tgbotapi.ModeMarkdown

	_, err := s.bot.Send(msg)
	return err
}

// Stop stops sender (graceful shutdown)
func (s *TelegramSender) Stop() {
	s.once.Do(func() {
		close(s.stopChan)
		s.wg.Wait()
	})
}
