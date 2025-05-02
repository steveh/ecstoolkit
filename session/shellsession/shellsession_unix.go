//go:build darwin || freebsd || linux || netbsd || openbsd
// +build darwin freebsd linux netbsd openbsd

// Package shellsession starts shell session.
package shellsession

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/steveh/ecstoolkit/message"
)

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

// Stop restores the terminal settings and exits.
func (s *ShellSession) Stop() error {
	// Must be closed to avoid errors.
	if err := s.session.Close(); err != nil {
		return fmt.Errorf("closing DataChannel: %w", err)
	}

	s.shutdown()

	return nil
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
	ctx, cancelFunc := context.WithCancel(ctx)
	s.shutdown = cancelFunc

	// handle double echo and disable input buffering
	if err := s.disableEchoAndInputBuffering(); err != nil {
		return fmt.Errorf("disabling echo and input buffering: %w", err)
	}

	defer func() {
		if enableErr := s.enableEchoAndInputBuffering(); enableErr != nil {
			s.logger.Error("Failed to enable echo and input buffering", "error", enableErr)
		}
	}()

	go func() {
		stdinBytes := make([]byte, StdinBufferLimit)
		reader := bufio.NewReader(os.Stdin)

		var err error

		var stdinBytesLen int

		for {
			if stdinBytesLen, err = reader.Read(stdinBytes); err != nil {
				s.logger.Error("Unable to read from Stdin", "error", err)

				break
			}

			if err = s.session.SendInputDataMessage(message.Output, stdinBytes[:stdinBytesLen]); err != nil {
				s.logger.Error("sending UTF8 char", "error", err)

				break
			}
			// sleep to limit the rate of data transfer
			time.Sleep(time.Millisecond)
		}

		s.shutdown()
	}()

	// Wait for context to be canceled by a call to Stop(), or stdin closing
	<-ctx.Done()

	if err := ctx.Err(); err != nil {
		return nil // not an error
	}

	return nil
}
