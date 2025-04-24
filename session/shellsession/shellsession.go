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

// Package shellsession starts shell session.
package shellsession

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"time"

	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
	"github.com/steveh/ecstoolkit/session"
	"github.com/steveh/ecstoolkit/session/sessionutil"
	"github.com/steveh/ecstoolkit/util"
	"golang.org/x/term"
)

const (
	// ResizeSleepInterval defines the interval between terminal size checks.
	ResizeSleepInterval = time.Millisecond * 500
	// StdinBufferLimit defines the maximum size of the standard input buffer.
	StdinBufferLimit = 1024
)

// ShellSession represents a shell session that handles terminal interactions.
type ShellSession struct {
	session.Session

	// SizeData is used to store size data at session level to compare with new size.
	SizeData          message.SizeData
	originalSttyState bytes.Buffer
	shutdown          context.CancelFunc
}

var _ session.ISessionPlugin = (*ShellSession)(nil)

// GetTerminalSizeCall is a function that retrieves the terminal dimensions.
var GetTerminalSizeCall = term.GetSize

// Name is the session name used in the plugin.
func (s *ShellSession) Name() string {
	return config.ShellPluginName
}

// Initialize sets up the shell session with the provided session parameters.
func (s *ShellSession) Initialize(ctx context.Context, _ log.T, sessionVar *session.Session) {
	s.Session = *sessionVar
	s.DataChannel.RegisterOutputStreamHandler(s.ProcessStreamMessagePayload, true)
	s.DataChannel.RegisterOutputMessageHandler(ctx, s.Stop, func(_ []byte) {})
}

// SetSessionHandlers sets up handlers for terminal input, resizing, and control signals.
func (s *ShellSession) SetSessionHandlers(ctx context.Context, log log.T) error {
	// handle re-size
	s.handleTerminalResize(log)

	// handle control signals
	s.handleControlSignals(log)

	// handles keyboard input
	return s.handleKeyboardInput(ctx, log)
}

// ProcessStreamMessagePayload prints payload received on datachannel to console.
func (s *ShellSession) ProcessStreamMessagePayload(log log.T, outputMessage message.ClientMessage) (bool, error) {
	s.DisplayMode.DisplayMessage(log, outputMessage)

	return true, nil
}

// handleControlSignals handles control signals when given by user.
func (s *ShellSession) handleControlSignals(log log.T) {
	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, sessionutil.ControlSignals...)

		for {
			sig := <-signals
			if b, ok := sessionutil.SignalsByteMap[sig]; ok {
				if err := s.DataChannel.SendInputDataMessage(message.Output, []byte{b}); err != nil {
					log.Error("sending control signals", "error", err)
				}
			}
		}
	}()
}

// handleTerminalResize checks size of terminal every 500ms and sends size data.
func (s *ShellSession) handleTerminalResize(log log.T) {
	go func() {
		for {
			width, height := terminalSize(log)

			if s.SizeData.Rows != height || s.SizeData.Cols != width {
				sizeData := message.SizeData{
					Cols: width,
					Rows: height,
				}
				s.SizeData = sizeData

				inputSizeData, err := json.Marshal(sizeData)
				if err != nil {
					log.Error("Cannot marshal size data", "error", err)
				}

				log.Debug("Sending input size data", "data", string(inputSizeData))

				if err = s.DataChannel.SendInputDataMessage(message.Size, inputSizeData); err != nil {
					log.Error("sending size data", "error", err)
				}
			}
			// repeating this loop for every 500ms
			time.Sleep(ResizeSleepInterval)
		}
	}()
}

// If running from IDE GetTerminalSizeCall will not work. Supply a fixed width and height value.
func terminalSize(log log.T) (uint32, uint32) {
	const (
		defaultWidth  = 300
		defaultHeight = 100
	)

	width, height, err := GetTerminalSizeCall(int(os.Stdin.Fd()))
	if err != nil {
		log.Error("Could not get size of terminal", "error", err, "width", width, "height", height)

		return defaultWidth, defaultHeight
	}

	safeWidth, err := util.SafeUint32(width)
	if err != nil {
		log.Error("Could not convert width to uint32", "error", err, "width", width)

		return defaultWidth, defaultHeight
	}

	safeHeight, err := util.SafeUint32(height)
	if err != nil {
		log.Error("Could not convert height to uint32", "error", err, "height", height)

		return defaultWidth, defaultHeight
	}

	if safeWidth == 0 || safeHeight == 0 {
		log.Error("Terminal size is zero", "width", safeWidth, "height", safeHeight)

		return defaultWidth, defaultHeight
	}

	return safeWidth, safeHeight
}
