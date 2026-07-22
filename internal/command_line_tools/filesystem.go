package clitools

import "os"

type FileSystem interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
}

type OSFileSystem struct{}

func (OSFileSystem) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
func (OSFileSystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}
func (OSFileSystem) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
