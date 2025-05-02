package encryption

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/kms"
)

// KMSKeySizeInBytes is the key size that is fetched from KMS. 64 bytes key is split into two halves.
// First half 32 bytes key is used by agent for encryption and second half 32 bytes by clients like cli/console.
const KMSKeySizeInBytes int32 = 64

// KMSGenerateDataKey gets cipher text and plain text keys from KMS service.
// It returns the encrypted data key and the plaintext data key.
func KMSGenerateDataKey(ctx context.Context, kmsKeyID string, svc *kms.Client, encryptionContext Context) ([]byte, []byte, error) {
	kmsKeySize := KMSKeySizeInBytes

	generateDataKeyInput := kms.GenerateDataKeyInput{
		KeyId:             &kmsKeyID,
		NumberOfBytes:     &kmsKeySize,
		EncryptionContext: encryptionContext,
	}

	generateDataKeyOutput, err := svc.GenerateDataKey(ctx, &generateDataKeyInput)
	if err != nil {
		return nil, nil, fmt.Errorf("calling KMS GenerateDataKey API: %w", err)
	}

	return generateDataKeyOutput.CiphertextBlob, generateDataKeyOutput.Plaintext, nil
}
