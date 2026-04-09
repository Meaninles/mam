package assets

type credentialCipher interface {
	Decrypt(ciphertext string) (string, error)
}
