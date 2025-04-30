// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

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
