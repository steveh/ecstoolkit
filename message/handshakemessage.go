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
	"time"
)

// ActionType used in Handshake to determine action requested by the agent.
type ActionType string

const (
	// KMSEncryption represents the KMS encryption action type.
	KMSEncryption ActionType = "KMSEncryption"
	// SessionType represents the session type action type.
	SessionType ActionType = "SessionType"
)

// ActionStatus represents the status of a client action.
type ActionStatus int

const (
	// Success indicates that the action was successful.
	Success ActionStatus = 1
	// Failed indicates that the action failed.
	Failed ActionStatus = 2
	// Unsupported indicates that the action is not supported.
	Unsupported ActionStatus = 3
)

// KMSEncryptionRequest represents a request to initialize KMS encryption.
type KMSEncryptionRequest struct {
	KMSKeyID string `json:"KMSKeyId"`
}

// KMSEncryptionResponse represents the response containing KMS encryption setup data.
type KMSEncryptionResponse struct {
	KMSCipherTextKey  []byte `json:"KMSCipherTextKey"`
	KMSCipherTextHash []byte `json:"KMSCipherTextHash"`
}

// SessionTypeRequest represents a request containing session type and plugin properties.
type SessionTypeRequest struct {
	SessionType string      `json:"SessionType"`
	Properties  interface{} `json:"Properties"`
}

// HandshakeRequestPayload represents the handshake payload sent by the agent to the session manager plugin.
type HandshakeRequestPayload struct {
	AgentVersion           string                  `json:"AgentVersion"`
	RequestedClientActions []RequestedClientAction `json:"RequestedClientActions"`
}

// RequestedClientAction represents an action requested by the agent to the plugin.
type RequestedClientAction struct {
	ActionType       ActionType      `json:"ActionType"`
	ActionParameters json.RawMessage `json:"ActionParameters"`
}

// ProcessedClientAction represents the result of processing an action by the plugin.
type ProcessedClientAction struct {
	ActionType   ActionType   `json:"ActionType"`
	ActionStatus ActionStatus `json:"ActionStatus"`
	ActionResult interface{}  `json:"ActionResult"`
	Error        string       `json:"Error"`
}

// HandshakeResponsePayload represents the response sent by the plugin in response to the handshake request.
type HandshakeResponsePayload struct {
	ClientVersion          string                  `json:"ClientVersion"`
	ProcessedClientActions []ProcessedClientAction `json:"ProcessedClientActions"`
	Errors                 []string                `json:"Errors"`
}

// EncryptionChallengeRequest represents a challenge sent by the agent to the client.
type EncryptionChallengeRequest struct {
	Challenge []byte `json:"Challenge"`
}

// EncryptionChallengeResponse represents the client's response to the encryption challenge.
type EncryptionChallengeResponse struct {
	Challenge []byte `json:"Challenge"`
}

// HandshakeCompletePayload represents the completion of the handshake process.
type HandshakeCompletePayload struct {
	HandshakeTimeToComplete time.Duration `json:"HandshakeTimeToComplete"`
	CustomerMessage         string        `json:"CustomerMessage"`
}
