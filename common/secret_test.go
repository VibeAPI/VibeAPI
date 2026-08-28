package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptedSecretDoesNotPersistPlaintextAndDetectsKeyChanges(t *testing.T) {
	previous := CryptoSecret
	CryptoSecret = "prompt-audit-test-secret"
	t.Cleanup(func() { CryptoSecret = previous })

	encrypted, err := EncryptSecret("sk-sensitive-value")
	require.NoError(t, err)
	assert.NotContains(t, encrypted, "sk-sensitive-value")

	plaintext, err := DecryptSecret(encrypted)
	require.NoError(t, err)
	assert.Equal(t, "sk-sensitive-value", plaintext)

	CryptoSecret = "changed-secret"
	_, err = DecryptSecret(encrypted)
	assert.Error(t, err)
}
