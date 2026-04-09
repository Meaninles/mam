//go:build windows

package assets

import (
	"encoding/base64"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

type dpapiCredentialCipher struct{}

func newSystemCredentialCipher() credentialCipher {
	return dpapiCredentialCipher{}
}

func (dpapiCredentialCipher) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode credential: %w", err)
	}

	input := bytesToDataBlob(raw)
	var description *uint16
	var output windows.DataBlob
	if err := windows.CryptUnprotectData(input, &description, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return "", fmt.Errorf("decrypt credential: %w", err)
	}
	defer freeDataBlob(&output)

	return string(dataBlobBytes(&output)), nil
}

func bytesToDataBlob(data []byte) *windows.DataBlob {
	if len(data) == 0 {
		return &windows.DataBlob{}
	}

	return &windows.DataBlob{
		Size: uint32(len(data)),
		Data: &data[0],
	}
}

func dataBlobBytes(blob *windows.DataBlob) []byte {
	if blob == nil || blob.Data == nil || blob.Size == 0 {
		return nil
	}

	return append([]byte(nil), unsafe.Slice(blob.Data, int(blob.Size))...)
}

func freeDataBlob(blob *windows.DataBlob) {
	if blob == nil || blob.Data == nil {
		return
	}

	_, _ = windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(blob.Data))))
}
