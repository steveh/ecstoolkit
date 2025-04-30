package datachannel

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/steveh/ecstoolkit/encryption"
	"github.com/steveh/ecstoolkit/log"
)

// EncryptorBuilder is a function that builds an encryption object.
type EncryptorBuilder interface {
	Build(ctx context.Context, kmsKeyID string, sessionID string, targetID string) (encryption.IEncrypter, error)
}

// KMSEncryptorBuilder builds encryption objects using AWS KMS.
type KMSEncryptorBuilder struct {
	logger    log.T
	kmsClient *kms.Client
}

// NewKMSEncryptorBuilder creates a new KMSEncryptorBuilder with the provided KMS client and logger.
func NewKMSEncryptorBuilder(kmsClient *kms.Client, logger log.T) (*KMSEncryptorBuilder, error) {
	return &KMSEncryptorBuilder{
		logger:    logger,
		kmsClient: kmsClient,
	}, nil
}

// Build creates a new encryption object using the provided KMS key ID, session ID, and target ID.
//
//nolint:ireturn
func (b *KMSEncryptorBuilder) Build(ctx context.Context, kmsKeyID string, sessionID string, targetID string) (encryption.IEncrypter, error) {
	encryptionContext := map[string]string{
		"aws:ssm:SessionId": sessionID,
		"aws:ssm:TargetId":  targetID,
	}

	encryptor, err := encryption.NewEncrypter(ctx, b.logger, kmsKeyID, encryptionContext, b.kmsClient)
	if err != nil {
		return nil, fmt.Errorf("creating encryptor: %w", err)
	}

	return encryptor, nil
}
