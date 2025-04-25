package message

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const (
	integerLength = 4
	longLength    = 8
	uuidLength    = 16
)

// GetString gets a string value from the byte array starting from the specified offset to the defined length.
func GetString(byteArray []byte, offset int, stringLength int) (string, error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+stringLength-1 > byteArrayLength-1 || offset < 0 {
		return "", fmt.Errorf("%w: offset %d, stringLength %d, byteArrayLength %d", ErrOffsetOutside, offset, stringLength, byteArrayLength)
	}

	// remove nulls from the bytes array
	b := bytes.Trim(byteArray[offset:offset+stringLength], "\x00")

	return strings.TrimSpace(string(b)), nil
}

// GetUInteger gets an unsigned integer.
func GetUInteger(byteArray []byte, offset int) (uint32, error) {
	temp, err := GetInteger(byteArray, offset)
	if err != nil {
		return 0, err
	}

	if temp < 0 {
		return 0, fmt.Errorf("%w: cannot convert negative value %d to uint", ErrNegative, temp)
	}

	return uint32(temp), err
}

// GetInteger gets an integer value from a byte array starting from the specified offset.
func GetInteger(byteArray []byte, offset int) (int32, error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+integerLength > byteArrayLength || offset < 0 {
		return 0, fmt.Errorf("%w: offset %d, integerLength %d, byteArrayLength %d", ErrOffsetOutside, offset, integerLength, byteArrayLength)
	}

	return bytesToInteger(byteArray[offset : offset+integerLength])
}

func bytesToInteger(input []byte) (int32, error) {
	inputLength := len(input)
	if inputLength != integerLength {
		return 0, fmt.Errorf("%w: input slice not %d items", ErrOffsetOutside, integerLength)
	}

	buf := bytes.NewBuffer(input)

	var res int32
	if err := binary.Read(buf, binary.BigEndian, &res); err != nil {
		return 0, fmt.Errorf("reading int from bytes: %w", err)
	}

	return res, nil
}

// GetULong gets an unsigned long integer.
func GetULong(byteArray []byte, offset int) (uint64, error) {
	temp, err := GetLong(byteArray, offset)
	if err != nil {
		return 0, err
	}

	if temp < 0 {
		return 0, fmt.Errorf("cannot convert negative value %d to ulong", temp)
	}

	return uint64(temp), err
}

// GetLong gets a long integer value from a byte array starting from the specified offset. 64 bit.
func GetLong(byteArray []byte, offset int) (int64, error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+longLength > byteArrayLength || offset < 0 {
		return 0, fmt.Errorf("%w: offset %d, longLength %d, byteArrayLength %d", ErrOffsetOutside, offset, longLength, byteArrayLength)
	}

	return bytesToLong(byteArray[offset : offset+longLength])
}

// bytesToLong gets a Long integer from a byte array.
func bytesToLong(input []byte) (int64, error) {
	inputLength := len(input)
	if inputLength != longLength {
		return 0, fmt.Errorf("%w: input slice not %d items", ErrOffsetOutside, longLength)
	}

	buf := bytes.NewBuffer(input)

	var res int64
	if err := binary.Read(buf, binary.BigEndian, &res); err != nil {
		return 0, fmt.Errorf("reading long from bytes: %w", err)
	}

	return res, nil
}

// GetUUID gets the 128bit uuid from an array of bytes starting from the offset.
func GetUUID(byteArray []byte, offset int) (uuid.UUID, error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+uuidLength-1 > byteArrayLength-1 || offset < 0 {
		return uuid.Nil, fmt.Errorf("%w: offset %d, uuidLength %d, byteArrayLength %d", ErrOffsetOutside, offset, uuidLength, byteArrayLength)
	}

	leastSignificantLong, err := GetLong(byteArray, offset)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: getting LSB long value", ErrOffsetOutside)
	}

	leastSignificantBytes, err := LongToBytes(leastSignificantLong)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: getting LSB bytes value", ErrOffsetOutside)
	}

	mostSignificantLong, err := GetLong(byteArray, offset+longLength)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: getting MSB long value", ErrOffsetOutside)
	}

	mostSignificantBytes, err := LongToBytes(mostSignificantLong)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: getting MSB bytes value", ErrOffsetOutside)
	}

	mostSignificantBytes = append(mostSignificantBytes, leastSignificantBytes...)

	result, err := uuid.FromBytes(mostSignificantBytes)
	if err != nil {
		return uuid.Nil, fmt.Errorf("creating UUID from bytes: %w", err)
	}

	return result, nil
}

// LongToBytes gets bytes array from a long integer.
func LongToBytes(input int64) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, input); err != nil {
		return nil, fmt.Errorf("writing long to bytes: %w", err)
	}

	if buf.Len() != longLength {
		return make([]byte, longLength), fmt.Errorf("%w: buffer output length not equal to %d", ErrOffsetOutside, longLength)
	}

	return buf.Bytes(), nil
}

// GetBytes gets an array of bytes starting from the offset.
func GetBytes(byteArray []byte, offset int, byteLength int) ([]byte, error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+byteLength-1 > byteArrayLength-1 || offset < 0 {
		return make([]byte, byteLength), fmt.Errorf("%w: offset %d, byteLength %d, byteArrayLength %d", ErrOffsetOutside, offset, byteLength, byteArrayLength)
	}

	return byteArray[offset : offset+byteLength], nil
}
