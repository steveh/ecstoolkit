// Package version contains version constants and utilities.
package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompareWhenVersionsAreSame(t *testing.T) {
	t.Parallel()

	thisVersion, err := NewVersion("2.3.617.9")
	require.NoError(t, err)

	otherVersion, err := NewVersion("2.3.617.9")
	require.NoError(t, err)

	actual, err := thisVersion.compare(otherVersion)
	require.NoError(t, err)
	assert.Equal(t, 0, actual)
}

func TestCompareWhenThisVersionIsGreaterThanOtherVersion(t *testing.T) {
	t.Parallel()

	thisVersion, err := NewVersion("2.3.617.9")
	require.NoError(t, err)

	otherVersion, err := NewVersion("2.3.56.9")
	require.NoError(t, err)

	actual, err := thisVersion.compare(otherVersion)
	require.NoError(t, err)
	assert.Equal(t, 1, actual)
}

func TestCompareWhenThisVersionIsLesserThanOtherVersion(t *testing.T) {
	t.Parallel()

	thisVersion, err := NewVersion("2.3.617.9")
	require.NoError(t, err)

	otherVersion, err := NewVersion("2.10.763.9")
	require.NoError(t, err)

	actual, err := thisVersion.compare(otherVersion)
	require.NoError(t, err)
	assert.Equal(t, -1, actual)
}

func TestCompareWhenVersionsLengthMismatch(t *testing.T) {
	t.Parallel()

	thisVersion, err := NewVersion("2.3.56.0")
	require.NoError(t, err)

	otherVersion, err := NewVersion("2.5.45")
	require.NoError(t, err)

	_, err = thisVersion.compare(otherVersion)
	require.Error(t, err)
}

func TestNewVersion(t *testing.T) {
	t.Parallel()

	version, err := NewVersion("2.3.525.0")
	require.NoError(t, err)
	assert.Equal(t, []string{"2", "3", "525", "0"}, version.version)
}

func TestNewVersionWhenGivenVersionIsEmptyString(t *testing.T) {
	t.Parallel()

	_, err := NewVersion("")
	require.Error(t, err)
}
