package assets

import "os"

func ensureLocalDirectory(path string) error {
	return os.MkdirAll(path, 0o755)
}

func deleteLocalFile(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func deleteLocalDirectory(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
