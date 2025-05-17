// Package version contains version constants and utilities.
package version

import (
	"testing"

	"github.com/steveh/ecstoolkit/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Comparison struct {
	agentVersion     string
	supportedVersion string
}

func TestDoesAgentSupportTCPMultiplexing(t *testing.T) {
	t.Parallel()

	// Test exact version of feature; TCPMultiplexingSupported after 3.0.196.0
	v, err := DoesAgentSupportTCPMultiplexing(config.TCPMultiplexingSupportedAfterThisAgentVersion)
	require.NoError(t, err)
	assert.False(t, v)

	// Test versions prior to feature implementation
	oldVersions := []string{
		"1.2.3.4",
		"2.3.4.5",
		"3.0.195.100",
		"2.99.1000.0",
	}
	for _, tc := range oldVersions {
		v, err := DoesAgentSupportTCPMultiplexing(tc)
		require.NoError(t, err)
		assert.False(t, v)
	}

	// Test versions after feature implementation
	newVersions := []string{
		"3.1.0.0",
		"3.0.197.0",
		"3.0.196.1",
		"4.0.0.0",
	}
	for _, tc := range newVersions {
		v, err := DoesAgentSupportTCPMultiplexing(tc)
		require.NoError(t, err)
		assert.True(t, v)
	}
}

func TestDoesAgentSupportTerminateSessionFlag(t *testing.T) {
	t.Parallel()

	// Test exact version of feature; TerminateSessionFlag supported after 2.3.722.0
	v, err := DoesAgentSupportTerminateSessionFlag(config.TerminateSessionFlagSupportedAfterThisAgentVersion)
	require.NoError(t, err)
	assert.False(t, v)

	// Test versions prior to feature implementation
	oldVersions := []string{
		"1.2.3.4",
		"2.3.4.5",
		"2.3.721.100",
		"0.3.1000.0",
	}
	for _, tc := range oldVersions {
		v, err := DoesAgentSupportTerminateSessionFlag(tc)
		require.NoError(t, err)
		assert.False(t, v)
	}

	// Test versions after feature implementation
	newVersions := []string{
		"3.1.0.0",
		"3.0.197.0",
		"2.3.723.0",
		"4.0.0.0",
	}
	for _, tc := range newVersions {
		v, err := DoesAgentSupportTerminateSessionFlag(tc)
		require.NoError(t, err)
		assert.True(t, v)
	}
}

func TestIsAgentVersionGreaterThanSupportedVersionWithNormalInputs(t *testing.T) {
	t.Parallel()

	const (
		defaultSupportedVersion = "3.0.0.0"
	)

	// Test normal inputs where agentVersion <= supportedVersion
	normalNegativeCases := []Comparison{
		{"3.0.0.0", defaultSupportedVersion},
		{"1.2.3.4", defaultSupportedVersion},
		{"2.99.99.99", defaultSupportedVersion},
		{"3.4.5.2", "3.4.5.2"},
	}
	for _, tc := range normalNegativeCases {
		v, err := isAgentVersionGreaterThanSupportedVersion(tc.agentVersion, tc.supportedVersion)
		require.NoError(t, err)
		assert.False(t, v)
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
		v, err := isAgentVersionGreaterThanSupportedVersion(tc.agentVersion, tc.supportedVersion)
		require.NoError(t, err)
		assert.True(t, v)
	}
}

func TestIsAgentVersionGreaterThanSupportedVersionEdgeCases(t *testing.T) {
	t.Parallel()

	// Test non-numeric strings
	t.Run("Non-numeric strings", func(t *testing.T) {
		t.Parallel()

		notNumberCase := Comparison{"randomString", "randomString"}
		v, err := isAgentVersionGreaterThanSupportedVersion(notNumberCase.agentVersion, notNumberCase.supportedVersion)
		require.Error(t, err)
		assert.False(t, v)
	})

	t.Run("Uneven-length strings", func(t *testing.T) {
		t.Parallel()

		unevenLengthCase := Comparison{"1.4.1.2.4.1", "3.0.0.0"}
		v, err := isAgentVersionGreaterThanSupportedVersion(unevenLengthCase.agentVersion, unevenLengthCase.supportedVersion)
		require.Error(t, err)
		assert.False(t, v)
	})

	t.Run("Invalid Version Numbers", func(t *testing.T) {
		t.Parallel()

		invalidVersionNumberCases := []Comparison{
			{"", "3.0.0.0"},
			{"3.0.0.0", ""},
			{"3,0.0.0", "3.0.2.0"},
		}

		for _, tc := range invalidVersionNumberCases {
			v, err := isAgentVersionGreaterThanSupportedVersion(tc.agentVersion, tc.supportedVersion)
			require.Error(t, err)
			assert.False(t, v)
		}
	})
}

func TestDoesAgentSupportTerminateSessionFlagForSupportedScenario(t *testing.T) {
	t.Parallel()

	v, err := DoesAgentSupportTerminateSessionFlag("2.3.750.0")
	require.NoError(t, err)
	assert.True(t, v)
}

func TestDoesAgentSupportTerminateSessionFlagForNotSupportedScenario(t *testing.T) {
	t.Parallel()

	v, err := DoesAgentSupportTerminateSessionFlag("2.3.614.0")
	require.NoError(t, err)
	assert.False(t, v)
}

func TestDoesAgentSupportTerminateSessionFlagWhenAgentVersionIsEqualSupportedAfterVersion(t *testing.T) {
	t.Parallel()

	v, err := DoesAgentSupportTerminateSessionFlag("2.3.722.0")
	require.NoError(t, err)
	assert.False(t, v)
}

func TestDoesAgentSupportDisableSmuxKeepAliveForNotSupportedScenario(t *testing.T) {
	t.Parallel()

	v, err := DoesAgentSupportDisableSmuxKeepAlive("3.1.1476.0")
	require.NoError(t, err)
	assert.False(t, v)
}

func TestDoesAgentSupportDisableSmuxKeepAliveForSupportedScenario(t *testing.T) {
	t.Parallel()

	v, err := DoesAgentSupportDisableSmuxKeepAlive("3.1.1600.0")
	require.NoError(t, err)
	assert.True(t, v)
}
