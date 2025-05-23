// Package message defines data channel messages structure.
package message

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Error messages.
var (
	ErrOffsetOutside  = errors.New("offset outside")
	ErrNotEnoughSpace = errors.New("not enough space")
	ErrNegative       = errors.New("cannot convert negative number to unsigned")
	ErrNil            = errors.New("input is nil")
	ErrInvalid        = errors.New("invalid input")
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
func SerializeClientMessageWithAcknowledgeContent(acknowledgeContent AcknowledgeContent) ([]byte, error) {
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

	reply, err := m.SerializeClientMessage()
	if err != nil {
		return nil, fmt.Errorf("serializing acknowledge content: %w", err)
	}

	return reply, nil
}
