// Package shellsession starts shell session.
package shellsession

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"time"

	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
	"github.com/steveh/ecstoolkit/session"
	"github.com/steveh/ecstoolkit/session/sessionutil"
	"github.com/steveh/ecstoolkit/util"
	"golang.org/x/sync/errgroup"
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
	session session.ISessionTypeSupport

	// SizeData is used to store size data at session level to compare with new size.
	SizeData          message.SizeData
	originalSttyState bytes.Buffer
	shutdown          context.CancelFunc
	logger            log.T
	terminalSizer     TerminalSizer
}

var _ session.ISessionPlugin = (*ShellSession)(nil)

// TerminalSizer is a function type that retrieves the size of the terminal.
type TerminalSizer = func(fd int) (width, height int, err error)

// NewShellSession creates a new shell session.
func NewShellSession(logger log.T, sess session.ISessionSupport) (*ShellSession, error) {
	s := &ShellSession{
		session:       sess,
		logger:        logger.With("subsystem", "ShellSession"),
		terminalSizer: term.GetSize,
	}

	sess.RegisterOutputStreamHandler(s.ProcessStreamMessagePayload, true)

	sess.RegisterStopHandler(s.Stop)

	sess.RegisterIncomingMessageHandler(func(_ []byte) {})

	return s, nil
}

// Name is the session name used in the plugin.
func (s *ShellSession) Name() string {
	return config.ShellPluginName
}

// SetSessionHandlers sets up handlers for terminal input, resizing, and control signals.
func (s *ShellSession) SetSessionHandlers(ctx context.Context) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	s.shutdown = cancel

	// handle double echo and disable input buffering
	if err := s.disableEchoAndInputBuffering(); err != nil {
		return fmt.Errorf("disabling echo and input buffering: %w", err)
	}

	defer func() {
		if enableErr := s.enableEchoAndInputBuffering(); enableErr != nil && err == nil {
			err = fmt.Errorf("enabling echo and input buffering: %w", enableErr)
		}
	}()

	eg, ctx := errgroup.WithContext(ctx)

	// handle re-size
	eg.Go(func() error {
		return s.handleTerminalResize(ctx)
	})

	// handle control signals
	eg.Go(func() error {
		return s.HandleControlSignals(ctx)
	})

	// handles keyboard input
	eg.Go(func() error {
		return s.handleKeyboardInput(ctx)
	})

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("waiting for goroutines: %w", err)
	}

	return nil
}

// ProcessStreamMessagePayload prints payload received on datachannel to console.
func (s *ShellSession) ProcessStreamMessagePayload(outputMessage message.ClientMessage) (bool, error) {
	s.session.DisplayMessage(outputMessage)

	return true, nil
}

// Stop restores the terminal settings and exits.
func (s *ShellSession) Stop() error {
	// Must be closed to avoid errors.
	if err := s.session.Close(); err != nil {
		return fmt.Errorf("closing DataChannel: %w", err)
	}

	s.shutdown()

	return nil
}

// HandleControlSignals handles control signals when given by user.
func (s *ShellSession) HandleControlSignals(ctx context.Context) error {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, sessionutil.ControlSignals...)

	for {
		select {
		case <-ctx.Done():
			return nil
		case sig := <-signals:
			if b, ok := sessionutil.SignalsByteMap[sig]; ok {
				if err := s.session.SendInputDataMessage(message.Output, []byte{b}); err != nil {
					return fmt.Errorf("sending control signal: %w", err)
				}
			}
		}
	}
}

// handleTerminalResize checks size of terminal every 500ms and sends size data.
func (s *ShellSession) handleTerminalResize(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			width, height := s.getTerminalSize()

			if s.SizeData.Rows != height || s.SizeData.Cols != width {
				sizeData := message.SizeData{
					Cols: width,
					Rows: height,
				}

				s.SizeData = sizeData

				inputSizeData, err := json.Marshal(sizeData)
				if err != nil {
					return fmt.Errorf("marshalling size data: %w", err)
				}

				s.logger.Debug("Sending input size data", "data", string(inputSizeData))

				if err = s.session.SendInputDataMessage(message.Size, inputSizeData); err != nil {
					return fmt.Errorf("sending size data: %w", err)
				}
			}

			time.Sleep(ResizeSleepInterval)
		}
	}
}

// If running from IDE GetTerminalSizeCall will not work. Supply a fixed width and height value.
func (s *ShellSession) getTerminalSize() (uint32, uint32) {
	const (
		defaultWidth  = 300
		defaultHeight = 100
	)

	width, height, err := s.terminalSizer(int(os.Stdin.Fd()))
	if err != nil {
		s.logger.Error("Could not get size of terminal", "error", err, "width", width, "height", height)

		return defaultWidth, defaultHeight
	}

	safeWidth, err := util.SafeUint32(width)
	if err != nil {
		s.logger.Error("Could not convert width to uint32", "error", err, "width", width)

		return defaultWidth, defaultHeight
	}

	safeHeight, err := util.SafeUint32(height)
	if err != nil {
		s.logger.Error("Could not convert height to uint32", "error", err, "height", height)

		return defaultWidth, defaultHeight
	}

	if safeWidth == 0 || safeHeight == 0 {
		s.logger.Error("Terminal size is zero", "width", safeWidth, "height", safeHeight)

		return defaultWidth, defaultHeight
	}

	return safeWidth, safeHeight
}

// disableEchoAndInputBuffering disables echo to avoid double echo and disable input buffering.
func (s *ShellSession) disableEchoAndInputBuffering() error {
	if err := getState(&s.originalSttyState); err != nil {
		return fmt.Errorf("getting terminal state: %w", err)
	}

	if err := setState(bytes.NewBufferString("cbreak")); err != nil {
		return fmt.Errorf("setting cbreak mode: %w", err)
	}

	if err := setState(bytes.NewBufferString("-echo")); err != nil {
		return fmt.Errorf("disabling echo: %w", err)
	}

	return nil
}

func (s *ShellSession) enableEchoAndInputBuffering() error {
	if err := setState(&s.originalSttyState); err != nil {
		return fmt.Errorf("restoring original terminal state: %w", err)
	}

	if err := setState(bytes.NewBufferString("echo")); err != nil { // for linux
		return fmt.Errorf("enabling echo: %w", err)
	}

	return nil
}

// handleKeyboardInput handles input entered by customer on terminal.
func (s *ShellSession) handleKeyboardInput(ctx context.Context) error {
	stdinBytes := make([]byte, StdinBufferLimit)
	reader := newContextReader(ctx, bufio.NewReader(os.Stdin))

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			stdinBytesLen, err := reader.Read(stdinBytes)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}

				return fmt.Errorf("reading from stdin: %w", err)
			}

			if err := s.session.SendInputDataMessage(message.Output, stdinBytes[:stdinBytesLen]); err != nil {
				return fmt.Errorf("sending input data: %w", err)
			}

			// sleep to limit the rate of data transfer
			time.Sleep(time.Millisecond)
		}
	}
}

// getState gets current state of terminal.
func getState(state *bytes.Buffer) error {
	cmd := exec.Command("stty", "-g")
	cmd.Stdin = os.Stdin
	cmd.Stdout = state

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("getting terminal state: %w", err)
	}

	return nil
}

// setState sets the new settings to terminal.
func setState(state *bytes.Buffer) error {
	cmd := exec.Command("stty", state.String()) //nolint:gosec
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("setting terminal state: %w", err)
	}

	return nil
}
