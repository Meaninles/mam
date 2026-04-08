package identity

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var fileLocks sync.Map

func LoadOrCreate(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("identity file path is required")
	}

	lock := fileLock(path)
	lock.Lock()
	defer lock.Unlock()

	if content, err := os.ReadFile(path); err == nil {
		id := strings.TrimSpace(string(content))
		if id == "" {
			return "", fmt.Errorf("identity file is empty")
		}
		return id, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read identity file: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create identity directory: %w", err)
	}

	id, err := generateID()
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(path, []byte(id), 0o644); err != nil {
		return "", fmt.Errorf("write identity file: %w", err)
	}

	return id, nil
}

func generateID() (string, error) {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate agent id: %w", err)
	}
	return "agent-" + hex.EncodeToString(buffer), nil
}

func fileLock(path string) *sync.Mutex {
	lock, _ := fileLocks.LoadOrStore(path, &sync.Mutex{})
	return lock.(*sync.Mutex)
}
