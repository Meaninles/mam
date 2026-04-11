package assets

import "os"

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

func filepathDir(path string) string {
	for index := len(path) - 1; index >= 0; index-- {
		if path[index] == '\\' || path[index] == '/' {
			return path[:index]
		}
	}
	return ""
}
