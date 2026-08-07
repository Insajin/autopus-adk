package cli

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompanionOMPContextPromotionKeyID_RejectsUntrustedStdinKeyWithoutSecretFlags(t *testing.T) {
	seed := sha256.Sum256([]byte("promotion-key-id-cli-test"))
	privateKey := ed25519.NewKeyFromSeed(seed[:])
	encoded := base64.StdEncoding.EncodeToString(privateKey)

	command := newCompanionOMPContextPromotionKeyIDCmd()
	command.SetIn(strings.NewReader(encoded))
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	require.Error(t, command.Execute())
	assert.NotContains(t, output.String(), encoded)
	for _, forbidden := range []string{"key", "key-file", "private-key", "signing-key"} {
		assert.Nil(t, command.Flags().Lookup(forbidden))
	}
}
