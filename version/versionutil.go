// Package version contains CLI version constant and utilities.
package version

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Number represents a semantic version number.
type Number struct {
	version []string
}

// ErrInvalidVersion is returned when the version string is invalid.
var ErrInvalidVersion = errors.New("invalid version")

// ErrLengthMismatch is returned when the length of two version strings do not match.
var ErrLengthMismatch = errors.New("length mismatch between versions")

// NewVersion initializes Number struct by splitting given version string into string list using separator ".".
func NewVersion(versionString string) (Number, error) {
	if versionString == "" {
		return Number{}, fmt.Errorf("%w: %s", ErrInvalidVersion, versionString)
	}

	return Number{
		strings.Split(versionString, "."),
	}, nil
}

// compare returns 0 if thisVersion is equal to otherVersion, 1 if thisVersion is greater than otherVersion, -1 otherwise.
func (thisVersion Number) compare(otherVersion Number) (int, error) {
	if len(thisVersion.version) != len(otherVersion.version) {
		return -1, fmt.Errorf("%w: length mismatch for versions %v and %v", ErrLengthMismatch, thisVersion.version, otherVersion.version)
	}

	var (
		thisVersionSlice  int
		otherVersionSlice int
		err               error
	)

	for i := range thisVersion.version {
		if thisVersionSlice, err = strconv.Atoi(thisVersion.version[i]); err != nil {
			return -1, fmt.Errorf("converting version slice to integer: %w", err)
		}

		if otherVersionSlice, err = strconv.Atoi(otherVersion.version[i]); err != nil {
			return -1, fmt.Errorf("converting other version slice to integer: %w", err)
		}

		if thisVersionSlice > otherVersionSlice {
			return 1, nil
		} else if thisVersionSlice < otherVersionSlice {
			return -1, nil
		}
	}

	return 0, nil
}
