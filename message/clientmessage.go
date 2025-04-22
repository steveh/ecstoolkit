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
	"log/slog"

	"github.com/google/uuid"
)

const (
	// InputStreamMessage represents message type for input data.
	InputStreamMessage = "input_stream_data"

	// OutputStreamMessage represents message type for output data.
	OutputStreamMessage = "output_stream_data"

	// AcknowledgeMessage represents message type for acknowledge.
	AcknowledgeMessage = "acknowledge"

	// ChannelClosedMessage represents message type for ChannelClosed.
	ChannelClosedMessage = "channel_closed"

	// StartPublicationMessage represents the message type that notifies the CLI to start sending stream messages.
	StartPublicationMessage = "start_publication"

	// PausePublicationMessage represents the message type that notifies the CLI to pause sending stream messages
	// as the remote data channel is inactive.
	PausePublicationMessage = "pause_publication"
)

// AcknowledgeContent is used to inform the sender of an acknowledge message that the message has been received.
// * MessageType is a 32 byte UTF-8 string containing the message type.
// * MessageID is a 40 byte UTF-8 string containing the UUID identifying this message being acknowledged.
// * SequenceNumber is an 8 byte integer containing the message sequence number for serialized message.
// * IsSequentialMessage is a boolean field representing whether the acknowledged message is part of a sequence.
type AcknowledgeContent struct {
	MessageType         string `json:"AcknowledgedMessageType"`
	MessageID           string `json:"AcknowledgedMessageId"`
	SequenceNumber      int64  `json:"AcknowledgedMessageSequenceNumber"`
	IsSequentialMessage bool   `json:"IsSequentialMessage"`
}

// ChannelClosed is used to inform the client to close the channel
// * MessageID is a 40 byte UTF-8 string containing the UUID identifying this message.
// * CreatedDate is a string field containing the message create epoch millis in UTC.
// * DestinationID is a string field containing the session target.
// * SessionID is a string field representing which session to close.
// * MessageType is a 32 byte UTF-8 string containing the message type.
// * SchemaVersion is a 4 byte integer containing the message schema version number.
// * Output is a string field containing the error message for channel close.
type ChannelClosed struct {
	MessageID     string `json:"MessageId"`
	CreatedDate   string `json:"CreatedDate"`
	DestinationID string `json:"DestinationId"`
	SessionID     string `json:"SessionId"`
	MessageType   string `json:"MessageType"`
	SchemaVersion int    `json:"SchemaVersion"`
	Output        string `json:"Output"`
}

// PayloadType represents the type of payload in a client message.
type PayloadType uint32

const (
	// Output represents a standard output payload type.
	Output PayloadType = 1
	// Error represents an error payload type.
	Error PayloadType = 2
	// Size represents a size data payload type.
	Size PayloadType = 3
	// Parameter represents a parameter payload type.
	Parameter PayloadType = 4
	// HandshakeRequestPayloadType represents a handshake request payload type.
	HandshakeRequestPayloadType PayloadType = 5
	// HandshakeResponsePayloadType represents a handshake response payload type.
	HandshakeResponsePayloadType PayloadType = 6
	// HandshakeCompletePayloadType represents a handshake complete payload type.
	HandshakeCompletePayloadType PayloadType = 7
	// EncChallengeRequest is the payload type for encryption challenge requests.
	EncChallengeRequest PayloadType = 8
	// EncChallengeResponse is the payload type for encryption challenge responses.
	EncChallengeResponse PayloadType = 9
	// Flag is the payload type for flag messages.
	Flag PayloadType = 10
	// StdErr is the payload type for standard error messages.
	StdErr PayloadType = 11
	// ExitCode is the payload type for exit code messages.
	ExitCode PayloadType = 12
)

// PayloadTypeFlag represents flags that can be set in a client message payload.
type PayloadTypeFlag uint32

const (
	// DisconnectToPort indicates that the port connection should be disconnected.
	DisconnectToPort PayloadTypeFlag = 1
	// TerminateSession indicates that the session should be terminated.
	TerminateSession PayloadTypeFlag = 2
	// ConnectToPortError indicates that there was an error connecting to the port.
	ConnectToPortError PayloadTypeFlag = 3
)

// SizeData represents the size dimensions for terminal windows.
type SizeData struct {
	Cols uint32 `json:"cols"`
	Rows uint32 `json:"rows"`
}

// IClientMessage defines the interface for client message operations.
type IClientMessage interface {
	Validate() error
	DeserializeClientMessage(log *slog.Logger, input []byte) (err error)
	SerializeClientMessage(log *slog.Logger) (result []byte, err error)
	DeserializeDataStreamAcknowledgeContent(log *slog.Logger) (dataStreamAcknowledge AcknowledgeContent, err error)
	DeserializeChannelClosedMessage(log *slog.Logger) (channelClosed ChannelClosed, err error)
	DeserializeHandshakeRequest(log *slog.Logger) (handshakeRequest HandshakeRequestPayload, err error)
	DeserializeHandshakeComplete(log *slog.Logger) (handshakeComplete HandshakeCompletePayload, err error)
}

// ClientMessage represents a message for client to send/receive. ClientMessage Message in MGS is equivalent to MDS' InstanceMessage.
// All client messages are sent in this form to the MGS service.
type ClientMessage struct {
	HeaderLength   uint32
	MessageType    string
	SchemaVersion  uint32
	CreatedDate    uint64
	SequenceNumber int64
	Flags          uint64
	MessageID      uuid.UUID
	PayloadDigest  []byte
	PayloadType    uint32
	PayloadLength  uint32
	Payload        []byte
}

// * HL - HeaderLength is a 4 byte integer that represents the header length.
// * MessageType is a 32 byte UTF-8 string containing the message type.
// * SchemaVersion is a 4 byte integer containing the message schema version number.
// * CreatedDate is an 8 byte integer containing the message create epoch millis in UTC.
// * SequenceNumber is an 8 byte integer containing the message sequence number for serialized message streams.
// * Flags is an 8 byte unsigned integer containing a packed array of control flags:
// *   Bit 0 is SYN - SYN is set (1) when the recipient should consider Seq to be the first message number in the stream
// *   Bit 1 is FIN - FIN is set (1) when this message is the final message in the sequence.
// * MessageID is a 40 byte UTF-8 string containing a random UUID identifying this message.
// * Payload digest is a 32 byte containing the SHA-256 hash of the payload.
// * Payload length is an 4 byte unsigned integer containing the byte length of data in the Payload field.
// * Payload is a variable length byte data.
//
// * | HL|         MessageType           |Ver|  CD   |  Seq  | Flags |
// * |         MessageID                     |           Digest              | PayType | PayLen|
// * |         Payload      			|

const (
	// ClientMessageHLLength represents the length of the header length field in bytes.
	ClientMessageHLLength = 4
	// ClientMessageMessageTypeLength represents the length of the message type field in bytes.
	ClientMessageMessageTypeLength = 32
	// ClientMessageSchemaVersionLength represents the length of the schema version field in bytes.
	ClientMessageSchemaVersionLength = 4
	// ClientMessageCreatedDateLength represents the length of the created date field in bytes.
	ClientMessageCreatedDateLength = 8
	// ClientMessageSequenceNumberLength represents the length of the sequence number field in bytes.
	ClientMessageSequenceNumberLength = 8
	// ClientMessageFlagsLength represents the length of the flags field in bytes.
	ClientMessageFlagsLength = 8
	// ClientMessageMessageIDLength represents the length of the message ID field in bytes.
	ClientMessageMessageIDLength = 16
	// ClientMessagePayloadDigestLength represents the length of the payload digest field in bytes.
	ClientMessagePayloadDigestLength = 32
	// ClientMessagePayloadTypeLength represents the length of the payload type field in bytes.
	ClientMessagePayloadTypeLength = 4
	// ClientMessagePayloadLengthLength represents the length of the payload length field in bytes.
	ClientMessagePayloadLengthLength = 4
)

const (
	// ClientMessageHLOffset represents the offset of the header length field in the message.
	ClientMessageHLOffset = 0
	// ClientMessageMessageTypeOffset represents the offset of the message type field in the message.
	ClientMessageMessageTypeOffset = ClientMessageHLOffset + ClientMessageHLLength
	// ClientMessageSchemaVersionOffset represents the offset of the schema version field in the message.
	ClientMessageSchemaVersionOffset = ClientMessageMessageTypeOffset + ClientMessageMessageTypeLength
	// ClientMessageCreatedDateOffset represents the offset of the created date field in the message.
	ClientMessageCreatedDateOffset = ClientMessageSchemaVersionOffset + ClientMessageSchemaVersionLength
	// ClientMessageSequenceNumberOffset represents the offset of the sequence number field in the message.
	ClientMessageSequenceNumberOffset = ClientMessageCreatedDateOffset + ClientMessageCreatedDateLength
	// ClientMessageFlagsOffset represents the offset of the flags field in the message.
	ClientMessageFlagsOffset = ClientMessageSequenceNumberOffset + ClientMessageSequenceNumberLength
	// ClientMessageMessageIDOffset represents the offset of the message ID field in the message.
	ClientMessageMessageIDOffset = ClientMessageFlagsOffset + ClientMessageFlagsLength
	// ClientMessagePayloadDigestOffset represents the offset of the payload digest field in the message.
	ClientMessagePayloadDigestOffset = ClientMessageMessageIDOffset + ClientMessageMessageIDLength
	// ClientMessagePayloadTypeOffset represents the offset of the payload type field in the message.
	ClientMessagePayloadTypeOffset = ClientMessagePayloadDigestOffset + ClientMessagePayloadDigestLength
	// ClientMessagePayloadLengthOffset represents the offset of the payload length field in the message.
	ClientMessagePayloadLengthOffset = ClientMessagePayloadTypeOffset + ClientMessagePayloadTypeLength
	// ClientMessagePayloadOffset represents the offset of the payload data in the message.
	ClientMessagePayloadOffset = ClientMessagePayloadLengthOffset + ClientMessagePayloadLengthLength
)
