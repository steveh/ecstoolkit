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
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/steveh/ecstoolkit/datachannel"
	"github.com/steveh/ecstoolkit/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test StartSession.
func TestStartSessionForStandardStreamForwarding(t *testing.T) {
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

	if _, err := out.Write(getMockOutputMessage().Payload); err != nil {
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

	mockWsChannel := getMockWsChannel()

	sess := *getSessionMock(t, mockWsChannel)
	portSession := PortSession{
		session:        &sess,
		portParameters: PortParameters{PortNumber: "22"},
		portSessionType: &StandardStreamForwarding{
			session:        &sess,
			portParameters: PortParameters{PortNumber: "22"},
			logger:         getMockLogger(),
		},
		logger: getMockLogger(),
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
	err = deserializedMsg.DeserializeClientMessage(actualPayload)
	require.NoError(t, err)
	assert.Equal(t, getMockOutputMessage().Payload, deserializedMsg.Payload)
}
