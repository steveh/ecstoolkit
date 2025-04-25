package message

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/steveh/ecstoolkit/log"
)

// GetString gets a string value from the byte array starting from the specified offset to the defined length.
func GetString(log log.T, byteArray []byte, offset int, stringLength int) (string, error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+stringLength-1 > byteArrayLength-1 || offset < 0 {
		log.Error("GetString failed: Offset is invalid.")

		return "", ErrOffsetOutsideByteArray
	}

	// remove nulls from the bytes array
	b := bytes.Trim(byteArray[offset:offset+stringLength], "\x00")

	return strings.TrimSpace(string(b)), nil
}

// GetUInteger gets an unsigned integer.
func GetUInteger(log log.T, byteArray []byte, offset int) (uint32, error) {
	temp, err := GetInteger(log, byteArray, offset)
	if err != nil {
		return 0, err
	}

	if temp < 0 {
		return 0, fmt.Errorf("cannot convert negative value %d to uint32", temp)
	}

	return uint32(temp), err
}

// GetInteger gets an integer value from a byte array starting from the specified offset.
func GetInteger(log log.T, byteArray []byte, offset int) (int32, error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+4 > byteArrayLength || offset < 0 {
		log.Error("GetInteger failed: Offset is invalid.")

		return 0, ErrOffsetOutside
	}

	return bytesToInteger(log, byteArray[offset:offset+4])
}

// bytesToInteger gets an integer from a byte array.
func bytesToInteger(log log.T, input []byte) (int32, error) {
	var res int32

	inputLength := len(input)
	if inputLength != 4 { //nolint:mnd
		log.Error("bytesToInteger failed: input array size is not equal to 4.")

		return 0, ErrOffsetOutside
	}

	buf := bytes.NewBuffer(input)
	if err := binary.Read(buf, binary.BigEndian, &res); err != nil {
		return 0, fmt.Errorf("reading integer from bytes: %w", err)
	}

	return res, nil
}

// GetULong gets an unsigned long integer.
func GetULong(log log.T, byteArray []byte, offset int) (uint64, error) {
	temp, err := GetLong(log, byteArray, offset)
	if err != nil {
		return 0, err
	}

	if temp < 0 {
		return 0, fmt.Errorf("cannot convert negative value %d to uint64", temp)
	}

	return uint64(temp), err
}

// GetLong gets a long integer value from a byte array starting from the specified offset. 64 bit.
func GetLong(log log.T, byteArray []byte, offset int) (int64, error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+8 > byteArrayLength || offset < 0 {
		log.Error("GetLong failed: Offset is invalid.")

		return 0, ErrOffsetOutsideByteArray
	}

	return bytesToLong(log, byteArray[offset:offset+8])
}

// bytesToLong gets a Long integer from a byte array.
func bytesToLong(log log.T, input []byte) (int64, error) {
	var res int64

	inputLength := len(input)
	if inputLength != 8 { //nolint:mnd
		log.Error("bytesToLong failed: input array size is not equal to 8.")

		return 0, ErrOffsetOutside
	}

	buf := bytes.NewBuffer(input)
	if err := binary.Read(buf, binary.BigEndian, &res); err != nil {
		return 0, fmt.Errorf("reading long from bytes: %w", err)
	}

	return res, nil
}

// GetUUID gets the 128bit uuid from an array of bytes starting from the offset.
func GetUUID(log log.T, byteArray []byte, offset int) (uuid.UUID, error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+16-1 > byteArrayLength-1 || offset < 0 {
		log.Error("getUUID failed: Offset is invalid.")

		return uuid.Nil, ErrOffsetOutsideByteArray
	}

	leastSignificantLong, err := GetLong(log, byteArray, offset)
	if err != nil {
		log.Error("getUUID failed: getting uuid LSBs Long value")

		return uuid.Nil, ErrOffsetOutside
	}

	leastSignificantBytes, err := LongToBytes(log, leastSignificantLong)
	if err != nil {
		log.Error("getUUID failed: getting uuid LSBs bytes value")

		return uuid.Nil, ErrOffsetOutside
	}

	mostSignificantLong, err := GetLong(log, byteArray, offset+8) //nolint:mnd
	if err != nil {
		log.Error("getUUID failed: getting uuid MSBs Long value")

		return uuid.Nil, ErrOffsetOutside
	}

	mostSignificantBytes, err := LongToBytes(log, mostSignificantLong)
	if err != nil {
		log.Error("getUUID failed: getting uuid MSBs bytes value")

		return uuid.Nil, ErrOffsetOutside
	}

	mostSignificantBytes = append(mostSignificantBytes, leastSignificantBytes...)

	result, err := uuid.FromBytes(mostSignificantBytes)
	if err != nil {
		return uuid.Nil, fmt.Errorf("creating UUID from bytes: %w", err)
	}

	return result, nil
}

// LongToBytes gets bytes array from a long integer.
func LongToBytes(log log.T, input int64) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, input); err != nil {
		return nil, fmt.Errorf("writing long to bytes: %w", err)
	}

	if buf.Len() != 8 { //nolint:mnd
		log.Error("LongToBytes failed: buffer output length is not equal to 8.")

		return make([]byte, 8), ErrOffsetOutside //nolint:mnd
	}

	return buf.Bytes(), nil
}

// GetBytes gets an array of bytes starting from the offset.
func GetBytes(log log.T, byteArray []byte, offset int, byteLength int) ([]byte, error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+byteLength-1 > byteArrayLength-1 || offset < 0 {
		log.Error("GetBytes failed: Offset is invalid.")

		return make([]byte, byteLength), ErrOffsetOutsideByteArray
	}

	return byteArray[offset : offset+byteLength], nil
}
