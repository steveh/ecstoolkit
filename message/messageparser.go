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

// Package message defines data channel messages structure.
package message

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/steveh/ecstoolkit/log"
)

// Error messages.
var (
	ErrOffsetOutside          = errors.New("offset outside")
	ErrNotEnoughSpace         = errors.New("not enough space")
	ErrOffsetOutsideByteArray = errors.New("offset outside byte array")
)

// SerializeClientMessagePayload marshals payloads for all session specific messages into bytes.
func SerializeClientMessagePayload(obj any) ([]byte, error) {
	reply, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("marshaling message: %w", err)
	}

	return reply, nil
}

// SerializeClientMessageWithAcknowledgeContent marshals client message with payloads of acknowledge contents into bytes.
func SerializeClientMessageWithAcknowledgeContent(log log.T, acknowledgeContent AcknowledgeContent) ([]byte, error) {
	acknowledgeContentBytes, err := SerializeClientMessagePayload(acknowledgeContent)
	if err != nil {
		return nil, fmt.Errorf("serializing acknowledge content payload: %w: %+v", err, acknowledgeContent)
	}

	messageID := uuid.New()
	m := ClientMessage{
		MessageType:    AcknowledgeMessage,
		SchemaVersion:  1,
		CreatedDate:    uint64(time.Now().UnixMilli()), //nolint:gosec
		SequenceNumber: 0,
		Flags:          3, //nolint:mnd
		MessageID:      messageID,
		Payload:        acknowledgeContentBytes,
	}

	reply, err := m.SerializeClientMessage(log)
	if err != nil {
		return nil, fmt.Errorf("serializing acknowledge content: %w", err)
	}

	return reply, nil
}
