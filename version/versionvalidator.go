// Package version contains version constants and utilities.
package version

import (
	"fmt"

	"github.com/steveh/ecstoolkit/config"
)

// DoesAgentSupportTCPMultiplexing returns true if given agentVersion supports TCP multiplexing in port plugin, false otherwise.
func DoesAgentSupportTCPMultiplexing(agentVersion string) (bool, error) {
	return isAgentVersionGreaterThanSupportedVersion(agentVersion, config.TCPMultiplexingSupportedAfterThisAgentVersion)
}

// DoesAgentSupportDisableSmuxKeepAlive returns true if given agentVersion disables smux KeepAlive in TCP multiplexing in port plugin, false otherwise.
func DoesAgentSupportDisableSmuxKeepAlive(agentVersion string) (bool, error) {
	return isAgentVersionGreaterThanSupportedVersion(agentVersion, config.TCPMultiplexingWithSmuxKeepAliveDisabledAfterThisAgentVersion)
}

// DoesAgentSupportTerminateSessionFlag returns true if given agentVersion supports TerminateSession flag, false otherwise.
func DoesAgentSupportTerminateSessionFlag(agentVersion string) (bool, error) {
	return isAgentVersionGreaterThanSupportedVersion(agentVersion, config.TerminateSessionFlagSupportedAfterThisAgentVersion)
}

// isAgentVersionGreaterThanSupportedVersion returns true if agentVersion is greater than supportedVersion,
// false in case of any error and agentVersion is equalTo or less than supportedVersion.
func isAgentVersionGreaterThanSupportedVersion(agentVersionString string, supportedVersionString string) (bool, error) {
	supportedVersion, err := NewVersion(supportedVersionString)
	if err != nil {
		return false, fmt.Errorf("initializing supported version: %w", err)
	}

	agentVersion, err := NewVersion(agentVersionString)
	if err != nil {
		return false, fmt.Errorf("initializing agent version: %w", err)
	}

	compareResult, err := agentVersion.compare(supportedVersion)
	if err != nil {
		return false, fmt.Errorf("comparing agent version with supported version: %w", err)
	}

	return (compareResult == 1), nil
}
