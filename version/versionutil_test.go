// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package version contains version constants and utilities.
package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompareWhenVersionsAreSame(t *testing.T) {
	thisVersion, err := NewVersion("2.3.617.9")
	require.NoError(t, err)

	otherVersion, err := NewVersion("2.3.617.9")
	require.NoError(t, err)

	actual, err := thisVersion.compare(otherVersion)
	require.NoError(t, err)
	assert.Equal(t, 0, actual)
}

func TestCompareWhenThisVersionIsGreaterThanOtherVersion(t *testing.T) {
	thisVersion, err := NewVersion("2.3.617.9")
	require.NoError(t, err)

	otherVersion, err := NewVersion("2.3.56.9")
	require.NoError(t, err)

	actual, err := thisVersion.compare(otherVersion)
	require.NoError(t, err)
	assert.Equal(t, 1, actual)
}

func TestCompareWhenThisVersionIsLesserThanOtherVersion(t *testing.T) {
	thisVersion, err := NewVersion("2.3.617.9")
	require.NoError(t, err)

	otherVersion, err := NewVersion("2.10.763.9")
	require.NoError(t, err)

	actual, err := thisVersion.compare(otherVersion)
	require.NoError(t, err)
	assert.Equal(t, -1, actual)
}

func TestCompareWhenVersionsLengthMismatch(t *testing.T) {
	thisVersion, err := NewVersion("2.3.56.0")
	require.NoError(t, err)

	otherVersion, err := NewVersion("2.5.45")
	require.NoError(t, err)

	_, err = thisVersion.compare(otherVersion)
	require.Error(t, err)
}

func TestNewVersion(t *testing.T) {
	version, err := NewVersion("2.3.525.0")
	require.NoError(t, err)
	assert.Equal(t, []string{"2", "3", "525", "0"}, version.version)
}

func TestNewVersionWhenGivenVersionIsEmptyString(t *testing.T) {
	_, err := NewVersion("")
	require.Error(t, err)
}
