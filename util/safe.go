// Package util provides utility functions.
package util

import (
	"errors"
	"math"
)

// SafeUint32 converts any integer type to uint32, returning an error if it would overflow.
func SafeUint32[T ~int | ~int32 | ~int64 | ~uint | ~uint32 | ~uint64](value T) (uint32, error) {
	if value < 0 {
		return 0, errors.New("cannot convert negative value to uint32")
	}

	if int(value) > math.MaxUint32 {
		return 0, errors.New("value exceeds uint32 range")
	}

	return uint32(value), nil
}

// SafeInt32 converts any integer type to int32, returning an error if it would overflow.
func SafeInt32[T ~int | ~int32 | ~int64 | ~uint | ~uint32 | ~uint64](value T) (int32, error) {
	if int(value) > math.MaxInt32 {
		return 0, errors.New("value exceeds int32 range")
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
			return 0, errors.New("value exceeds int64 range")
		}
	}

	return int64(value), nil
}
