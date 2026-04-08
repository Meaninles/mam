package identity

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestLoadOrCreateGeneratesAndPersistsIdentity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-id.txt")

	id, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("load or create identity: %v", err)
	}
	if id == "" {
		t.Fatal("expected generated agent id")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read identity file: %v", err)
	}
	if string(content) == "" {
		t.Fatal("expected persisted agent id")
	}
}

func TestLoadOrCreateKeepsStableIdentity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-id.txt")

	first, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}

	second, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}

	if first != second {
		t.Fatalf("expected stable identity, got %s and %s", first, second)
	}
}

func TestLoadOrCreateRejectsBrokenIdentityFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-id.txt")
	if err := os.WriteFile(path, []byte(" \n "), 0o644); err != nil {
		t.Fatalf("write broken identity file: %v", err)
	}

	if _, err := LoadOrCreate(path); err == nil {
		t.Fatal("expected broken identity file error")
	}
}

func TestLoadOrCreateSupportsConcurrentReaders(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-id.txt")

	const readers = 8
	results := make([]string, readers)
	errs := make([]error, readers)

	var wg sync.WaitGroup
	for index := 0; index < readers; index++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = LoadOrCreate(path)
		}(index)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			t.Fatalf("concurrent load failed: %v", err)
		}
	}

	for _, result := range results {
		if result != results[0] {
			t.Fatalf("expected stable concurrent identity, got %v", results)
		}
	}
}
