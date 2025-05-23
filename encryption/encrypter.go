// Package encryption provides encryption and decryption functionality using AWS KMS.
package encryption

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/steveh/ecstoolkit/log"
)

const (
	nonceSize = 12
)

// KMSKeyProvider defines the interface for AWS KMS key operations.
type KMSKeyProvider interface {
	GenerateDataKey()
}

// IEncrypter defines the interface for encryption and decryption operations.
type IEncrypter interface {
	Encrypt(plainText []byte) (cipherText []byte, err error)
	Decrypt(cipherText []byte) (plainText []byte, err error)
	GetEncryptedDataKey() (ciptherTextBlob []byte)
}

// Context is a map that holds the encryption context for KMS.
type Context map[string]string

// Encrypter implements the IEncrypter interface using AWS KMS for key management.
type Encrypter struct {
	KMSService *kms.Client
	logger     log.T

	kmsKeyID      string
	cipherTextKey []byte
	encryptionKey []byte
	decryptionKey []byte
}

// NewEncrypter creates a new Encrypter instance with the given KMS key and encryption context.
func NewEncrypter(ctx context.Context, logger log.T, kmsKeyID string, encryptionContext Context, kmsService *kms.Client) (*Encrypter, error) {
	e := Encrypter{
		kmsKeyID:   kmsKeyID,
		KMSService: kmsService,
		logger:     logger.With("subsystem", "Encrypter"),
	}

	err := e.generateEncryptionKey(ctx, kmsKeyID, encryptionContext)

	return &e, err
}

// GetEncryptedDataKey returns the cipherText that was pulled from KMS.
func (e *Encrypter) GetEncryptedDataKey() []byte {
	return e.cipherTextKey
}

// GetKMSKeyID gets the KMS key id that is used to generate the encryption key.
func (e *Encrypter) GetKMSKeyID() string {
	return e.kmsKeyID
}

// Encrypt encrypts a byte slice and returns the encrypted slice.
func (e *Encrypter) Encrypt(plainText []byte) ([]byte, error) {
	aesgcm, err := getAEAD(e.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("getAEAD: %w", err)
	}

	cipherText := make([]byte, nonceSize+len(plainText))

	nonce := make([]byte, nonceSize)
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("error when generating nonce for encryption, %w", err)
	}

	// Encrypt plain text using given key and newly generated nonce
	cipherTextWithoutNonce := aesgcm.Seal(nil, nonce, plainText, nil)

	// Append nonce to the beginning of the cipher text to be used while decrypting
	cipherText = append(cipherText[:nonceSize], nonce...)
	cipherText = append(cipherText[nonceSize:], cipherTextWithoutNonce...)

	return cipherText, nil
}

// Decrypt decrypts a byte slice and returns the decrypted slice.
func (e *Encrypter) Decrypt(cipherText []byte) ([]byte, error) {
	aesgcm, err := getAEAD(e.decryptionKey)
	if err != nil {
		return nil, fmt.Errorf("getAEAD: %w", err)
	}

	// Pull the nonce out of the cipherText
	nonce := cipherText[:nonceSize]
	cipherTextWithoutNonce := cipherText[nonceSize:]

	// Decrypt just the actual cipherText using nonce extracted above
	plainText, err := aesgcm.Open(nil, nonce, cipherTextWithoutNonce, nil)
	if err != nil {
		return nil, fmt.Errorf("error decrypting encrypted test, %w", err)
	}

	return plainText, nil
}

// generateEncryptionKey calls KMS to generate a new encryption key.
func (e *Encrypter) generateEncryptionKey(ctx context.Context, kmsKeyID string, encryptionContext Context) error {
	cipherTextKey, plainTextKey, err := KMSGenerateDataKey(ctx, kmsKeyID, e.KMSService, encryptionContext)
	if err != nil {
		return fmt.Errorf("generating data key from KMS: %w", err)
	}

	keySize := len(plainTextKey) / 2 //nolint:mnd
	e.decryptionKey = plainTextKey[:keySize]
	e.encryptionKey = plainTextKey[keySize:]
	e.cipherTextKey = cipherTextKey

	return nil
}

// getAEAD gets AEAD which is a GCM cipher mode providing authenticated encryption with associated data.
func getAEAD(plainTextKey []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(plainTextKey)
	if err != nil {
		return nil, fmt.Errorf("error creating NewCipher, %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("error creating NewGCM, %w", err)
	}

	return aesgcm, nil
}
