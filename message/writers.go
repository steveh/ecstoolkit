package message

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/google/uuid"
	"github.com/steveh/ecstoolkit/util"
)

// PutUInteger puts a uint32 into a byte array starting from the specified offset.
func PutUInteger(byteArray []byte, offset int, value uint32) error {
	safe, err := util.SafeInt32(value)
	if err != nil {
		return fmt.Errorf("getting int32: %w", err)
	}

	return PutInteger(byteArray, offset, safe)
}

// PutInteger puts an int32 into a byte array starting from the specified offset.
func PutInteger(byteArray []byte, offset int, value int32) error {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+integerLength > byteArrayLength || offset < 0 {
		return fmt.Errorf("%w: offset %d, integerLength %d, byteArrayLength %d", ErrOffsetOutside, offset, integerLength, byteArrayLength)
	}

	bytes, err := IntegerToBytes(value)
	if err != nil {
		return fmt.Errorf("converting to bytes: %w", err)
	}

	copy(byteArray[offset:offset+integerLength], bytes)

	return nil
}

// IntegerToBytes gets bytes array from an integer.
func IntegerToBytes(input int32) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, input); err != nil {
		return nil, fmt.Errorf("writing integer to bytes: %w", err)
	}

	if buf.Len() != integerLength {
		return make([]byte, integerLength), fmt.Errorf("%w: buffer output length %d, integerLength %d", ErrOffsetOutside, buf.Len(), integerLength)
	}

	return buf.Bytes(), nil
}

// PutString puts a string value to a byte array starting from the specified offset.
func PutString(byteArray []byte, offsetStart int, offsetEnd int, inputString string) error {
	byteArrayLength := len(byteArray)
	if offsetStart > byteArrayLength-1 || offsetEnd > byteArrayLength-1 || offsetStart > offsetEnd || offsetStart < 0 {
		return fmt.Errorf("%w: offsetStart %d, offsetEnd %d, byteArrayLength %d", ErrOffsetOutside, offsetStart, offsetEnd, byteArrayLength)
	}

	if offsetEnd-offsetStart+1 < len(inputString) {
		return fmt.Errorf("%w: offsetStart %d, offsetEnd %d, stringLength %d", ErrNotEnoughSpace, offsetStart, offsetEnd, len(inputString))
	}

	// wipe out the array location first and then insert the new value.
	for i := offsetStart; i <= offsetEnd; i++ {
		byteArray[i] = ' '
	}

	copy(byteArray[offsetStart:offsetEnd+1], inputString)

	return nil
}

// PutBytes puts bytes into the array at the correct offset.
func PutBytes(byteArray []byte, offsetStart int, offsetEnd int, inputBytes []byte) error {
	byteArrayLength := len(byteArray)
	if offsetStart > byteArrayLength-1 || offsetEnd > byteArrayLength-1 || offsetStart > offsetEnd || offsetStart < 0 {
		return fmt.Errorf("%w: offsetStart %d, offsetEnd %d, byteArrayLength %d", ErrOffsetOutside, offsetStart, offsetEnd, byteArrayLength)
	}

	if offsetEnd-offsetStart+1 != len(inputBytes) {
		return fmt.Errorf("%w: offsetStart %d, offsetEnd %d, bytesLength %d", ErrNotEnoughSpace, offsetStart, offsetEnd, len(inputBytes))
	}

	copy(byteArray[offsetStart:offsetEnd+1], inputBytes)

	return nil
}

// PutUUID puts the 128 bit uuid to an array of bytes starting from the offset.
func PutUUID(byteArray []byte, offset int, input uuid.UUID) error {
	if input == uuid.Nil {
		return ErrNil
	}

	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+uuidLength-1 > byteArrayLength-1 || offset < 0 {
		return fmt.Errorf("%w: offset %d, uuidLength %d, byteArrayLength %d", ErrOffsetOutside, offset, uuidLength, byteArrayLength)
	}

	uuidBytes, err := input.MarshalBinary()
	if err != nil {
		return fmt.Errorf("marshaling UUID to bytes: %w", err)
	}

	leastSignificantLong, err := bytesToLong(uuidBytes[longLength:(longLength + longLength)])
	if err != nil {
		return fmt.Errorf("%w: getting LSB long value", err)
	}

	mostSignificantLong, err := bytesToLong(uuidBytes[0:longLength])
	if err != nil {
		return fmt.Errorf("%w: getting MSB long value", err)
	}

	err = PutLong(byteArray, offset, leastSignificantLong)
	if err != nil {
		return fmt.Errorf("%w: putting LSB long value", err)
	}

	err = PutLong(byteArray, offset+longLength, mostSignificantLong)
	if err != nil {
		return fmt.Errorf("%w: putting MSB long value", err)
	}

	return nil
}

// PutLong puts a long integer value to a byte array starting from the specified offset.
func PutLong(byteArray []byte, offset int, value int64) error {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+longLength > byteArrayLength || offset < 0 {
		return fmt.Errorf("%w: offset %d, longLength %d, byteArrayLength %d", ErrOffsetOutside, offset, longLength, byteArrayLength)
	}

	mbytes, err := LongToBytes(value)
	if err != nil {
		return fmt.Errorf("converting to bytes: %w", err)
	}

	copy(byteArray[offset:offset+longLength], mbytes)

	return nil
}

// PutULong puts an unsigned long integer.
func PutULong(byteArray []byte, offset int, value uint64) error {
	safe, err := util.SafeInt64(value)
	if err != nil {
		return fmt.Errorf("getting int64: %w", err)
	}

	return PutLong(byteArray, offset, safe)
}
