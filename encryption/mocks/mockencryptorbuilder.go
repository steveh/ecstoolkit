// Package mocks provides a mock implementation of the IEncryptorBuilder interface for testing purposes.
package mocks

import (
	context "context"

	"github.com/steveh/ecstoolkit/encryption"
)

// MockEncryptorBuilder is a mock implementation of the IEncryptorBuilder interface.
type MockEncryptorBuilder struct {
	encrypter encryption.IEncrypter
}

// NewMockEncryptorBuilder creates a new instance of MockEncryptorBuilder with the provided encrypter.
func NewMockEncryptorBuilder(encrypter encryption.IEncrypter) *MockEncryptorBuilder {
	return &MockEncryptorBuilder{
		encrypter: encrypter,
	}
}

// Build creates a new encryption.IEncrypter instance.
//
//nolint:ireturn
func (b *MockEncryptorBuilder) Build(_ context.Context, _ string, _ string, _ string) (encryption.IEncrypter, error) {
	return b.encrypter, nil
}
