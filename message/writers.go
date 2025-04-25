package message

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/google/uuid"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/util"
)

// putUInteger puts a uint32 into a byte array starting from the specified offset.
func putUInteger(log log.T, byteArray []byte, offset int, value uint32) error {
	safe, err := util.SafeInt32(value)
	if err != nil {
		return fmt.Errorf("getting int32: %w", err)
	}

	return PutInteger(log, byteArray, offset, safe)
}

// PutInteger puts an int32 into a byte array starting from the specified offset.
func PutInteger(log log.T, byteArray []byte, offset int, value int32) error {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+4 > byteArrayLength || offset < 0 {
		log.Error("PutInteger failed: Offset is invalid.")

		return ErrOffsetOutside
	}

	bytes, err := IntegerToBytes(log, value)
	if err != nil {
		log.Error("PutInteger failed: IntegerToBytes Failed.")

		return err
	}

	copy(byteArray[offset:offset+4], bytes)

	return nil
}

// IntegerToBytes gets bytes array from an integer.
func IntegerToBytes(log log.T, input int32) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, input); err != nil {
		return nil, fmt.Errorf("writing integer to bytes: %w", err)
	}

	if buf.Len() != 4 { //nolint:mnd
		log.Error("IntegerToBytes failed: buffer output length is not equal to 4.")

		return make([]byte, 4), ErrOffsetOutside //nolint:mnd
	}

	return buf.Bytes(), nil
}

// PutString puts a string value to a byte array starting from the specified offset.
func PutString(log log.T, byteArray []byte, offsetStart int, offsetEnd int, inputString string) error {
	byteArrayLength := len(byteArray)
	if offsetStart > byteArrayLength-1 || offsetEnd > byteArrayLength-1 || offsetStart > offsetEnd || offsetStart < 0 {
		log.Error("putString failed: Offset is invalid.")

		return ErrOffsetOutside
	}

	if offsetEnd-offsetStart+1 < len(inputString) {
		log.Error("PutString failed: Not enough space to save the string.")

		return ErrNotEnoughSpace
	}

	// wipe out the array location first and then insert the new value.
	for i := offsetStart; i <= offsetEnd; i++ {
		byteArray[i] = ' '
	}

	copy(byteArray[offsetStart:offsetEnd+1], inputString)

	return nil
}

// PutBytes puts bytes into the array at the correct offset.
func PutBytes(log log.T, byteArray []byte, offsetStart int, offsetEnd int, inputBytes []byte) error {
	byteArrayLength := len(byteArray)
	if offsetStart > byteArrayLength-1 || offsetEnd > byteArrayLength-1 || offsetStart > offsetEnd || offsetStart < 0 {
		log.Error("PutBytes failed: Offset is invalid.")

		return ErrOffsetOutsideByteArray
	}

	if offsetEnd-offsetStart+1 != len(inputBytes) {
		log.Error("PutBytes failed: Not enough space to save the bytes.")

		return ErrNotEnoughSpace
	}

	copy(byteArray[offsetStart:offsetEnd+1], inputBytes)

	return nil
}

// PutUUID puts the 128 bit uuid to an array of bytes starting from the offset.
func PutUUID(log log.T, byteArray []byte, offset int, input uuid.UUID) error {
	if input == uuid.Nil {
		log.Error("PutUUID failed: input is null.")

		return ErrOffsetOutside
	}

	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+16-1 > byteArrayLength-1 || offset < 0 {
		log.Error("PutUUID failed: Offset is invalid.")

		return ErrOffsetOutsideByteArray
	}

	uuidBytes, err := input.MarshalBinary()
	if err != nil {
		log.Error("PutUUID failed: marshaling UUID to bytes")

		return fmt.Errorf("marshaling UUID to bytes: %w", err)
	}

	leastSignificantLong, err := bytesToLong(log, uuidBytes[8:16])
	if err != nil {
		log.Error("PutUUID failed: getting leastSignificant Long value")

		return ErrOffsetOutside
	}

	mostSignificantLong, err := bytesToLong(log, uuidBytes[0:8])
	if err != nil {
		log.Error("PutUUID failed: getting mostSignificantLong Long value")

		return ErrOffsetOutside
	}

	err = PutLong(log, byteArray, offset, leastSignificantLong)
	if err != nil {
		log.Error("PutUUID failed: putting leastSignificantLong Long value")

		return ErrOffsetOutside
	}

	err = PutLong(log, byteArray, offset+8, mostSignificantLong) //nolint:mnd
	if err != nil {
		log.Error("PutUUID failed: putting mostSignificantLong Long value")

		return ErrOffsetOutside
	}

	return nil
}

// PutLong puts a long integer value to a byte array starting from the specified offset.
func PutLong(log log.T, byteArray []byte, offset int, value int64) error {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+8 > byteArrayLength || offset < 0 {
		log.Error("PutLong failed: Offset is invalid.")

		return ErrOffsetOutside
	}

	mbytes, err := LongToBytes(log, value)
	if err != nil {
		log.Error("PutLong failed: LongToBytes Failed.")

		return err
	}

	copy(byteArray[offset:offset+8], mbytes)

	return nil
}

// putULong puts an unsigned long integer.
func putULong(log log.T, byteArray []byte, offset int, value uint64) error {
	safe, err := util.SafeInt64(value)
	if err != nil {
		return fmt.Errorf("getting int64: %w", err)
	}

	return PutLong(log, byteArray, offset, safe)
}
