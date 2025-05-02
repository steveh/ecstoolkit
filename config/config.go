// Package config provides configuration constants for the application.
package config

import "time"

const (
	// RolePublishSubscribe defines the role for publish/subscribe functionality.
	RolePublishSubscribe = "publish_subscribe"
	// MessageSchemaVersion defines the version of the message schema used for communication.
	MessageSchemaVersion = "1.0"
	// DefaultTransmissionTimeout defines the default timeout duration for message transmission.
	DefaultTransmissionTimeout = 200 * time.Millisecond
	// DefaultRoundTripTime defines the default round trip time for message transmission.
	DefaultRoundTripTime = 100 * time.Millisecond
	// DefaultRoundTripTimeVariation defines the default variation in round trip time for message transmission.
	DefaultRoundTripTimeVariation = 0
	// ResendSleepInterval defines the interval between message resend attempts.
	ResendSleepInterval = 100 * time.Millisecond
	// ResendMaxAttempt defines the maximum number of message resend attempts (5 minutes / ResendSleepInterval).
	ResendMaxAttempt = 3000 // 5 minutes / ResendSleepInterval
	// StreamDataPayloadSize is the maximum size of a stream data payload in bytes.
	StreamDataPayloadSize = 1024
	// OutgoingMessageBufferCapacity is the maximum number of messages that can be stored in the outgoing message buffer.
	OutgoingMessageBufferCapacity = 10000
	// IncomingMessageBufferCapacity is the maximum number of messages that can be stored in the incoming message buffer.
	IncomingMessageBufferCapacity = 10000
	// RTTConstant is used to calculate the round trip time.
	RTTConstant = 1.0 / 8.0
	// RTTVConstant is used to calculate the variation in round trip time.
	RTTVConstant = 1.0 / 4.0
	// ClockGranularity is the granularity of the clock used for timing operations.
	ClockGranularity = 10 * time.Millisecond
	// MaxTransmissionTimeout is the maximum timeout duration for message transmission.
	MaxTransmissionTimeout = 1 * time.Second
	// RetryBase is the base value for retry attempts.
	RetryBase = 2
	// DataChannelNumMaxRetries is the maximum number of retries for data channel operations.
	DataChannelNumMaxRetries = 5
	// DataChannelRetryInitialDelayMillis is the initial delay in milliseconds before retrying data channel operations.
	DataChannelRetryInitialDelayMillis = 100
	// DataChannelRetryMaxIntervalMillis is the maximum interval in milliseconds between retries for data channel operations.
	DataChannelRetryMaxIntervalMillis = 5000
	// RetryAttempt is the maximum number of retry attempts for operations.
	RetryAttempt = 5
	// PingTimeInterval is the interval between ping messages sent to the server.
	PingTimeInterval = 5 * time.Minute

	// ShellPluginName is the name of the shell plugin for standard stream operations.
	ShellPluginName = "Standard_Stream"
	// PortPluginName is the name of the port plugin for port forwarding operations.
	PortPluginName = "Port"
	// InteractiveCommandsPluginName is the name of the interactive commands plugin for interactive shell operations.
	InteractiveCommandsPluginName = "InteractiveCommands"
	// NonInteractiveCommandsPluginName is the name of the non-interactive commands plugin for non-interactive shell operations.
	NonInteractiveCommandsPluginName = "NonInteractiveCommands"

	// TerminateSessionFlagSupportedAfterThisAgentVersion is the minimum agent version that supports session termination flags.
	TerminateSessionFlagSupportedAfterThisAgentVersion = "2.3.722.0"

	// TCPMultiplexingSupportedAfterThisAgentVersion is the minimum agent version that supports TCP multiplexing.
	TCPMultiplexingSupportedAfterThisAgentVersion = "3.0.196.0"

	// TCPMultiplexingWithSmuxKeepAliveDisabledAfterThisAgentVersion is the minimum agent version that supports TCP multiplexing with SMUX keep-alive disabled.
	TCPMultiplexingWithSmuxKeepAliveDisabledAfterThisAgentVersion = "3.1.1511.0"
)
