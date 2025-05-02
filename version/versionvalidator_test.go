// Package version contains version constants and utilities.
package version

import (
	"testing"

	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/log"
	"github.com/stretchr/testify/assert"
)

type Comparison struct {
	agentVersion     string
	supportedVersion string
}

func TestDoesAgentSupportTCPMultiplexing(t *testing.T) {
	t.Parallel()

	mockLogger := log.NewMockLog()

	// Test exact version of feature; TCPMultiplexingSupported after 3.0.196.0
	assert.False(t, DoesAgentSupportTCPMultiplexing(mockLogger, config.TCPMultiplexingSupportedAfterThisAgentVersion))

	// Test versions prior to feature implementation
	oldVersions := []string{
		"1.2.3.4",
		"2.3.4.5",
		"3.0.195.100",
		"2.99.1000.0",
	}
	for _, tc := range oldVersions {
		assert.False(t, DoesAgentSupportTCPMultiplexing(mockLogger, tc))
	}

	// Test versions after feature implementation
	newVersions := []string{
		"3.1.0.0",
		"3.0.197.0",
		"3.0.196.1",
		"4.0.0.0",
	}
	for _, tc := range newVersions {
		assert.True(t, DoesAgentSupportTCPMultiplexing(mockLogger, tc))
	}
}

func TestDoesAgentSupportTerminateSessionFlag(t *testing.T) {
	t.Parallel()

	mockLogger := log.NewMockLog()

	// Test exact version of feature; TerminateSessionFlag supported after 2.3.722.0
	assert.False(t, DoesAgentSupportTerminateSessionFlag(mockLogger, config.TerminateSessionFlagSupportedAfterThisAgentVersion))

	// Test versions prior to feature implementation
	oldVersions := []string{
		"1.2.3.4",
		"2.3.4.5",
		"2.3.721.100",
		"0.3.1000.0",
	}
	for _, tc := range oldVersions {
		assert.False(t, DoesAgentSupportTerminateSessionFlag(mockLogger, tc))
	}

	// Test versions after feature implementation
	newVersions := []string{
		"3.1.0.0",
		"3.0.197.0",
		"2.3.723.0",
		"4.0.0.0",
	}
	for _, tc := range newVersions {
		assert.True(t, DoesAgentSupportTerminateSessionFlag(mockLogger, tc))
	}
}

func TestIsAgentVersionGreaterThanSupportedVersionWithNormalInputs(t *testing.T) {
	t.Parallel()

	const (
		defaultSupportedVersion = "3.0.0.0"
	)

	mockLogger := log.NewMockLog()

	// Test normal inputs where agentVersion <= supportedVersion
	normalNegativeCases := []Comparison{
		{"3.0.0.0", defaultSupportedVersion},
		{"1.2.3.4", defaultSupportedVersion},
		{"2.99.99.99", defaultSupportedVersion},
		{"3.4.5.2", "3.4.5.2"},
	}
	for _, tc := range normalNegativeCases {
		assert.False(t, isAgentVersionGreaterThanSupportedVersion(mockLogger, tc.agentVersion, tc.supportedVersion))
	}

	// Test normal inputs where agentVersion > supportedVersion
	normalPositiveCases := []Comparison{
		{"3.0.0.1", defaultSupportedVersion},
		{"4.0.0.0", defaultSupportedVersion},
		{"3.1.0.0", defaultSupportedVersion},
		{"3.0.100.0", defaultSupportedVersion},
		{"5.0.0.2", "5.0.0.0"},
	}
	for _, tc := range normalPositiveCases {
		assert.True(t, isAgentVersionGreaterThanSupportedVersion(mockLogger, tc.agentVersion, tc.supportedVersion))
	}
}

func TestIsAgentVersionGreaterThanSupportedVersionEdgeCases(t *testing.T) {
	t.Parallel()

	// Test non-numeric strings
	t.Run("Non-numeric strings", func(t *testing.T) {
		t.Parallel()

		errorLog := log.NewMockLog()
		notNumberCase := Comparison{"randomString", "randomString"}
		assert.False(t, isAgentVersionGreaterThanSupportedVersion(errorLog, notNumberCase.agentVersion, notNumberCase.supportedVersion))
	})
	t.Run("Uneven-length strings", func(t *testing.T) {
		t.Parallel()

		errorLog := log.NewMockLog()
		unevenLengthCase := Comparison{"1.4.1.2.4.1", "3.0.0.0"}
		assert.False(t, isAgentVersionGreaterThanSupportedVersion(errorLog, unevenLengthCase.agentVersion, unevenLengthCase.supportedVersion))
	})
	t.Run("Invalid Version Numbers", func(t *testing.T) {
		t.Parallel()

		errorLog := log.NewMockLog()
		invalidVersionNumberCases := []Comparison{
			{"", "3.0.0.0"},
			{"3.0.0.0", ""},
			{"3,0.0.0", "3.0.2.0"},
		}

		for _, tc := range invalidVersionNumberCases {
			assert.False(t, isAgentVersionGreaterThanSupportedVersion(errorLog, tc.agentVersion, tc.supportedVersion))
		}
	})
}

func TestDoesAgentSupportTerminateSessionFlagForSupportedScenario(t *testing.T) {
	t.Parallel()

	mockLogger := log.NewMockLog()

	assert.True(t, DoesAgentSupportTerminateSessionFlag(mockLogger, "2.3.750.0"))
}

func TestDoesAgentSupportTerminateSessionFlagForNotSupportedScenario(t *testing.T) {
	t.Parallel()

	mockLogger := log.NewMockLog()

	assert.False(t, DoesAgentSupportTerminateSessionFlag(mockLogger, "2.3.614.0"))
}

func TestDoesAgentSupportTerminateSessionFlagWhenAgentVersionIsEqualSupportedAfterVersion(t *testing.T) {
	t.Parallel()

	mockLogger := log.NewMockLog()

	assert.False(t, DoesAgentSupportTerminateSessionFlag(mockLogger, "2.3.722.0"))
}

func TestDoesAgentSupportDisableSmuxKeepAliveForNotSupportedScenario(t *testing.T) {
	t.Parallel()

	mockLogger := log.NewMockLog()

	assert.False(t, DoesAgentSupportDisableSmuxKeepAlive(mockLogger, "3.1.1476.0"))
}

func TestDoesAgentSupportDisableSmuxKeepAliveForSupportedScenario(t *testing.T) {
	t.Parallel()

	mockLogger := log.NewMockLog()

	assert.True(t, DoesAgentSupportDisableSmuxKeepAlive(mockLogger, "3.1.1600.0"))
}
