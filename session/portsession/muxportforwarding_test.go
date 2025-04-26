// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	"net"
	"testing"
	"time"

	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// test readStream.
func TestReadStream(t *testing.T) {
	t.Parallel()

	out, in := net.Pipe()
	defer func() {
		if err := out.Close(); err != nil {
			t.Errorf("Failed to close out: %v", err)
		}
	}()

	mockLogger := log.NewMockLog()
	mockWsChannel := getMockWsChannel()
	outputMessage := getMockOutputMessage()

	session := *getSessionMock(t, mockWsChannel)

	portSession := PortSession{
		session: &session,
		portSessionType: &MuxPortForwarding{
			session:   &session,
			muxClient: &MuxClient{in, nil},
			mgsConn:   &MgsConn{nil, out},
			logger:    mockLogger,
		},
		logger: mockLogger,
	}

	go func() {
		if _, err := in.Write(outputMessage.Payload); err != nil {
			t.Errorf("Failed to write to in: %v", err)

			return
		}

		if err := in.Close(); err != nil {
			t.Errorf("Failed to close in: %v", err)
		}
	}()

	var actualPayload []byte

	// Mock SendMessage on the wsChannel
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything).Return(func(input []byte, _ int) error {
		actualPayload = input

		return nil
	})

	go func() {
		if err := portSession.portSessionType.ReadStream(context.TODO()); err != nil {
			t.Errorf("Failed to read stream: %v", err)
		}
	}()

	time.Sleep(time.Second)

	deserializedMsg := &message.ClientMessage{}
	err := deserializedMsg.DeserializeClientMessage(actualPayload)
	require.NoError(t, err)
	assert.Equal(t, outputMessage.Payload, deserializedMsg.Payload)
}

// test writeStream.
func TestWriteStream(t *testing.T) {
	t.Parallel()

	out, in := net.Pipe()
	defer func() {
		if err := in.Close(); err != nil {
			t.Errorf("Failed to close in: %v", err)
		}

		if err := out.Close(); err != nil {
			t.Errorf("Failed to close out: %v", err)
		}
	}()

	mockLogger := log.NewMockLog()
	mockWsChannel := getMockWsChannel()
	outputMessage := getMockOutputMessage()

	sess := *getSessionMock(t, mockWsChannel)
	portSession := PortSession{
		portSessionType: &MuxPortForwarding{
			session: &sess,
			mgsConn: &MgsConn{nil, in},
			logger:  mockLogger,
		},
		logger: mockLogger,
	}

	go func() {
		if err := portSession.portSessionType.WriteStream(outputMessage); err != nil {
			t.Errorf("Failed to write stream: %v", err)
		}
	}()

	msg := make([]byte, 20)

	n, err := out.Read(msg)
	if err != nil {
		t.Errorf("Failed to read from out: %v", err)

		return
	}

	msg = msg[:n]

	assert.Equal(t, outputMessage.Payload, msg)
}

// Test handleDataTransfer.
func TestHandleDataTransferSrcToDst(t *testing.T) {
	t.Parallel()

	outputMessage := getMockOutputMessage()

	msg := make([]byte, 20)
	out, in := net.Pipe()
	out1, in1 := net.Pipe()

	defer func() {
		if err := out1.Close(); err != nil {
			t.Errorf("Failed to close out1: %v", err)
		}
	}()

	go func() {
		if _, err := in.Write(outputMessage.Payload); err != nil {
			t.Errorf("Failed to write to in: %v", err)

			return
		}

		if err := in.Close(); err != nil {
			t.Errorf("Failed to close in: %v", err)
		}
	}()
	go func() {
		n, err := out1.Read(msg)
		if err != nil {
			t.Errorf("Failed to read from out1: %v", err)

			return
		}

		msg = msg[:n]
	}()

	handleDataTransfer(in1, out)
	assert.Equal(t, outputMessage.Payload, msg)
}

func TestHandleDataTransferDstToSrc(t *testing.T) {
	t.Parallel()

	outputMessage := getMockOutputMessage()

	msg := make([]byte, 20)
	out, in := net.Pipe()
	out1, in1 := net.Pipe()

	defer func() {
		if err := out.Close(); err != nil {
			t.Errorf("Failed to close out: %v", err)
		}
	}()

	go func() {
		if _, err := in1.Write(outputMessage.Payload); err != nil {
			t.Errorf("Failed to write to in1: %v", err)

			return
		}

		if err := in1.Close(); err != nil {
			t.Errorf("Failed to close in1: %v", err)
		}
	}()
	go func() {
		n, err := out.Read(msg)
		if err != nil {
			t.Errorf("Failed to read from out: %v", err)

			return
		}

		msg = msg[:n]
	}()

	handleDataTransfer(in, out1)
	assert.Equal(t, outputMessage.Payload, msg)
}
