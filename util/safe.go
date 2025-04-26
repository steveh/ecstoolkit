// Package util provides utility functions.
package util

import (
	"errors"
	"fmt"
	"math"
)

var (
	// ErrNegativeValue indicates that a negative value was provided where it is not allowed.
	ErrNegativeValue = errors.New("negative value provided")

	// ErrOverflow indicates that a value exceeds the maximum limit for the target type.
	ErrOverflow = errors.New("value exceeds maximum limit for target type")
)

// SafeUint32 converts any integer type to uint32, returning an error if it would overflow.
func SafeUint32[T ~int | ~int32 | ~int64 | ~uint | ~uint32 | ~uint64](value T) (uint32, error) {
	if value < 0 {
		return 0, fmt.Errorf("%w: cannot convert negative value %d to unsigned integer", ErrNegativeValue, value)
	}

	if int(value) > math.MaxUint32 {
		return 0, fmt.Errorf("%w: value %d exceeds uint32 range", ErrOverflow, value)
	}

	return uint32(value), nil
}

// SafeInt32 converts any integer type to int32, returning an error if it would overflow.
func SafeInt32[T ~int | ~int32 | ~int64 | ~uint | ~uint32 | ~uint64](value T) (int32, error) {
	if int(value) > math.MaxInt32 {
		return 0, fmt.Errorf("%w: value %d exceeds int32 range", ErrOverflow, value)
	}

	return int32(value), nil
}

// SafeInt64 converts any integer type to int64, returning an error if it would overflow.
func SafeInt64[T ~int | ~int32 | ~int64 | ~uint | ~uint32 | ~uint64](value T) (int64, error) {
	switch any(value).(type) {
	case int, int32, int64:
		break
	case uint, uint32, uint64:
		if uint(value) > uint(math.MaxInt64) {
			return 0, fmt.Errorf("%w: value %d exceeds int64 range", ErrOverflow, value)
		}
	}

	return int64(value), nil
}
