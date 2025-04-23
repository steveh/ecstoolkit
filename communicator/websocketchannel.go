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
	Initialize(log log.T, channelURL string, channelToken string)
	Open(log log.T) error
	Close(log log.T) error
	SendMessage(input []byte, inputType int) error
	StartPings(log log.T, pingInterval time.Duration)
	GetChannelToken() string
	GetStreamURL() string
	SetChannelToken(channelToken string)
	SetOnError(onErrorHandler func(error))
	SetOnMessage(onMessageHandler func([]byte))
}

// WebSocketChannel parent class for DataChannel.
type WebSocketChannel struct {
	IWebSocketChannel
	URL          string
	OnMessage    func([]byte)
	OnError      func(error)
	IsOpen       bool
	writeLock    *sync.Mutex
	Connection   *websocket.Conn
	ChannelToken string
}

// GetChannelToken gets the channel token.
func (webSocketChannel *WebSocketChannel) GetChannelToken() string {
	return webSocketChannel.ChannelToken
}

// SetChannelToken sets the channel token.
func (webSocketChannel *WebSocketChannel) SetChannelToken(channelToken string) {
	webSocketChannel.ChannelToken = channelToken
}

// GetStreamURL gets stream url.
func (webSocketChannel *WebSocketChannel) GetStreamURL() string {
	return webSocketChannel.URL
}

// SetOnError sets OnError field of websocket channel.
func (webSocketChannel *WebSocketChannel) SetOnError(onErrorHandler func(error)) {
	webSocketChannel.OnError = onErrorHandler
}

// SetOnMessage sets OnMessage field of websocket channel.
func (webSocketChannel *WebSocketChannel) SetOnMessage(onMessageHandler func([]byte)) {
	webSocketChannel.OnMessage = onMessageHandler
}

// Initialize initializes websocket channel fields.
func (webSocketChannel *WebSocketChannel) Initialize(_ log.T, channelURL string, channelToken string) {
	webSocketChannel.ChannelToken = channelToken
	webSocketChannel.URL = channelURL
}

// StartPings starts the pinging process to keep the websocket channel alive.
func (webSocketChannel *WebSocketChannel) StartPings(log log.T, pingInterval time.Duration) {
	go func() {
		for {
			if !webSocketChannel.IsOpen {
				return
			}

			log.Debug("WebsocketChannel: Send ping. Message.")
			webSocketChannel.writeLock.Lock()
			err := webSocketChannel.Connection.WriteMessage(websocket.PingMessage, []byte("keepalive"))
			webSocketChannel.writeLock.Unlock()

			if err != nil {
				log.Error("Error sending websocket ping", "error", err)

				return
			}

			time.Sleep(pingInterval)
		}
	}()
}

// SendMessage sends a byte message through the websocket connection.
// Examples of message type are websocket.TextMessage or websocket.Binary.
func (webSocketChannel *WebSocketChannel) SendMessage(input []byte, inputType int) error {
	if !webSocketChannel.IsOpen {
		return errors.New("connection is closed")
	}

	if len(input) < 1 {
		return errors.New("empty input")
	}

	webSocketChannel.writeLock.Lock()
	err := webSocketChannel.Connection.WriteMessage(inputType, input)
	webSocketChannel.writeLock.Unlock()

	if err != nil {
		return fmt.Errorf("writing websocket message: %w", err)
	}

	return nil
}

// Close closes the corresponding connection.
func (webSocketChannel *WebSocketChannel) Close(log log.T) error {
	log.Debug("Closing websocket channel connection to: " + webSocketChannel.URL)

	if webSocketChannel.IsOpen {
		// Send signal to stop receiving message
		webSocketChannel.IsOpen = false

		if err := websocketutil.NewWebsocketUtil(log, nil).CloseConnection(webSocketChannel.Connection); err != nil {
			return fmt.Errorf("closing websocket connection: %w", err)
		}

		return nil
	}

	log.Warn("Websocket channel connection to: " + webSocketChannel.URL + " is already Closed!")

	return nil
}

// Open upgrades the http connection to a websocket connection.
func (webSocketChannel *WebSocketChannel) Open(log log.T) error {
	// initialize the write mutex
	webSocketChannel.writeLock = &sync.Mutex{}

	ws, err := websocketutil.NewWebsocketUtil(log, nil).OpenConnection(webSocketChannel.URL)
	if err != nil {
		return fmt.Errorf("opening websocket connection: %w", err)
	}

	webSocketChannel.Connection = ws
	webSocketChannel.IsOpen = true
	webSocketChannel.StartPings(log, config.PingTimeInterval)

	// spin up a different routine to listen to the incoming traffic
	go func() {
		defer func() {
			if msg := recover(); msg != nil {
				log.Error("WebsocketChannel listener run panic", "error", msg)
			}
		}()

		retryCount := 0

		for {
			if !webSocketChannel.IsOpen {
				log.Debug("Ending the channel listening routine since the channel is closed", "url", webSocketChannel.URL)

				break
			}

			messageType, rawMessage, err := webSocketChannel.Connection.ReadMessage()

			switch {
			case err != nil:
				retryCount++
				if retryCount >= config.RetryAttempt {
					log.Error("Reached retry limit for receiving messages", "retryLimit", config.RetryAttempt)
					webSocketChannel.OnError(err)

					break
				}

				log.Warn("Error receiving message", "retryCount", retryCount, "error", err.Error(), "messageType", messageType)
			case messageType != websocket.TextMessage && messageType != websocket.BinaryMessage:
				// We only accept text messages which are interpreted as UTF-8 or binary encoded text.
				log.Error("Invalid message type", "messageType", messageType, "reason", "Only UTF-8 or binary encoded text accepted")
			default:
				retryCount = 0

				webSocketChannel.OnMessage(rawMessage)
			}
		}
	}()

	return nil
}
