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
	"golang.org/x/term"
)

const (
	ResizeSleepInterval = time.Millisecond * 500
	StdinBufferLimit    = 1024
)

type ShellSession struct {
	session.Session

	// SizeData is used to store size data at session level to compare with new size.
	SizeData          message.SizeData
	originalSttyState bytes.Buffer
	shutdown          context.CancelFunc
	log               log.T
}

var _ session.ISessionPlugin = (*ShellSession)(nil)

var GetTerminalSizeCall = func(fd int) (width int, height int, err error) {
	return term.GetSize(fd)
}

// Name is the session name used in the plugin.
func (ShellSession) Name() string {
	return config.ShellPluginName
}

func (s *ShellSession) Initialize(ctx context.Context, log log.T, sessionVar *session.Session) {
	s.Session = *sessionVar
	s.DataChannel.RegisterOutputStreamHandler(s.ProcessStreamMessagePayload, true)
	s.DataChannel.GetWsChannel().SetOnMessage(
		func(input []byte) {
			s.DataChannel.OutputMessageHandler(ctx, log, s.Stop, s.SessionId, input)
		})
}

// StartSession takes input and write it to data channel.
func (s *ShellSession) SetSessionHandlers(ctx context.Context, log log.T) error {
	// handle re-size
	s.handleTerminalResize(log)

	// handle control signals
	s.handleControlSignals(log)

	// handles keyboard input
	return s.handleKeyboardInput(ctx, log)
}

// handleControlSignals handles control signals when given by user.
func (s *ShellSession) handleControlSignals(log log.T) {
	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, sessionutil.ControlSignals...)

		for {
			sig := <-signals
			if b, ok := sessionutil.SignalsByteMap[sig]; ok {
				if err := s.DataChannel.SendInputDataMessage(log, message.Output, []byte{b}); err != nil {
					log.Error("Failed to send control signals", "error", err)
				}
			}
		}
	}()
}

// handleTerminalResize checks size of terminal every 500ms and sends size data.
func (s *ShellSession) handleTerminalResize(log log.T) {
	var (
		width         int
		height        int
		inputSizeData []byte
		err           error
	)

	go func() {
		for {
			// If running from IDE GetTerminalSizeCall will not work. Supply a fixed width and height value.
			if width, height, err = GetTerminalSizeCall(int(os.Stdout.Fd())); err != nil {
				width = 300
				height = 100
				log.Error("Could not get size of terminal", "error", err, "width", width, "height", height)
			}

			if s.SizeData.Rows != uint32(height) || s.SizeData.Cols != uint32(width) {
				sizeData := message.SizeData{
					Cols: uint32(width),
					Rows: uint32(height),
				}
				s.SizeData = sizeData

				if inputSizeData, err = json.Marshal(sizeData); err != nil {
					log.Error("Cannot marshal size data", "error", err)
				}

				log.Debug("Sending input size data", "data", string(inputSizeData))

				if err = s.DataChannel.SendInputDataMessage(log, message.Size, inputSizeData); err != nil {
					log.Error("Failed to send size data", "error", err)
				}
			}
			// repeating this loop for every 500ms
			time.Sleep(ResizeSleepInterval)
		}
	}()
}

// ProcessStreamMessagePayload prints payload received on datachannel to console.
func (s ShellSession) ProcessStreamMessagePayload(log log.T, outputMessage message.ClientMessage) (isHandlerReady bool, err error) {
	s.DisplayMode.DisplayMessage(log, outputMessage)

	return true, nil
}
