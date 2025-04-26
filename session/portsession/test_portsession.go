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
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/steveh/ecstoolkit/communicator/mocks"
	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/datachannel"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
	"github.com/steveh/ecstoolkit/session"
	"github.com/stretchr/testify/require"
)

var (
	agentVersion         = "2.3.750.0"
	mockLog              = log.NewMockLog()
	mockWebSocketChannel = mocks.IWebSocketChannel{}
	outputMessage        = message.ClientMessage{
		PayloadType:   uint32(message.Output),
		Payload:       []byte("testing123"),
		PayloadLength: 10, //nolint:mnd
	}
	properties = map[string]any{
		"PortNumber": "22",
	}
)

func getSessionMock(t *testing.T) *session.Session {
	t.Helper()

	return getSessionMockWithParams(t, properties, agentVersion)
}

var mockLogger = log.NewMockLog()

func getSessionMockWithParams(t *testing.T, properties any, agentVersion string) *session.Session {
	t.Helper()

	mockKMSClient := &kms.Client{}

	dataChannel, err := datachannel.NewDataChannel(mockKMSClient, &mockWebSocketChannel, "clientId", "sessionId", "targetId", mockLogger)
	require.NoError(t, err)

	dataChannel.SetAgentVersion(agentVersion)

	actionParams := message.SessionTypeRequest{
		SessionType: config.PortPluginName,
		Properties:  properties,
	}
	b, err := json.Marshal(actionParams)
	require.NoError(t, err)
	err = dataChannel.ProcessSessionTypeHandshakeAction(b)
	require.NoError(t, err)

	mockSession, err := session.NewSession(nil, dataChannel, "", mockLogger)
	require.NoError(t, err)

	return mockSession
}
