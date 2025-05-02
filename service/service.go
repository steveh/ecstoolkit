// Package service is a wrapper for the new Service
package service

// OpenDataChannelInput represents the input parameters for opening a data channel.
type OpenDataChannelInput struct {
	_ struct{} `type:"structure"`

	// MessageSchemaVersion is a required field
	MessageSchemaVersion *string `json:"MessageSchemaVersion" min:"1" required:"true" type:"string"`

	// RequestID is a required field
	RequestID *string `json:"RequestId" min:"16" required:"true" type:"string"`

	// TokenValue is a required field
	TokenValue *string `json:"TokenValue" min:"1" required:"true" type:"string"`

	// ClientID is a required field
	ClientID *string `json:"ClientId" min:"1" required:"true" type:"string"`

	// ClientVersion is a required field
	ClientVersion *string `json:"ClientVersion" min:"1" required:"true" type:"string"`
}
