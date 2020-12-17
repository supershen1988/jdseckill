package utils

import (
	"fmt"
	"os"
)

func fileExists(name string) bool {
	_, err := os.Stat(name)
	return !os.IsNotExist(err)
}

func mkdir(dirPath string, dirMode os.FileMode) error {
	err := os.MkdirAll(dirPath, dirMode)
	if err != nil {
		return fmt.Errorf("%s: making directory: %v", dirPath, err)
	}
	return nil
}

func DeleteDirectory(path string) bool {
	err := os.RemoveAll(path)
	if err != nil {
		return false
	} else {
		return true
	}
}

func DeleteFile(path string) bool{
	err := os.Remove(path)
	if err != nil {
		return false
	} else {
		return true
	}
}
