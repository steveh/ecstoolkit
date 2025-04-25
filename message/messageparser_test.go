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

// message package defines data channel messages structure.
package message_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/steveh/ecstoolkit/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type EXPECTATION int

const (
	SUCCESS EXPECTATION = iota
	ERROR
)

func getNByteBuffer(n int) []byte {
	return make([]byte, n)
}

// Default generator for smaller data types e.g. strings, integers.
func get8ByteBuffer() []byte {
	return getNByteBuffer(8)
}

// Default generator for UUID.
func get16ByteBuffer() []byte {
	return getNByteBuffer(16)
}

var (
	defaultByteBufferGenerator = get8ByteBuffer
	messageID                  = "dd01e56b-ff48-483e-a508-b5f073f31b16"
	messageType                = message.InputStreamMessage
	schemaVersion              = uint32(1)
	createdDate                = uint64(1503434274948)
	destinationID              = "destination-id"
	actionType                 = "start"
	payload                    = []byte("payload")
	defaultUUID                = "dd01e56b-ff48-483e-a508-b5f073f31b16"
	ackMessagePayload          = []byte(fmt.Sprintf(
		`{
			"AcknowledgedMessageType": "%s",
			"AcknowledgedMessageId":"%s"
		}`,
		message.AcknowledgeMessage,
		messageID))
	channelClosedPayload = []byte(fmt.Sprintf(
		`{
			"MessageType": "%s",
			"MessageId": "%s",
			"CreatedDate": "%s",
			"SessionId": "%s",
			"SchemaVersion": %s,
			"Output": "%s"
		}`,
		message.ChannelClosedMessage,
		messageID,
		strconv.FormatUint(createdDate, 10),
		sessionID,
		strconv.FormatUint(uint64(schemaVersion), 10),
		string(payload),
	))
	handshakeReqPayload = []byte(fmt.Sprintf(
		`{
			"AgentVersion": "%s",
			"RequestedClientActions": [
				{
					"ActionType": "%s",
					"ActionParameters": %s
				}
			]
		}`,
		agentVersion,
		actionType,
		sampleParameters,
	))
	handshakeCompletePayload = []byte(fmt.Sprintf(
		`{
			"HandshakeTimeToComplete": %d,
			"CustomerMessage": "%s"
		}`,
		timeToComplete,
		customerMessage,
	))
	timeToComplete   = 1000000
	customerMessage  = "Handshake Complete"
	sampleParameters = "{\"name\": \"richard\"}"
	sequenceNumber   = int64(2)
	agentVersion     = "3.0"
	sessionID        = "sessionId_01234567890abcedf"
)

type TestParams struct {
	name        string
	expectation EXPECTATION
	byteArray   []byte
	offsetStart int
	offsetEnd   int
	input       interface{}
	expected    interface{}
}

func TestPutString(t *testing.T) {
	t.Logf("Starting test suite: %s", t.Name())

	testCases := []TestParams{
		{
			"Basic",
			SUCCESS,
			defaultByteBufferGenerator(),
			0,
			7,
			"hello",
			"hello",
		},
		{
			"Basic offset",
			SUCCESS,
			defaultByteBufferGenerator(),
			1,
			7,
			"hello",
			"hello",
		},
		{
			"Bad offset",
			ERROR,
			defaultByteBufferGenerator(),
			-1,
			7,
			"hello",
			message.ErrOffsetOutside,
		},
		{
			"Data too long for buffer",
			ERROR,
			defaultByteBufferGenerator(),
			0,
			7,
			"longinPutString",
			message.ErrNotEnoughSpace,
		},
	}
	for _, tc := range testCases {
		testString := "Running test case: " + tc.name
		t.Run(testString, func(t *testing.T) {
			// Asserting type as string for input
			strInput, ok := tc.input.(string)
			assert.True(t, ok, "Type assertion failed in %s:%s", t.Name(), tc.name)

			err := message.PutString(
				tc.byteArray,
				tc.offsetStart,
				tc.offsetEnd,
				strInput)

			switch tc.expectation {
			case SUCCESS:
				require.NoError(t, err, "%s:%s threw an error when no error was expected.", t.Name(), tc.name)
				assert.Contains(t, string(tc.byteArray), tc.expected)
			case ERROR:
				require.Error(t, err, "%s:%s did not throw an error when an error was expected.", t.Name(), tc.name)
				expectedErr, ok := tc.expected.(error)
				assert.True(t, ok, "%s:%s expected value is not an error", t.Name(), tc.name)
				require.ErrorIs(t, err, expectedErr, "%s:%s does not match the expected error", t.Name(), tc.name)
			default:
				t.Fatal("Test expectation was not correctly set.")
			}
		})
	}
}

func TestPutBytes(t *testing.T) {
	t.Logf("Starting test suite: %s", t.Name())

	testCases := []TestParams{
		{
			"Basic",
			SUCCESS,
			defaultByteBufferGenerator(),
			0,
			3,
			[]byte{0x22, 0x55, 0xff, 0x22},
			[]byte{0x22, 0x55, 0xff, 0x22, 0x00, 0x00, 0x00, 0x00},
		},
		{
			"Basic offset",
			SUCCESS,
			defaultByteBufferGenerator(),
			1,
			4,
			[]byte{0x22, 0x55, 0xff, 0x22},
			[]byte{0x00, 0x22, 0x55, 0xff, 0x22, 0x00, 0x00, 0x00},
		},
		{
			"Bad offset",
			ERROR,
			defaultByteBufferGenerator(),
			-1,
			7,
			[]byte{0x22, 0x55, 0x00, 0x22},
			message.ErrOffsetOutside,
		},
		{
			"Data too long for buffer",
			ERROR,
			defaultByteBufferGenerator(),
			0,
			2,
			[]byte{0x22, 0x55, 0x00, 0x22},
			message.ErrNotEnoughSpace,
		},
	}
	for _, tc := range testCases {
		testString := "Running test case: " + tc.name
		t.Run(testString, func(t *testing.T) {
			// Assert type as byte array
			byteInput, ok := tc.input.([]byte)
			assert.True(t, ok, "Type assertion failed in %s:%s", t.Name(), tc.name)

			err := message.PutBytes(
				tc.byteArray,
				tc.offsetStart,
				tc.offsetEnd,
				byteInput)

			switch tc.expectation {
			case SUCCESS:
				require.NoError(t, err, "%s:%s threw an error when no error was expected.", t.Name(), tc.name)
				assert.True(t, reflect.DeepEqual(tc.byteArray, tc.expected))
			case ERROR:
				require.Error(t, err, "%s:%s did not throw an error when an error was expected.", t.Name(), tc.name)
				expectedErr, ok := tc.expected.(error)
				assert.True(t, ok, "%s:%s expected value is not an error", t.Name(), tc.name)
				require.ErrorIs(t, err, expectedErr, "%s:%s does not match the expected error", t.Name(), tc.name)
			default:
				t.Fatal("Test expectation was not correctly set.")
			}
		})
	}
}

func TestLongToBytes(t *testing.T) {
	t.Logf("Starting test suite: %s", t.Name())

	testcases := []struct {
		name        string
		expectation EXPECTATION
		input       int64
		expected    interface{}
	}{
		{
			"Basic",
			SUCCESS,
			5747283,
			[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x57, 0xb2, 0x53},
		},
	}

	for _, tc := range testcases {
		testString := "Running test case: " + tc.name
		t.Run(testString, func(t *testing.T) {
			bytes, err := message.LongToBytes(tc.input)

			switch tc.expectation {
			case SUCCESS:
				require.NoError(t, err, "An error was thrown when none was expected.")
				assert.True(t, reflect.DeepEqual(bytes, tc.expected))
			case ERROR:
				require.Error(t, err, "No error was thrown when one was expected.")
				assert.Contains(t, err, tc.expected)
			}
		})
	}
}

func TestPutLong(t *testing.T) {
	t.Logf("Starting test suite: %s", t.Name())
	// OffsetEnd is not used in PutLong: Long is always 8-bytes
	testCases := []TestParams{
		{
			"Basic",
			SUCCESS,
			getNByteBuffer(9),
			0,
			0,
			5747283,
			[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x57, 0xb2, 0x53, 0x00},
		},
		{
			"Basic offset",
			SUCCESS,
			getNByteBuffer(10),
			1,
			0,
			92837273,
			[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x05, 0x88, 0x95, 0x99, 0x00},
		},
		{
			"Exact offset",
			SUCCESS,
			defaultByteBufferGenerator(),
			0,
			0,
			50,
			[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x32},
		},
		{
			"Exact offset +1",
			ERROR,
			defaultByteBufferGenerator(),
			5,
			0,
			50,
			message.ErrOffsetOutside,
		},
		{
			"Negative offset",
			ERROR,
			getNByteBuffer(9),
			-1,
			0,
			5748,
			message.ErrOffsetOutside,
		},
		{
			"Offset out of bounds",
			ERROR,
			getNByteBuffer(4),
			10,
			0,
			938283,
			message.ErrOffsetOutside,
		},
	}
	for _, tc := range testCases {
		testString := "Running test case: " + tc.name
		t.Run(testString, func(t *testing.T) {
			// Assert type as long int
			longInput, ok := tc.input.(int)
			assert.True(t, reflect.DeepEqual(tc.input, longInput), "Cast went wrong. Expected: %v, Got: %v", tc.input, longInput)
			assert.True(t, ok, "Type assertion failed in %s:%s", t.Name(), tc.name)

			err := message.PutLong(
				tc.byteArray,
				tc.offsetStart,
				int64(longInput))

			switch tc.expectation {
			case SUCCESS:
				require.NoError(t, err, "%s:%s threw an error when no error was expected.", t.Name(), tc.name)
				assert.Equal(t, tc.expected, tc.byteArray)
			case ERROR:
				require.Error(t, err, "%s:%s did not throw an error when an error was expected.", t.Name(), tc.name)
				expectedErr, ok := tc.expected.(error)
				assert.True(t, ok, "%s:%s expected value is not an error", t.Name(), tc.name)
				require.ErrorIs(t, err, expectedErr, "%s:%s does not match the expected error", t.Name(), tc.name)
			default:
				t.Fatal("Test expectation was not correctly set.")
			}
		})
	}
}

func TestPutInteger(t *testing.T) {
	t.Logf("Starting test suite: %s", t.Name())
	// OffsetEnd is not used in PutInt: Int is always 4-bytes
	testCases := []TestParams{
		{
			"Basic",
			SUCCESS,
			getNByteBuffer(5),
			0,
			0,
			324,
			[]byte{0x00, 0x00, 0x01, 0x44, 0x00},
		},
		{
			"Basic offset",
			SUCCESS,
			defaultByteBufferGenerator(),
			1,
			0,
			520392,
			[]byte{0x00, 0x00, 0x07, 0xf0, 0xc8, 0x00, 0x00, 0x00},
		},
		{
			"Exact offset",
			SUCCESS,
			getNByteBuffer(4),
			0,
			0,
			50,
			[]byte{0x00, 0x00, 0x00, 0x32},
		},
		{
			"Exact offset +1",
			ERROR,
			defaultByteBufferGenerator(),
			5,
			0,
			50,
			message.ErrOffsetOutside,
		},
		{
			"Negative offset",
			ERROR,
			getNByteBuffer(9),
			-1,
			0,
			5748,
			message.ErrOffsetOutside,
		},
		{
			"Offset out of bounds",
			ERROR,
			getNByteBuffer(4),
			10,
			0,
			938283,
			message.ErrOffsetOutside,
		},
	}
	for _, tc := range testCases {
		testString := "Running test case: " + tc.name
		t.Run(testString, func(t *testing.T) {
			// Assert type as long int
			intInput, ok := tc.input.(int)
			assert.True(t, reflect.DeepEqual(tc.input, intInput), "Cast went wrong. Expected: %v, Got: %v", tc.input, intInput)
			assert.True(t, ok, "Type assertion failed in %s:%s", t.Name(), tc.name)

			err := message.PutInteger(
				tc.byteArray,
				tc.offsetStart,
				int32(intInput), //nolint:gosec
			)

			switch tc.expectation {
			case SUCCESS:
				require.NoError(t, err, "%s:%s threw an error when no error was expected.", t.Name(), tc.name)
				assert.Equal(t, tc.expected, tc.byteArray)
			case ERROR:
				require.Error(t, err, "%s:%s did not throw an error when an error was expected.", t.Name(), tc.name)
				expectedErr, ok := tc.expected.(error)
				assert.True(t, ok, "%s:%s expected value is not an error", t.Name(), tc.name)
				require.ErrorIs(t, err, expectedErr, "%s:%s does not match the expected error", t.Name(), tc.name)
			default:
				t.Fatal("Test expectation was not correctly set.")
			}
		})
	}
}

func TestGetString(t *testing.T) {
	t.Logf("Starting test suite: %s", t.Name())
	// For GetString, the test parameter "offsetEnd" is used to indicate the length of the string to be read.
	testCases := []TestParams{
		{
			"Basic",
			SUCCESS,
			[]byte{0x72, 0x77, 0x00},
			0,
			2,
			nil,
			"rw",
		},
		{
			"Basic offset",
			SUCCESS,
			[]byte{0x00, 0x00, 0x72, 0x77, 0x00},
			2,
			2,
			nil,
			"rw",
		},
		{
			"Negative offset",
			ERROR,
			getNByteBuffer(9),
			-1,
			0,
			nil,
			message.ErrOffsetOutside,
		},
		{
			"Offset out of bounds",
			ERROR,
			getNByteBuffer(4),
			10,
			2,
			nil,
			message.ErrOffsetOutside,
		},
	}
	for _, tc := range testCases {
		testString := "Running test case: " + tc.name
		t.Run(testString, func(t *testing.T) {
			strOut, err := message.GetString(
				tc.byteArray,
				tc.offsetStart,
				tc.offsetEnd)

			switch tc.expectation {
			case SUCCESS:
				strExpected, ok := tc.expected.(string)
				assert.True(t, ok, "%s:%s expected value is not a string", t.Name(), tc.name)
				require.NoError(t, err, "%s:%s threw an error when no error was expected.", t.Name(), tc.name)
				assert.Equal(t, strExpected, strOut)
			case ERROR:
				require.Error(t, err, "%s:%s did not throw an error when an error was expected.", t.Name(), tc.name)
				expectedErr, ok := tc.expected.(error)
				assert.True(t, ok, "%s:%s expected value is not an error", t.Name(), tc.name)
				require.ErrorIs(t, err, expectedErr, "%s:%s does not match the expected error", t.Name(), tc.name)
			default:
				t.Fatal("Test expectation was not correctly set.")
			}
		})
	}
}

func TestGetBytes(t *testing.T) {
	t.Logf("Starting test suite: %s", t.Name())
	// For GetBytes, the test parameter "offsetEnd" is used to indicate the length of the bytes to be read.
	testCases := []TestParams{
		{
			"Basic",
			SUCCESS,
			[]byte{0x72, 0x77, 0x00},
			0,
			2,
			nil,
			[]byte{0x72, 0x77},
		},
		{
			"Basic offset",
			SUCCESS,
			[]byte{0x00, 0x00, 0x72, 0x77, 0x00},
			2,
			2,
			nil,
			[]byte{0x72, 0x77},
		},
		{
			"Negative offset",
			ERROR,
			defaultByteBufferGenerator(),
			-1,
			0,
			nil,
			message.ErrOffsetOutside,
		},
		{
			"Offset out of bounds",
			ERROR,
			getNByteBuffer(4),
			10,
			2,
			nil,
			message.ErrOffsetOutside,
		},
	}
	for _, tc := range testCases {
		testString := "Running test case: " + tc.name
		t.Run(testString, func(t *testing.T) {
			byteOut, err := message.GetBytes(
				tc.byteArray,
				tc.offsetStart,
				tc.offsetEnd)

			switch tc.expectation {
			case SUCCESS:
				require.NoError(t, err, "%s:%s threw an error when no error was expected.", t.Name(), tc.name)
				assert.Equal(t, tc.expected, byteOut)
			case ERROR:
				require.Error(t, err, "%s:%s did not throw an error when an error was expected.", t.Name(), tc.name)
				expectedErr, ok := tc.expected.(error)
				assert.True(t, ok, "%s:%s expected value is not an error", t.Name(), tc.name)
				require.ErrorIs(t, err, expectedErr, "%s:%s does not match the expected error", t.Name(), tc.name)
			default:
				t.Fatal("Test expectation was not correctly set.")
			}
		})
	}
}

func TestGetLong(t *testing.T) {
	t.Logf("Starting test suite: %s", t.Name())
	// For GetLong, effsetEnd is not used as a test parameter.
	testCases := []TestParams{
		{
			"Basic",
			SUCCESS,
			[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x5a, 0x05, 0x66, 0x00},
			0,
			0,
			nil,
			5899622,
		},
		{
			"Basic offset",
			SUCCESS,
			[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x5a, 0x05, 0x6a, 0x00},
			2,
			0,
			nil,
			5899626,
		},
		{
			"Exact offset",
			SUCCESS,
			[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x32},
			0,
			0,
			nil,
			50,
		},
		{
			"Exact offset +1",
			ERROR,
			[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			1,
			0,
			nil,
			message.ErrOffsetOutside,
		},
		{
			"Negative offset",
			ERROR,
			getNByteBuffer(9),
			-1,
			0,
			nil,
			message.ErrOffsetOutside,
		},
		{
			"Offset out of bounds",
			ERROR,
			getNByteBuffer(4),
			10,
			2,
			nil,
			message.ErrOffsetOutside,
		},
	}
	for _, tc := range testCases {
		testString := "Running test case: " + tc.name
		t.Run(testString, func(t *testing.T) {
			longOut, err := message.GetLong(
				tc.byteArray,
				tc.offsetStart)
			assert.IsType(t, int64(1), longOut, "Returned value is not the correct type.")

			switch tc.expectation {
			case SUCCESS:
				expectedInt, ok := tc.expected.(int)
				assert.True(t, ok, "%s:%s expected value is not an int", t.Name(), tc.name)

				expectedLong := int64(expectedInt)

				require.NoError(t, err, "%s:%s threw an error when no error was expected.", t.Name(), tc.name)
				assert.Equal(t, expectedLong, longOut)
			case ERROR:
				require.Error(t, err, "%s:%s did not throw an error when an error was expected.", t.Name(), tc.name)
				expectedErr, ok := tc.expected.(error)
				assert.True(t, ok, "%s:%s expected value is not an error", t.Name(), tc.name)
				require.ErrorIs(t, err, expectedErr, "%s:%s does not match the expected error", t.Name(), tc.name)
			default:
				t.Fatal("Test expectation was not correctly set.")
			}
		})
	}
}

func TestClientMessage_Validate(t *testing.T) {
	u, err := uuid.Parse(messageID)
	require.NoError(t, err)

	clientMessage := message.ClientMessage{
		SchemaVersion:  schemaVersion,
		SequenceNumber: 1,
		Flags:          2,
		MessageID:      u,
		Payload:        payload,
		PayloadLength:  3,
	}

	err = clientMessage.Validate()
	require.Error(t, err, "No error was thrown when one was expected.")
	assert.Contains(t, err.Error(), "HeaderLength cannot be zero")

	clientMessage.HeaderLength = 1
	err = clientMessage.Validate()
	require.Error(t, err, "No error was thrown when one was expected.")
	assert.Contains(t, err.Error(), "MessageType is missing")

	clientMessage.MessageType = messageType
	err = clientMessage.Validate()
	require.Error(t, err, "No error was thrown when one was expected.")
	assert.Contains(t, err.Error(), "CreatedDate is missing")

	clientMessage.CreatedDate = createdDate
	err = clientMessage.Validate()
	require.Error(t, err, "No error was thrown when one was expected.")
	assert.Contains(t, err.Error(), "PayloadDigest is invalid")

	hasher := sha256.New()
	hasher.Write(payload)
	clientMessage.PayloadDigest = hasher.Sum(nil)
	err = clientMessage.Validate()
	require.NoError(t, err, "An error was thrown when none was expected.")
}

func TestClientMessage_ValidateStartPublicationMessage(t *testing.T) {
	u, err := uuid.Parse(messageID)
	require.NoError(t, err)

	clientMessage := message.ClientMessage{
		MessageType:    message.StartPublicationMessage,
		SchemaVersion:  schemaVersion,
		CreatedDate:    createdDate,
		SequenceNumber: 1,
		Flags:          2,
		MessageID:      u,
		Payload:        payload,
		PayloadLength:  3,
	}

	err = clientMessage.Validate()
	require.NoError(t, err, "Validating StartPublicationMessage should not throw an error")
}

func TestClientMessage_DeserializeDataStreamAcknowledgeContent(t *testing.T) {
	t.Logf("Starting test: %s", t.Name())
	// ClientMessage is initialized with improperly formatted json data
	testMessage := message.ClientMessage{
		Payload: payload,
	}

	ackMessage, err := testMessage.DeserializeDataStreamAcknowledgeContent()
	assert.Equal(t, message.AcknowledgeContent{}, ackMessage)
	require.Error(t, err, "An error was not thrown when one was expected.")

	testMessage.MessageType = message.AcknowledgeMessage
	ackMessage2, err := testMessage.DeserializeDataStreamAcknowledgeContent()
	assert.Equal(t, message.AcknowledgeContent{}, ackMessage2)
	require.Error(t, err, "An error was not thrown when one was expected.")

	testMessage.Payload = ackMessagePayload
	ackMessage3, err := testMessage.DeserializeDataStreamAcknowledgeContent()
	assert.Equal(t, message.AcknowledgeMessage, ackMessage3.MessageType)
	assert.Equal(t, messageID, ackMessage3.MessageID)
	require.NoError(t, err, "An error was thrown when one was not expected.")
}

func TestClientMessage_DeserializeChannelClosedMessage(t *testing.T) {
	t.Logf("Starting test: %s", t.Name())
	// ClientMessage is initialized with improperly formatted json data
	testMessage := message.ClientMessage{
		Payload: payload,
	}

	closeMessage, err := testMessage.DeserializeChannelClosedMessage()
	assert.Equal(t, message.ChannelClosed{}, closeMessage)
	require.Error(t, err, "An error was not thrown when one was expected.")

	testMessage.MessageType = message.ChannelClosedMessage
	closeMessage2, err := testMessage.DeserializeChannelClosedMessage()
	assert.Equal(t, message.ChannelClosed{}, closeMessage2)
	require.Error(t, err, "An error was not thrown when one was expected.")

	testMessage.Payload = channelClosedPayload
	closeMessage3, err := testMessage.DeserializeChannelClosedMessage()
	assert.Equal(t, message.ChannelClosedMessage, closeMessage3.MessageType)
	assert.Equal(t, messageID, closeMessage3.MessageID)
	assert.Equal(t, strconv.FormatUint(createdDate, 10), closeMessage3.CreatedDate)
	assert.Equal(t, int(schemaVersion), closeMessage3.SchemaVersion)
	assert.Equal(t, sessionID, closeMessage3.SessionID)
	assert.Equal(t, string(payload), closeMessage3.Output)
	require.NoError(t, err, "An error was thrown when one was not expected.")
}

func TestClientMessage_DeserializeHandshakeRequest(t *testing.T) {
	t.Logf("Starting test: %s", t.Name())
	// ClientMessage is initialized with improperly formatted json data
	testMessage := message.ClientMessage{
		Payload: payload,
	}

	handshakeReq, err := testMessage.DeserializeHandshakeRequest()
	assert.Equal(t, message.HandshakeRequestPayload{}, handshakeReq)
	require.Error(t, err, "An error was not thrown when one was expected.")

	testMessage.PayloadType = uint32(message.HandshakeRequestPayloadType)
	handshakeReq2, err := testMessage.DeserializeHandshakeRequest()
	assert.Equal(t, message.HandshakeRequestPayload{}, handshakeReq2)
	require.Error(t, err, "An error was not thrown when one was expected.")

	testMessage.Payload = handshakeReqPayload
	handshakeReq3, err := testMessage.DeserializeHandshakeRequest()
	assert.Equal(t, agentVersion, handshakeReq3.AgentVersion)
	assert.Equal(t, message.ActionType(actionType), handshakeReq3.RequestedClientActions[0].ActionType)
	assert.JSONEq(t, sampleParameters, string(handshakeReq3.RequestedClientActions[0].ActionParameters))
	require.NoError(t, err, "An error was thrown when one was not expected.")
}

func TestClientMessage_DeserializeHandshakeComplete(t *testing.T) {
	t.Logf("Starting test: %s", t.Name())
	// ClientMessage is initialized with improperly formatted json data
	testMessage := message.ClientMessage{
		Payload: payload,
	}

	handshakeComplete, err := testMessage.DeserializeHandshakeComplete()
	assert.Equal(t, message.HandshakeCompletePayload{}, handshakeComplete)
	require.Error(t, err, "An error was not thrown when one was expected.")

	testMessage.PayloadType = uint32(message.HandshakeCompletePayloadType)
	handshakeComplete2, err := testMessage.DeserializeHandshakeComplete()
	assert.Equal(t, message.HandshakeCompletePayload{}, handshakeComplete2)
	require.Error(t, err, "An error was not thrown when one was expected.")

	testMessage.Payload = handshakeCompletePayload
	handshakeComplete3, err := testMessage.DeserializeHandshakeComplete()
	assert.Equal(t, time.Duration(timeToComplete), handshakeComplete3.HandshakeTimeToComplete)
	assert.Equal(t, customerMessage, handshakeComplete3.CustomerMessage)
	require.NoError(t, err, "An error was thrown when one was not expected.")
}

func TestPutUuid(t *testing.T) {
	t.Logf("Starting test suite: %s", t.Name())
	// OffsetEnd is not used for putUUID as uuid are always 128-bit
	testCases := []TestParams{
		{
			"Basic",
			SUCCESS,
			get16ByteBuffer(),
			0,
			0,
			defaultUUID,
			defaultUUID,
		},
	}
	for _, tc := range testCases {
		testString := "Running test case: " + tc.name
		t.Run(testString, func(t *testing.T) {
			// Asserting type as string for input
			strInput, ok := tc.input.(string)
			assert.True(t, ok, "Type assertion failed in %s:%s", t.Name(), tc.name)

			// Get Uuid from string
			uuidInput, err := uuid.Parse(strInput)
			require.NoError(t, err)

			err = message.PutUUID(
				tc.byteArray,
				tc.offsetStart,
				uuidInput)

			switch tc.expectation {
			case SUCCESS:
				require.NoError(t, err, "%s:%s threw an error when no error was expected.", t.Name(), tc.name)
				strExpected, ok := tc.expected.(string)
				assert.True(t, ok, "%s:%s expected value is not a string", t.Name(), tc.name)

				uuidOut, err := uuid.Parse(strExpected)
				require.NoError(t, err)

				expectedBuffer := get16ByteBuffer()
				err = message.PutUUID(expectedBuffer, 0, uuidOut)
				require.NoError(t, err, "Error putting UUID")
				assert.Equal(t, expectedBuffer, tc.byteArray)
			case ERROR:
				require.Error(t, err, "%s:%s did not throw an error when an error was expected.", t.Name(), tc.name)
				expectedErr, ok := tc.expected.(error)
				assert.True(t, ok, "%s:%s expected value is not an error", t.Name(), tc.name)
				require.ErrorIs(t, err, expectedErr, "%s:%s does not match the expected error", t.Name(), tc.name)
			default:
				t.Fatal("Test expectation was not correctly set.")
			}
		})
	}
}

func TestPutGetString(t *testing.T) {
	input := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x01, 0x00, 0x01}
	err1 := message.PutString(input, 1, 8, "hello")
	require.NoError(t, err1)

	result, err := message.GetString(input, 1, 8)
	require.NoError(t, err)
	assert.Equal(t, "hello", result)
}

func TestPutGetInteger(t *testing.T) {
	input := []byte{0x00, 0x00, 0x00, 0x00, 0xFF, 0x00}
	err := message.PutInteger(input, 1, 256)
	require.NoError(t, err)
	assert.Equal(t, byte(0x00), input[1])
	assert.Equal(t, byte(0x00), input[2])
	assert.Equal(t, byte(0x01), input[3])
	assert.Equal(t, byte(0x00), input[4])

	result, err2 := message.GetInteger(input, 1)
	require.NoError(t, err2)
	assert.Equal(t, int32(256), result)

	result2, err3 := message.GetInteger(input, 2)
	assert.Equal(t, int32(65536), result2)
	require.NoError(t, err3)

	result3, err4 := message.GetInteger(input, 3)
	assert.Equal(t, int32(0), result3)
	require.Error(t, err4)
}

func TestPutGetLong(t *testing.T) {
	input := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00}
	err := message.PutLong(input, 1, 4294967296) // 2 to the 32 + 1
	require.NoError(t, err)
	assert.Equal(t, byte(0x00), input[1])
	assert.Equal(t, byte(0x00), input[2])
	assert.Equal(t, byte(0x00), input[3])
	assert.Equal(t, byte(0x01), input[4])
	assert.Equal(t, byte(0x00), input[5])
	assert.Equal(t, byte(0x00), input[6])
	assert.Equal(t, byte(0x00), input[7])
	assert.Equal(t, byte(0x00), input[8])

	testLong, err2 := message.GetLong(input, 1)
	require.NoError(t, err2)
	assert.Equal(t, int64(4294967296), testLong)
}

func TestGetBytesFromInteger(t *testing.T) {
	input := int32(256)
	result, err := message.IntegerToBytes(input)
	require.NoError(t, err)
	assert.Equal(t, byte(0x00), result[0])
	assert.Equal(t, byte(0x00), result[1])
	assert.Equal(t, byte(0x01), result[2])
	assert.Equal(t, byte(0x00), result[3])
}

func TestSerializeAndDeserializeClientMessage(t *testing.T) {
	u, err := uuid.Parse(messageID)
	require.NoError(t, err)

	clientMessage := message.ClientMessage{
		MessageType:    messageType,
		SchemaVersion:  schemaVersion,
		CreatedDate:    createdDate,
		SequenceNumber: 1,
		Flags:          2,
		MessageID:      u,
		Payload:        payload,
	}

	// Test SerializeClientMessage
	serializedBytes, err := clientMessage.SerializeClientMessage()
	require.NoError(t, err, "Error serializing message")

	seralizedMessageType := strings.TrimRight(string(serializedBytes[message.ClientMessageMessageTypeOffset:message.ClientMessageMessageTypeOffset+message.ClientMessageMessageTypeLength-1]), " ")
	assert.Equal(t, seralizedMessageType, messageType)

	serializedVersion, err := message.GetUInteger(serializedBytes, message.ClientMessageSchemaVersionOffset)
	require.NoError(t, err)
	assert.Equal(t, serializedVersion, schemaVersion)

	serializedCD, err := message.GetULong(serializedBytes, message.ClientMessageCreatedDateOffset)
	require.NoError(t, err)
	assert.Equal(t, serializedCD, createdDate)

	serializedSequence, err := message.GetLong(serializedBytes, message.ClientMessageSequenceNumberOffset)
	require.NoError(t, err)
	assert.Equal(t, int64(1), serializedSequence)

	serializedFlags, err := message.GetULong(serializedBytes, message.ClientMessageFlagsOffset)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), serializedFlags)

	seralizedMessageID, err := message.GetUUID(serializedBytes, message.ClientMessageMessageIDOffset)
	require.NoError(t, err)
	assert.Equal(t, seralizedMessageID.String(), messageID)

	serializedDigest, err := message.GetBytes(serializedBytes, message.ClientMessagePayloadDigestOffset, message.ClientMessagePayloadDigestLength)
	require.NoError(t, err)

	hasher := sha256.New()
	hasher.Write(clientMessage.Payload)
	expectedHash := hasher.Sum(nil)
	assert.True(t, reflect.DeepEqual(serializedDigest, expectedHash))

	// Test DeserializeClientMessage
	deserializedClientMessage := &message.ClientMessage{}
	err = deserializedClientMessage.DeserializeClientMessage(serializedBytes)
	require.NoError(t, err)
	assert.Equal(t, messageType, deserializedClientMessage.MessageType)
	assert.Equal(t, schemaVersion, deserializedClientMessage.SchemaVersion)
	assert.Equal(t, messageID, deserializedClientMessage.MessageID.String())
	assert.Equal(t, createdDate, deserializedClientMessage.CreatedDate)
	assert.Equal(t, uint64(2), deserializedClientMessage.Flags)
	assert.Equal(t, int64(1), deserializedClientMessage.SequenceNumber)
	assert.True(t, reflect.DeepEqual(payload, deserializedClientMessage.Payload))
}

func TestSerializeMessagePayloadNegative(t *testing.T) {
	functionEx := func() {}
	_, err := message.SerializeClientMessagePayload(functionEx)
	require.Error(t, err)
}

func TestSerializeAndDeserializeClientMessageWithAcknowledgeContent(t *testing.T) {
	acknowledgeContent := message.AcknowledgeContent{
		MessageType:         messageType,
		MessageID:           messageID,
		SequenceNumber:      sequenceNumber,
		IsSequentialMessage: true,
	}

	serializedClientMsg, _ := message.SerializeClientMessageWithAcknowledgeContent(acknowledgeContent)
	deserializedClientMsg := &message.ClientMessage{}
	err := deserializedClientMsg.DeserializeClientMessage(serializedClientMsg)
	require.NoError(t, err)
	deserializedAcknowledgeContent, err := deserializedClientMsg.DeserializeDataStreamAcknowledgeContent()

	require.NoError(t, err)
	assert.Equal(t, messageType, deserializedAcknowledgeContent.MessageType)
	assert.Equal(t, messageID, deserializedAcknowledgeContent.MessageID)
	assert.Equal(t, sequenceNumber, deserializedAcknowledgeContent.SequenceNumber)
	assert.True(t, deserializedAcknowledgeContent.IsSequentialMessage)
}

func TestDeserializeAgentMessageWithChannelClosed(t *testing.T) {
	channelClosed := message.ChannelClosed{
		MessageType:   message.ChannelClosedMessage,
		MessageID:     messageID,
		DestinationID: destinationID,
		SessionID:     sessionID,
		SchemaVersion: 1,
		CreatedDate:   "2018-01-01",
	}

	u, err := uuid.Parse(messageID)
	require.NoError(t, err)

	channelClosedJSON, err := json.Marshal(channelClosed)
	if err != nil {
		t.Fatalf("marshaling channel closed: %v", err)
	}

	agentMessage := message.ClientMessage{
		MessageType:    message.ChannelClosedMessage,
		SchemaVersion:  schemaVersion,
		CreatedDate:    createdDate,
		SequenceNumber: 1,
		Flags:          2,
		MessageID:      u,
		Payload:        channelClosedJSON,
	}

	deserializedChannelClosed, err := agentMessage.DeserializeChannelClosedMessage()

	require.NoError(t, err)
	assert.Equal(t, message.ChannelClosedMessage, deserializedChannelClosed.MessageType)
	assert.Equal(t, messageID, deserializedChannelClosed.MessageID)
	assert.Equal(t, sessionID, deserializedChannelClosed.SessionID)
	assert.Equal(t, "destination-id", deserializedChannelClosed.DestinationID)
}
