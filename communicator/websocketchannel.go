// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package communicator implements base communicator for network connections.
package communicator

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/websocketutil"
)

// IWebSocketChannel is the interface for DataChannel.
type IWebSocketChannel interface {
	Open(log log.T) error
	Close(log log.T) error
	SendMessage(input []byte, inputType int) error
	GetChannelToken() string
	GetStreamURL() string
	SetChannelToken(channelToken string)
	SetOnError(onErrorHandler func(error))
	SetOnMessage(onMessageHandler func([]byte))
}

// WebSocketChannel parent class for DataChannel.
type WebSocketChannel struct {
	IWebSocketChannel
	OnMessage    func([]byte)
	OnError      func(error)
	channelURL   string
	isOpen       bool
	writeLock    *sync.Mutex
	connection   *websocket.Conn
	channelToken string
}

// NewWebSocketChannel creates a WebSocketChannel.
func NewWebSocketChannel(channelURL string, channelToken string) (*WebSocketChannel, error) {
	return &WebSocketChannel{
		channelToken: channelToken,
		channelURL:   channelURL,
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

// SetOnError sets OnError field of websocket channel.
func (c *WebSocketChannel) SetOnError(onErrorHandler func(error)) {
	c.OnError = onErrorHandler
}

// SetOnMessage sets OnMessage field of websocket channel.
func (c *WebSocketChannel) SetOnMessage(onMessageHandler func([]byte)) {
	c.OnMessage = onMessageHandler
}

// SendMessage sends a byte message through the websocket connection.
// Examples of message type are websocket.TextMessage or websocket.Binary.
func (c *WebSocketChannel) SendMessage(input []byte, inputType int) error {
	if !c.isOpen {
		return errors.New("connection is closed")
	}

	if len(input) < 1 {
		return errors.New("empty input")
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
func (c *WebSocketChannel) Close(log log.T) error {
	log.Debug("Closing websocket channel connection", "url", c.channelURL)

	if c.isOpen {
		// Send signal to stop receiving message
		c.isOpen = false

		if err := websocketutil.NewWebsocketUtil(log, nil).CloseConnection(c.connection); err != nil {
			return fmt.Errorf("closing websocket connection: %w", err)
		}

		return nil
	}

	log.Warn("Websocket channel connection is already Closed!", "url", c.channelURL)

	return nil
}

// Open upgrades the http connection to a websocket connection.
func (c *WebSocketChannel) Open(log log.T) error {
	// initialize the write mutex
	c.writeLock = &sync.Mutex{}

	ws, err := websocketutil.NewWebsocketUtil(log, nil).OpenConnection(c.channelURL)
	if err != nil {
		return fmt.Errorf("opening websocket connection: %w", err)
	}

	c.connection = ws
	c.isOpen = true
	c.startPings(log, config.PingTimeInterval)

	// spin up a different routine to listen to the incoming traffic
	go func() {
		defer func() {
			if msg := recover(); msg != nil {
				log.Error("WebsocketChannel listener run panic", "error", msg)
			}
		}()

		retryCount := 0

		for {
			if !c.isOpen {
				log.Debug("Ending the channel listening routine since the channel is closed", "url", c.channelURL)

				break
			}

			messageType, rawMessage, err := c.connection.ReadMessage()

			switch {
			case err != nil:
				retryCount++
				if retryCount >= config.RetryAttempt {
					log.Error("Reached retry limit for receiving messages", "retryLimit", config.RetryAttempt)
					c.OnError(err)

					break
				}

				log.Warn("Error receiving message", "retryCount", retryCount, "error", err.Error(), "messageType", messageType)
			case messageType != websocket.TextMessage && messageType != websocket.BinaryMessage:
				// We only accept text messages which are interpreted as UTF-8 or binary encoded text.
				log.Error("Invalid message type", "messageType", messageType, "reason", "Only UTF-8 or binary encoded text accepted")
			default:
				retryCount = 0

				c.OnMessage(rawMessage)
			}
		}
	}()

	return nil
}

// IsOpen checks if the channel is open.
func (c *WebSocketChannel) IsOpen() bool {
	return c.isOpen
}

// startPings starts the pinging process to keep the websocket channel alive.
func (c *WebSocketChannel) startPings(log log.T, pingInterval time.Duration) {
	go func() {
		for {
			if !c.isOpen {
				return
			}

			log.Debug("WebsocketChannel: Send ping. Message.")
			c.writeLock.Lock()
			err := c.connection.WriteMessage(websocket.PingMessage, []byte("keepalive"))
			c.writeLock.Unlock()

			if err != nil {
				log.Error("Error sending websocket ping", "error", err)

				return
			}

			time.Sleep(pingInterval)
		}
	}()
}
