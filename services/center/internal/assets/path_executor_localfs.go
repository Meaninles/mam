package assets

import (
	"io"
	"os"
	"time"
)

func ensureLocalDirectory(path string) error {
	return os.MkdirAll(path, 0o755)
}

func writeLocalFile(path string, content []byte) error {
	parent := filepathDir(path)
	if parent != "" && parent != "." {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, content, 0o644)
}

func writeLocalStream(path string, reader io.Reader) error {
	parent := filepathDir(path)
	if parent != "" && parent != "." {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return err
		}
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, reader)
	return err
}

func deleteLocalFile(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func streamLocalFile(path string, consume func(reader io.Reader) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return consume(file)
}

func statLocalFile(path string) (fileMetadata, error) {
	info, err := os.Stat(path)
	if err != nil {
		return fileMetadata{}, err
	}
	return fileMetadata{
		SizeBytes:  info.Size(),
		ModifiedAt: info.ModTime().UTC(),
	}, nil
}

func setLocalFileModifiedTime(path string, modifiedAt time.Time) error {
	return os.Chtimes(path, modifiedAt, modifiedAt)
}

func deleteLocalDirectory(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func filepathDir(path string) string {
	for index := len(path) - 1; index >= 0; index-- {
		if path[index] == '\\' || path[index] == '/' {
			return path[:index]
		}
	}
	return ""
}
