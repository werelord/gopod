package podutils

import (
	"io"
	"io/fs"
	"os"
)

// mocking interfaces and default implementations to os and related package calls
type osFileSystem interface {
	OpenFile(string, int, fs.FileMode) (file, error)
	Open(string) (file, error)
	ReadDir(string) ([]fs.DirEntry, error)
	Remove(string) error
	Symlink(string, string) error
	Stat(string) (fs.FileInfo, error)
	MkdirAll(string, fs.FileMode) error
}

type file interface {
	io.ReadWriteCloser
	Stat() (os.FileInfo, error)
}

// osFS implements fileSystem using the local disk
type osFS struct{}

func (osFS) OpenFile(name string, flag int, perm fs.FileMode) (file, error) {
	return os.OpenFile(name, flag, perm)
}
func (osFS) Open(name string) (file, error) {
	return os.Open(name)
}
func (osFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(name)
}
func (osFS) Remove(name string) error {
	return os.Remove(name)
}
func (osFS) Symlink(oldname, newname string) error {
	return os.Symlink(oldname, newname)
}
func (osFS) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}
func (osFS) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}

var osfs osFileSystem = osFS{}


