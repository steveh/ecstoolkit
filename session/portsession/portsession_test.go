// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package portsession starts port session.
package portsession

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/steveh/ecstoolkit/datachannel"
	"github.com/steveh/ecstoolkit/jsonutil"
	"github.com/steveh/ecstoolkit/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Test Initialize.
func TestInitializePortSession(t *testing.T) {
	var portParameters PortParameters

	if err := jsonutil.Remarshal(properties, &portParameters); err != nil {
		t.Errorf("Failed to remarshal properties: %v", err)

		return
	}

	mockWebSocketChannel.On("SetOnMessage", mock.Anything)

	sess := getSessionMock(t)
	portSession, err := NewPortSession(context.TODO(), mockLog, sess)
	require.NoError(t, err, "Initialize port session")

	mockWebSocketChannel.AssertExpectations(t)
	assert.Equal(t, portParameters, portSession.portParameters, "Initialize port parameters")
	assert.IsType(t, &StandardStreamForwarding{}, portSession.portSessionType)
}

func TestInitializePortSessionForPortForwardingWithOldAgent(t *testing.T) {
	var portParameters PortParameters

	if err := jsonutil.Remarshal(map[string]interface{}{"portNumber": "8080", "type": "LocalPortForwarding"}, &portParameters); err != nil {
		t.Errorf("Failed to remarshal properties: %v", err)

		return
	}

	mockWebSocketChannel.On("SetOnMessage", mock.Anything)

	sess := getSessionMockWithParams(t, portParameters, "2.2.0.0")
	portSession, err := NewPortSession(context.TODO(), mockLog, sess)
	require.NoError(t, err, "Initialize port session")

	mockWebSocketChannel.AssertExpectations(t)
	assert.Equal(t, portParameters, portSession.portParameters, "Initialize port parameters")
	assert.IsType(t, &BasicPortForwarding{}, portSession.portSessionType)
}

func TestInitializePortSessionForPortForwarding(t *testing.T) {
	var portParameters PortParameters

	if err := jsonutil.Remarshal(map[string]interface{}{"portNumber": "8080", "type": "LocalPortForwarding"}, &portParameters); err != nil {
		t.Errorf("Failed to remarshal properties: %v", err)

		return
	}

	mockWebSocketChannel.On("SetOnMessage", mock.Anything)

	sess := getSessionMockWithParams(t, portParameters, "3.1.0.0")
	portSession, err := NewPortSession(context.TODO(), mockLog, sess)
	require.NoError(t, err, "Initialize port session")

	mockWebSocketChannel.AssertExpectations(t)
	assert.Equal(t, portParameters, portSession.portParameters, "Initialize port parameters")
	assert.IsType(t, &MuxPortForwarding{}, portSession.portSessionType)
}

func TestStartSessionWithClosedWsConn(t *testing.T) {
	in, out, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	// Ensure cleanup
	defer func() {
		if err := in.Close(); err != nil {
			t.Logf("Error closing in: %v", err)
		}

		if err := out.Close(); err != nil {
			t.Logf("Error closing out: %v", err)
		}
		// Restore original stdin
	}()

	if _, err := out.Write(outputMessage.Payload); err != nil {
		t.Errorf("Failed to write to out: %v", err)

		return
	}

	os.Stdin = in

	var actualPayload []byte

	done := make(chan struct{})
	errChan := make(chan error, 1)

	datachannel.SendMessageCall = func(_ *datachannel.DataChannel, input []byte, _ int) error {
		actualPayload = input

		close(done)

		return nil
	}

	sess := *getSessionMock(t)
	portSession := PortSession{
		Session:        sess,
		portParameters: PortParameters{PortNumber: "22"},
		portSessionType: &StandardStreamForwarding{
			inputStream:  in,
			outputStream: out,
			session:      &sess,
			logger:       mockLog,
		},
		logger: mockLog,
	}

	// Start session handlers in a goroutine
	go func() {
		if err := portSession.SetSessionHandlers(context.TODO()); err != nil {
			errChan <- fmt.Errorf("failed to set session handlers: %w", err)

			return
		}
	}()

	// Wait for message to be processed or timeout
	select {
	case <-done:
		// Message processed successfully
	case err := <-errChan:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message processing")
	}

	deserializedMsg := &message.ClientMessage{}
	err = deserializedMsg.DeserializeClientMessage(mockLog, actualPayload)
	require.NoError(t, err)
	assert.Equal(t, outputMessage.Payload, deserializedMsg.Payload)
}

// Test ProcessStreamMessagePayload.
func TestProcessStreamMessagePayload(t *testing.T) {
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create output pipe: %v", err)
	}

	// Create a dummy input pipe since the method requires it
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create input pipe: %v", err)
	}

	// Create a StandardStreamForwarding instance with our test streams
	ssf := &StandardStreamForwarding{
		inputStream:  inR, // Required to pass IsStreamNotSet check
		outputStream: outW,
	}

	// Create channels for synchronization
	done := make(chan struct{})
	errChan := make(chan error, 1)
	readyChan := make(chan struct{})

	var payload []byte

	session := *getSessionMock(t)

	go func() {
		portSession := PortSession{
			Session:         session,
			portParameters:  PortParameters{PortNumber: "22"},
			portSessionType: ssf,
			logger:          mockLog,
		}

		t.Log("Calling ProcessStreamMessagePayload")

		isReady, err := portSession.ProcessStreamMessagePayload(outputMessage)
		if err != nil {
			errChan <- fmt.Errorf("failed to process stream message payload: %w", err)

			return
		}

		if !isReady {
			errChan <- errors.New("handler was not ready")

			return
		}

		t.Log("ProcessStreamMessagePayload succeeded")

		// Signal that we're ready to read
		close(readyChan)

		// Read the payload from the output pipe
		t.Log("Reading from output pipe")

		payload, err = io.ReadAll(outR)
		if err != nil {
			errChan <- fmt.Errorf("failed to read from output pipe: %w", err)

			return
		}

		t.Log("Successfully read from output pipe")
		close(done)
	}()

	// Close pipes when we're done
	defer func() {
		if err := inR.Close(); err != nil {
			t.Logf("Error closing inR: %v", err)
		}

		if err := inW.Close(); err != nil {
			t.Logf("Error closing inW: %v", err)
		}

		if err := outR.Close(); err != nil {
			t.Logf("Error closing outR: %v", err)
		}

		if err := outW.Close(); err != nil {
			t.Logf("Error closing outW: %v", err)
		}
	}()

	// Wait for the goroutine to be ready to read
	select {
	case <-readyChan:
		// Ready to read
	case err := <-errChan:
		t.Fatal(err)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for handler to be ready")
	}

	// Close the write end of the pipe after writing
	if err := outW.Close(); err != nil {
		t.Fatalf("Failed to close output pipe: %v", err)
	}

	// Wait for processing to complete or timeout
	select {
	case <-done:
		t.Log("Test completed successfully")
		assert.Equal(t, outputMessage.Payload, payload)
	case err := <-errChan:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for stream processing")
	}
}
