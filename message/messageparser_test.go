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
package message

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/steveh/ecstoolkit/log"
	"github.com/stretchr/testify/assert"
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
	mockLogger                 = log.NewMockLog()
	defaultByteBufferGenerator = get8ByteBuffer
	messageId                  = "dd01e56b-ff48-483e-a508-b5f073f31b16"
	messageType                = InputStreamMessage
	schemaVersion              = uint32(1)
	createdDate                = uint64(1503434274948)
	destinationId              = "destination-id"
	actionType                 = "start"
	payload                    = []byte("payload")
	defaultUuid                = "dd01e56b-ff48-483e-a508-b5f073f31b16"
	ackMessagePayload          = []byte(fmt.Sprintf(
		`{
			"AcknowledgedMessageType": "%s",
			"AcknowledgedMessageId":"%s"
		}`,
		AcknowledgeMessage,
		messageId))
	channelClosedPayload = []byte(fmt.Sprintf(
		`{
			"MessageType": "%s",
			"MessageId": "%s",
			"CreatedDate": "%s",
			"SessionId": "%s",
			"SchemaVersion": %s,
			"Output": "%s"
		}`,
		ChannelClosedMessage,
		messageId,
		strconv.FormatUint(createdDate, 10),
		sessionId,
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
	sessionId        = "sessionId_01234567890abcedf"
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
			ErrOffsetOutside,
		},
		{
			"Data too long for buffer",
			ERROR,
			defaultByteBufferGenerator(),
			0,
			7,
			"longinputstring",
			ErrNotEnoughSpace,
		},
	}
	for _, tc := range testCases {
		testString := "Running test case: " + tc.name
		t.Run(testString, func(t *testing.T) {
			// Asserting type as string for input
			strInput, ok := tc.input.(string)
			assert.True(t, ok, "Type assertion failed in %s:%s", t.Name(), tc.name)

			err := putString(
				mockLogger,
				tc.byteArray,
				tc.offsetStart,
				tc.offsetEnd,
				strInput)

			switch tc.expectation {
			case SUCCESS:
				assert.NoError(t, err, "%s:%s threw an error when no error was expected.", t.Name(), tc.name)
				assert.Contains(t, string(tc.byteArray), tc.expected)
			case ERROR:
				assert.Error(t, err, "%s:%s did not throw an error when an error was expected.", t.Name(), tc.name)
				assert.True(t, errors.Is(err, tc.expected.(error)), "%s:%s does not match the expected error", t.Name(), tc.name)
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
			ErrOffsetOutsideByteArray,
		},
		{
			"Data too long for buffer",
			ERROR,
			defaultByteBufferGenerator(),
			0,
			2,
			[]byte{0x22, 0x55, 0x00, 0x22},
			ErrNotEnoughSpace,
		},
	}
	for _, tc := range testCases {
		testString := "Running test case: " + tc.name
		t.Run(testString, func(t *testing.T) {
			// Assert type as byte array
			byteInput, ok := tc.input.([]byte)
			assert.True(t, ok, "Type assertion failed in %s:%s", t.Name(), tc.name)

			err := putBytes(
				mockLogger,
				tc.byteArray,
				tc.offsetStart,
				tc.offsetEnd,
				byteInput)

			switch tc.expectation {
			case SUCCESS:
				assert.NoError(t, err, "%s:%s threw an error when no error was expected.", t.Name(), tc.name)
				assert.True(t, reflect.DeepEqual(tc.byteArray, tc.expected))
			case ERROR:
				assert.Error(t, err, "%s:%s did not throw an error when an error was expected.", t.Name(), tc.name)
				assert.True(t, errors.Is(err, tc.expected.(error)), "%s:%s does not match the expected error", t.Name(), tc.name)
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
			bytes, err := longToBytes(mockLogger, tc.input)

			switch tc.expectation {
			case SUCCESS:
				assert.NoError(t, err, "An error was thrown when none was expected.")
				assert.True(t, reflect.DeepEqual(bytes, tc.expected))
			case ERROR:
				assert.Error(t, err, "No error was thrown when one was expected.")
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
			1,
			0,
			50,
			"offset outside",
		},
		{
			"Negative offset",
			ERROR,
			getNByteBuffer(9),
			-1,
			0,
			5748,
			"offset outside",
		},
		{
			"Offset out of bounds",
			ERROR,
			getNByteBuffer(4),
			10,
			0,
			938283,
			"offset outside",
		},
	}
	for _, tc := range testCases {
		testString := "Running test case: " + tc.name
		t.Run(testString, func(t *testing.T) {
			// Assert type as long int
			longInput, ok := tc.input.(int)
			assert.True(t, reflect.DeepEqual(tc.input, longInput), "Cast went wrong. Expected: %v, Got: %v", tc.input, longInput)
			assert.True(t, ok, "Type assertion failed in %s:%s", t.Name(), tc.name)

			err := putLong(
				mockLogger,
				tc.byteArray,
				tc.offsetStart,
				int64(longInput))

			switch tc.expectation {
			case SUCCESS:
				assert.NoError(t, err, "%s:%s threw an error when no error was expected.", t.Name(), tc.name)
				assert.Equal(t, tc.expected, tc.byteArray)
			case ERROR:
				assert.Error(t, err, "%s:%s did not throw an error when an error was expected.", t.Name(), tc.name)
				assert.Contains(t, err.Error(), tc.expected, "%s:%s does not contain the intended message. Expected: \"%s\", Actual: \"%s\"", tc.expected, err)
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
			"offset outside",
		},
		{
			"Negative offset",
			ERROR,
			getNByteBuffer(9),
			-1,
			0,
			5748,
			"offset outside",
		},
		{
			"Offset out of bounds",
			ERROR,
			getNByteBuffer(4),
			10,
			0,
			938283,
			"offset outside",
		},
	}
	for _, tc := range testCases {
		testString := "Running test case: " + tc.name
		t.Run(testString, func(t *testing.T) {
			// Assert type as long int
			intInput, ok := tc.input.(int)
			assert.True(t, reflect.DeepEqual(tc.input, intInput), "Cast went wrong. Expected: %v, Got: %v", tc.input, intInput)
			assert.True(t, ok, "Type assertion failed in %s:%s", t.Name(), tc.name)

			err := putInteger(
				mockLogger,
				tc.byteArray,
				tc.offsetStart,
				int32(intInput))

			switch tc.expectation {
			case SUCCESS:
				assert.NoError(t, err, "%s:%s threw an error when no error was expected.", t.Name(), tc.name)
				assert.Equal(t, tc.expected, tc.byteArray)
			case ERROR:
				assert.Error(t, err, "%s:%s did not throw an error when an error was expected.", t.Name(), tc.name)
				assert.Contains(t, err.Error(), tc.expected, "%s:%s does not contain the intended message. Expected: \"%s\", Actual: \"%s\"", tc.expected, err)
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
			ErrOffsetOutsideByteArrayNoPoint,
		},
		{
			"Offset out of bounds",
			ERROR,
			getNByteBuffer(4),
			10,
			2,
			nil,
			ErrOffsetOutsideByteArrayNoPoint,
		},
	}
	for _, tc := range testCases {
		testString := "Running test case: " + tc.name
		t.Run(testString, func(t *testing.T) {
			strOut, err := getString(
				mockLogger,
				tc.byteArray,
				tc.offsetStart,
				tc.offsetEnd)

			switch tc.expectation {
			case SUCCESS:
				assert.NoError(t, err, "%s:%s threw an error when no error was expected.", t.Name(), tc.name)
				assert.Equal(t, tc.expected, strOut)
			case ERROR:
				assert.Error(t, err, "%s:%s did not throw an error when an error was expected.", t.Name(), tc.name)
				assert.True(t, errors.Is(err, tc.expected.(error)), "%s:%s does not match the expected error", t.Name(), tc.name)
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
			ErrOffsetOutsideByteArray,
		},
		{
			"Offset out of bounds",
			ERROR,
			getNByteBuffer(4),
			10,
			2,
			nil,
			ErrOffsetOutsideByteArray,
		},
	}
	for _, tc := range testCases {
		testString := "Running test case: " + tc.name
		t.Run(testString, func(t *testing.T) {
			byteOut, err := getBytes(
				mockLogger,
				tc.byteArray,
				tc.offsetStart,
				tc.offsetEnd)

			switch tc.expectation {
			case SUCCESS:
				assert.NoError(t, err, "%s:%s threw an error when no error was expected.", t.Name(), tc.name)
				assert.Equal(t, tc.expected, byteOut)
			case ERROR:
				assert.Error(t, err, "%s:%s did not throw an error when an error was expected.", t.Name(), tc.name)
				assert.True(t, errors.Is(err, tc.expected.(error)), "%s:%s does not match the expected error", t.Name(), tc.name)
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
			ErrOffsetOutsideByteArray,
		},
		{
			"Negative offset",
			ERROR,
			getNByteBuffer(9),
			-1,
			0,
			nil,
			ErrOffsetOutsideByteArray,
		},
		{
			"Offset out of bounds",
			ERROR,
			getNByteBuffer(4),
			10,
			2,
			nil,
			ErrOffsetOutsideByteArray,
		},
	}
	for _, tc := range testCases {
		testString := "Running test case: " + tc.name
		t.Run(testString, func(t *testing.T) {
			longOut, err := getLong(
				mockLogger,
				tc.byteArray,
				tc.offsetStart)
			assert.IsType(t, int64(1), longOut, "Returned value is not the correct type.")

			switch tc.expectation {
			case SUCCESS:
				expectedInt := tc.expected.(int)
				expectedLong := int64(expectedInt)

				assert.NoError(t, err, "%s:%s threw an error when no error was expected.", t.Name(), tc.name)
				assert.Equal(t, expectedLong, longOut)
			case ERROR:
				assert.Error(t, err, "%s:%s did not throw an error when an error was expected.", t.Name(), tc.name)
				assert.True(t, errors.Is(err, tc.expected.(error)), "%s:%s does not match the expected error", t.Name(), tc.name)
			default:
				t.Fatal("Test expectation was not correctly set.")
			}
		})
	}
}

func TestClientMessage_Validate(t *testing.T) {
	u, err := uuid.Parse(messageId)
	assert.NoError(t, err)

	clientMessage := ClientMessage{
		SchemaVersion:  schemaVersion,
		SequenceNumber: 1,
		Flags:          2,
		MessageId:      u,
		Payload:        payload,
		PayloadLength:  3,
	}

	err = clientMessage.Validate()
	assert.Error(t, err, "No error was thrown when one was expected.")
	assert.Contains(t, err.Error(), "HeaderLength cannot be zero")

	clientMessage.HeaderLength = 1
	err = clientMessage.Validate()
	assert.Error(t, err, "No error was thrown when one was expected.")
	assert.Contains(t, err.Error(), "MessageType is missing")

	clientMessage.MessageType = messageType
	err = clientMessage.Validate()
	assert.Error(t, err, "No error was thrown when one was expected.")
	assert.Contains(t, err.Error(), "CreatedDate is missing")

	clientMessage.CreatedDate = createdDate
	err = clientMessage.Validate()
	assert.Error(t, err, "No error was thrown when one was expected.")
	assert.Contains(t, err.Error(), "payload Hash is not valid")

	hasher := sha256.New()
	hasher.Write(payload)
	clientMessage.PayloadDigest = hasher.Sum(nil)
	err = clientMessage.Validate()
	assert.NoError(t, err, "An error was thrown when none was expected.")
}

func TestClientMessage_ValidateStartPublicationMessage(t *testing.T) {
	u, err := uuid.Parse(messageId)
	assert.NoError(t, err)

	clientMessage := ClientMessage{
		MessageType:    StartPublicationMessage,
		SchemaVersion:  schemaVersion,
		CreatedDate:    createdDate,
		SequenceNumber: 1,
		Flags:          2,
		MessageId:      u,
		Payload:        payload,
		PayloadLength:  3,
	}

	err = clientMessage.Validate()
	assert.NoError(t, err, "Validating StartPublicationMessage should not throw an error")
}

func TestClientMessage_DeserializeDataStreamAcknowledgeContent(t *testing.T) {
	t.Logf("Starting test: %s", t.Name())
	// ClientMessage is initialized with improperly formatted json data
	testMessage := ClientMessage{
		Payload: payload,
	}

	ackMessage, err := testMessage.DeserializeDataStreamAcknowledgeContent(mockLogger)
	assert.Equal(t, AcknowledgeContent{}, ackMessage)
	assert.Error(t, err, "An error was not thrown when one was expected.")

	testMessage.MessageType = AcknowledgeMessage
	ackMessage2, err := testMessage.DeserializeDataStreamAcknowledgeContent(mockLogger)
	assert.Equal(t, AcknowledgeContent{}, ackMessage2)
	assert.Error(t, err, "An error was not thrown when one was expected.")

	testMessage.Payload = ackMessagePayload
	ackMessage3, err := testMessage.DeserializeDataStreamAcknowledgeContent(mockLogger)
	assert.Equal(t, AcknowledgeMessage, ackMessage3.MessageType)
	assert.Equal(t, messageId, ackMessage3.MessageId)
	assert.NoError(t, err, "An error was thrown when one was not expected.")
}

func TestClientMessage_DeserializeChannelClosedMessage(t *testing.T) {
	t.Logf("Starting test: %s", t.Name())
	// ClientMessage is initialized with improperly formatted json data
	testMessage := ClientMessage{
		Payload: payload,
	}

	closeMessage, err := testMessage.DeserializeChannelClosedMessage(mockLogger)
	assert.Equal(t, ChannelClosed{}, closeMessage)
	assert.Error(t, err, "An error was not thrown when one was expected.")

	testMessage.MessageType = ChannelClosedMessage
	closeMessage2, err := testMessage.DeserializeChannelClosedMessage(mockLogger)
	assert.Equal(t, ChannelClosed{}, closeMessage2)
	assert.Error(t, err, "An error was not thrown when one was expected.")

	testMessage.Payload = channelClosedPayload
	closeMessage3, err := testMessage.DeserializeChannelClosedMessage(mockLogger)
	assert.Equal(t, ChannelClosedMessage, closeMessage3.MessageType)
	assert.Equal(t, messageId, closeMessage3.MessageId)
	assert.Equal(t, strconv.FormatUint(createdDate, 10), closeMessage3.CreatedDate)
	assert.Equal(t, int(schemaVersion), closeMessage3.SchemaVersion)
	assert.Equal(t, sessionId, closeMessage3.SessionId)
	assert.Equal(t, string(payload), closeMessage3.Output)
	assert.NoError(t, err, "An error was thrown when one was not expected.")
}

func TestClientMessage_DeserializeHandshakeRequest(t *testing.T) {
	t.Logf("Starting test: %s", t.Name())
	// ClientMessage is initialized with improperly formatted json data
	testMessage := ClientMessage{
		Payload: payload,
	}

	handshakeReq, err := testMessage.DeserializeHandshakeRequest(mockLogger)
	assert.Equal(t, HandshakeRequestPayload{}, handshakeReq)
	assert.Error(t, err, "An error was not thrown when one was expected.")

	testMessage.PayloadType = uint32(HandshakeRequestPayloadType)
	handshakeReq2, err := testMessage.DeserializeHandshakeRequest(mockLogger)
	assert.Equal(t, HandshakeRequestPayload{}, handshakeReq2)
	assert.Error(t, err, "An error was not thrown when one was expected.")

	testMessage.Payload = handshakeReqPayload
	handshakeReq3, err := testMessage.DeserializeHandshakeRequest(mockLogger)
	assert.Equal(t, agentVersion, handshakeReq3.AgentVersion)
	assert.Equal(t, ActionType(actionType), handshakeReq3.RequestedClientActions[0].ActionType)
	assert.JSONEq(t, sampleParameters, string(handshakeReq3.RequestedClientActions[0].ActionParameters))
	assert.NoError(t, err, "An error was thrown when one was not expected.")
}

func TestClientMessage_DeserializeHandshakeComplete(t *testing.T) {
	t.Logf("Starting test: %s", t.Name())
	// ClientMessage is initialized with improperly formatted json data
	testMessage := ClientMessage{
		Payload: payload,
	}

	handshakeComplete, err := testMessage.DeserializeHandshakeComplete(mockLogger)
	assert.Equal(t, HandshakeCompletePayload{}, handshakeComplete)
	assert.Error(t, err, "An error was not thrown when one was expected.")

	testMessage.PayloadType = uint32(HandshakeCompletePayloadType)
	handshakeComplete2, err := testMessage.DeserializeHandshakeComplete(mockLogger)
	assert.Equal(t, HandshakeCompletePayload{}, handshakeComplete2)
	assert.Error(t, err, "An error was not thrown when one was expected.")

	testMessage.Payload = handshakeCompletePayload
	handshakeComplete3, err := testMessage.DeserializeHandshakeComplete(mockLogger)
	assert.Equal(t, time.Duration(timeToComplete), handshakeComplete3.HandshakeTimeToComplete)
	assert.Equal(t, customerMessage, handshakeComplete3.CustomerMessage)
	assert.NoError(t, err, "An error was thrown when one was not expected.")
}

func TestPutUuid(t *testing.T) {
	t.Logf("Starting test suite: %s", t.Name())
	// OffsetEnd is not used for putUuid as uuid are always 128-bit
	testCases := []TestParams{
		{
			"Basic",
			SUCCESS,
			get16ByteBuffer(),
			0,
			0,
			defaultUuid,
			defaultUuid,
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
			assert.NoError(t, err)

			err = putUuid(
				mockLogger,
				tc.byteArray,
				tc.offsetStart,
				uuidInput)

			switch tc.expectation {
			case SUCCESS:
				assert.NoError(t, err, "%s:%s threw an error when no error was expected.", t.Name(), tc.name)
				strExpected := tc.expected.(string)
				uuidOut, err := uuid.Parse(strExpected)
				assert.NoError(t, err)

				expectedBuffer := get16ByteBuffer()
				err = putUuid(mockLogger, expectedBuffer, 0, uuidOut)
				assert.NoError(t, err, "Error putting UUID")
				assert.Equal(t, expectedBuffer, tc.byteArray)
			case ERROR:
				assert.Error(t, err, "%s:%s did not throw an error when an error was expected.", t.Name(), tc.name)
				assert.Contains(t, err.Error(), tc.expected, "%s:%s does not contain the intended message. Expected: \"%s\", Actual: \"%s\"", tc.expected, err)
			default:
				t.Fatal("Test expectation was not correctly set.")
			}
		})
	}
}

func TestPutGetString(t *testing.T) {
	input := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x01, 0x00, 0x01}
	err1 := putString(log.NewMockLog(), input, 1, 8, "hello")
	assert.NoError(t, err1)

	result, err := getString(log.NewMockLog(), input, 1, 8)
	assert.NoError(t, err)
	assert.Equal(t, "hello", result)
}

func TestPutGetInteger(t *testing.T) {
	input := []byte{0x00, 0x00, 0x00, 0x00, 0xFF, 0x00}
	err := putInteger(log.NewMockLog(), input, 1, 256)
	assert.NoError(t, err)
	assert.Equal(t, byte(0x00), input[1])
	assert.Equal(t, byte(0x00), input[2])
	assert.Equal(t, byte(0x01), input[3])
	assert.Equal(t, byte(0x00), input[4])

	result, err2 := getInteger(log.NewMockLog(), input, 1)
	assert.NoError(t, err2)
	assert.Equal(t, int32(256), result)

	result2, err3 := getInteger(log.NewMockLog(), input, 2)
	assert.Equal(t, int32(65536), result2)
	assert.NoError(t, err3)

	result3, err4 := getInteger(mockLogger, input, 3)
	assert.Equal(t, int32(0), result3)
	assert.Error(t, err4)
}

func TestPutGetLong(t *testing.T) {
	input := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00}
	err := putLong(log.NewMockLog(), input, 1, 4294967296) // 2 to the 32 + 1
	assert.NoError(t, err)
	assert.Equal(t, byte(0x00), input[1])
	assert.Equal(t, byte(0x00), input[2])
	assert.Equal(t, byte(0x00), input[3])
	assert.Equal(t, byte(0x01), input[4])
	assert.Equal(t, byte(0x00), input[5])
	assert.Equal(t, byte(0x00), input[6])
	assert.Equal(t, byte(0x00), input[7])
	assert.Equal(t, byte(0x00), input[8])

	testLong, err2 := getLong(log.NewMockLog(), input, 1)
	assert.NoError(t, err2)
	assert.Equal(t, int64(4294967296), testLong)
}

func TestGetBytesFromInteger(t *testing.T) {
	input := int32(256)
	result, err := integerToBytes(log.NewMockLog(), input)
	assert.NoError(t, err)
	assert.Equal(t, byte(0x00), result[0])
	assert.Equal(t, byte(0x00), result[1])
	assert.Equal(t, byte(0x01), result[2])
	assert.Equal(t, byte(0x00), result[3])
}

func TestSerializeAndDeserializeClientMessage(t *testing.T) {
	u, err := uuid.Parse(messageId)
	assert.NoError(t, err)

	clientMessage := ClientMessage{
		MessageType:    messageType,
		SchemaVersion:  schemaVersion,
		CreatedDate:    createdDate,
		SequenceNumber: 1,
		Flags:          2,
		MessageId:      u,
		Payload:        payload,
	}

	// Test SerializeClientMessage
	serializedBytes, err := clientMessage.SerializeClientMessage(log.NewMockLog())
	assert.NoError(t, err, "Error serializing message")

	seralizedMessageType := strings.TrimRight(string(serializedBytes[ClientMessage_MessageTypeOffset:ClientMessage_MessageTypeOffset+ClientMessage_MessageTypeLength-1]), " ")
	assert.Equal(t, seralizedMessageType, messageType)

	serializedVersion, err := getUInteger(log.NewMockLog(), serializedBytes, ClientMessage_SchemaVersionOffset)
	assert.NoError(t, err)
	assert.Equal(t, serializedVersion, schemaVersion)

	serializedCD, err := getULong(log.NewMockLog(), serializedBytes, ClientMessage_CreatedDateOffset)
	assert.NoError(t, err)
	assert.Equal(t, serializedCD, createdDate)

	serializedSequence, err := getLong(log.NewMockLog(), serializedBytes, ClientMessage_SequenceNumberOffset)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), serializedSequence)

	serializedFlags, err := getULong(log.NewMockLog(), serializedBytes, ClientMessage_FlagsOffset)
	assert.NoError(t, err)
	assert.Equal(t, uint64(2), serializedFlags)

	seralizedMessageId, err := getUuid(log.NewMockLog(), serializedBytes, ClientMessage_MessageIdOffset)
	assert.NoError(t, err)
	assert.Equal(t, seralizedMessageId.String(), messageId)

	serializedDigest, err := getBytes(log.NewMockLog(), serializedBytes, ClientMessage_PayloadDigestOffset, ClientMessage_PayloadDigestLength)
	assert.NoError(t, err)

	hasher := sha256.New()
	hasher.Write(clientMessage.Payload)
	expectedHash := hasher.Sum(nil)
	assert.True(t, reflect.DeepEqual(serializedDigest, expectedHash))

	// Test DeserializeClientMessage
	deserializedClientMessage := &ClientMessage{}
	err = deserializedClientMessage.DeserializeClientMessage(log.NewMockLog(), serializedBytes)
	assert.NoError(t, err)
	assert.Equal(t, messageType, deserializedClientMessage.MessageType)
	assert.Equal(t, schemaVersion, deserializedClientMessage.SchemaVersion)
	assert.Equal(t, messageId, deserializedClientMessage.MessageId.String())
	assert.Equal(t, createdDate, deserializedClientMessage.CreatedDate)
	assert.Equal(t, uint64(2), deserializedClientMessage.Flags)
	assert.Equal(t, int64(1), deserializedClientMessage.SequenceNumber)
	assert.True(t, reflect.DeepEqual(payload, deserializedClientMessage.Payload))
}

func TestSerializeMessagePayloadNegative(t *testing.T) {
	functionEx := func() {}
	_, err := SerializeClientMessagePayload(mockLogger, functionEx)
	assert.Error(t, err)
}

func TestSerializeAndDeserializeClientMessageWithAcknowledgeContent(t *testing.T) {
	acknowledgeContent := AcknowledgeContent{
		MessageType:         messageType,
		MessageId:           messageId,
		SequenceNumber:      sequenceNumber,
		IsSequentialMessage: true,
	}

	serializedClientMsg, _ := SerializeClientMessageWithAcknowledgeContent(log.NewMockLog(), acknowledgeContent)
	deserializedClientMsg := &ClientMessage{}
	err := deserializedClientMsg.DeserializeClientMessage(log.NewMockLog(), serializedClientMsg)
	assert.NoError(t, err)
	deserializedAcknowledgeContent, err := deserializedClientMsg.DeserializeDataStreamAcknowledgeContent(log.NewMockLog())

	assert.NoError(t, err)
	assert.Equal(t, messageType, deserializedAcknowledgeContent.MessageType)
	assert.Equal(t, messageId, deserializedAcknowledgeContent.MessageId)
	assert.Equal(t, sequenceNumber, deserializedAcknowledgeContent.SequenceNumber)
	assert.True(t, deserializedAcknowledgeContent.IsSequentialMessage)
}

func TestDeserializeAgentMessageWithChannelClosed(t *testing.T) {
	channelClosed := ChannelClosed{
		MessageType:   ChannelClosedMessage,
		MessageId:     messageId,
		DestinationId: destinationId,
		SessionId:     sessionId,
		SchemaVersion: 1,
		CreatedDate:   "2018-01-01",
	}

	u, err := uuid.Parse(messageId)
	assert.NoError(t, err)

	channelClosedJson, err := json.Marshal(channelClosed)
	if err != nil {
		t.Fatalf("marshaling channel closed: %v", err)
	}

	agentMessage := ClientMessage{
		MessageType:    ChannelClosedMessage,
		SchemaVersion:  schemaVersion,
		CreatedDate:    createdDate,
		SequenceNumber: 1,
		Flags:          2,
		MessageId:      u,
		Payload:        channelClosedJson,
	}

	deserializedChannelClosed, err := agentMessage.DeserializeChannelClosedMessage(log.NewMockLog())

	assert.NoError(t, err)
	assert.Equal(t, ChannelClosedMessage, deserializedChannelClosed.MessageType)
	assert.Equal(t, messageId, deserializedChannelClosed.MessageId)
	assert.Equal(t, sessionId, deserializedChannelClosed.SessionId)
	assert.Equal(t, "destination-id", deserializedChannelClosed.DestinationId)
}
