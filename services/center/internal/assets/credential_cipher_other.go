//go:build !windows

package assets

type passthroughCredentialCipher struct{}

func newSystemCredentialCipher() credentialCipher {
	return passthroughCredentialCipher{}
}

func (passthroughCredentialCipher) Decrypt(ciphertext string) (string, error) {
	return ciphertext, nil
}
