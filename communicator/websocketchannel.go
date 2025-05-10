// Package communicator implements a base communicator for network connections.
package communicator

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/websocketutil"
)

var (
	// ErrConnectionClosed is returned when the connection is closed.
	ErrConnectionClosed = fmt.Errorf("%w: connection closed", io.EOF)

	// ErrEmptyInput is returned when the input is empty.
	ErrEmptyInput = errors.New("input is empty")
)

// WebSocketChannel parent class for DataChannel.
type WebSocketChannel struct {
	channelURL   string
	isOpen       atomic.Bool // Use atomic.Bool instead of bool for thread safety
	writeLock    sync.Mutex
	connection   *websocket.Conn
	channelToken string
	logger       log.T
}

// NewWebSocketChannel creates a WebSocketChannel.
func NewWebSocketChannel(channelURL string, channelToken string, logger log.T) (*WebSocketChannel, error) {
	return &WebSocketChannel{
		channelToken: channelToken,
		channelURL:   channelURL,
		logger:       logger,
	}, nil
}

// GetChannelToken gets the channel token.
func (c *WebSocketChannel) GetChannelToken() string {
	return c.channelToken
}

// SetChannelToken sets the channel token.
func (c *WebSocketChannel) SetChannelToken(channelToken string) {
	c.channelToken = channelToken
}

// GetStreamURL gets stream url.
func (c *WebSocketChannel) GetStreamURL() string {
	return c.channelURL
}

// SendMessage sends a byte message through the websocket connection.
// Examples of message type are websocket.TextMessage or websocket.Binary.
func (c *WebSocketChannel) SendMessage(input []byte, inputType int) error {
	if !c.isOpen.Load() {
		return ErrConnectionClosed
	}

	if len(input) < 1 {
		return ErrEmptyInput
	}

	c.writeLock.Lock()
	err := c.connection.WriteMessage(inputType, input)
	c.writeLock.Unlock()

	if err != nil {
		return fmt.Errorf("writing websocket message: %w", err)
	}

	return nil
}

// Close closes the corresponding connection.
func (c *WebSocketChannel) Close() error {
	c.logger.Debug("Closing websocket channel connection", "url", c.channelURL)

	// Use CompareAndSwap to safely transition from open to closed state
	if c.isOpen.CompareAndSwap(true, false) {
		if err := websocketutil.NewWebsocketUtil(c.logger, nil).CloseConnection(c.connection); err != nil {
			return fmt.Errorf("closing websocket connection: %w", err)
		}

		return nil
	}

	c.logger.Warn("Websocket channel connection is already Closed!", "url", c.channelURL)

	return nil
}

// Open upgrades the http connection to a websocket connection.
func (c *WebSocketChannel) Open() error {
	ws, err := websocketutil.NewWebsocketUtil(c.logger, nil).OpenConnection(c.channelURL)
	if err != nil {
		return fmt.Errorf("opening websocket connection: %w", err)
	}

	c.connection = ws

	c.isOpen.Store(true)

	go func() {
		c.startPings(config.PingTimeInterval)
	}()

	return nil
}

// IsOpen checks if the channel is open.
func (c *WebSocketChannel) IsOpen() bool {
	return c.isOpen.Load()
}

// ReadMessage reads a message from the websocket connection.
func (c *WebSocketChannel) ReadMessage() ([]byte, error) {
	retryCount := 0

	for {
		if !c.isOpen.Load() {
			return nil, ErrConnectionClosed
		}

		messageType, rawMessage, err := c.connection.ReadMessage()

		switch {
		case err != nil:
			c.logger.Warn("Error receiving message", "retryCount", retryCount, "error", err.Error(), "messageType", messageType)

			retryCount++

			if retryCount >= config.RetryAttempt {
				return nil, fmt.Errorf("reached retry limit for receiving messages: %w", err)
			}
		case messageType != websocket.TextMessage && messageType != websocket.BinaryMessage:
			// We only accept text messages which are interpreted as UTF-8 or binary encoded text.
			return nil, fmt.Errorf("invalid message type: %w", err)
		default:
			return rawMessage, nil
		}
	}
}

// startPings starts the pinging process to keep the websocket channel alive.
func (c *WebSocketChannel) startPings(pingInterval time.Duration) {
	for {
		if !c.isOpen.Load() {
			return
		}

		if err := c.ping(); err != nil {
			c.logger.Error(err.Error())
		}

		time.Sleep(pingInterval)
	}
}

func (c *WebSocketChannel) ping() error {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	c.logger.Debug("WebsocketChannel: Sending ping")

	if err := c.connection.WriteMessage(websocket.PingMessage, []byte("keepalive")); err != nil {
		return fmt.Errorf("sending ping message: %w", err)
	}

	return nil
}
