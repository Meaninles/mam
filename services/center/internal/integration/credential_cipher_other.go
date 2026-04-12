//go:build !windows

package integration

import "fmt"

type unsupportedCredentialCipher struct{}

func newSystemCredentialCipher() credentialCipher {
	return unsupportedCredentialCipher{}
}

func (unsupportedCredentialCipher) Encrypt(string) (string, error) {
	return "", fmt.Errorf("credential encryption is only implemented on Windows")
}

func (unsupportedCredentialCipher) Decrypt(string) (string, error) {
	return "", fmt.Errorf("credential decryption is only implemented on Windows")
}
