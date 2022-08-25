package podutils

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// mocking interfaces and default implementations to os and related package calls
type osInterface interface {
	OpenFile(string, int, fs.FileMode) (fileInterface, error)
	Open(string) (fileInterface, error)
	ReadDir(string) ([]fs.DirEntry, error)
	Remove(string) error
	Symlink(string, string) error
	Stat(string) (fs.FileInfo, error)
	MkdirAll(string, fs.FileMode) error
}

type fileInterface interface {
	io.ReadWriteCloser
	//Stat() (os.FileInfo, error)
}

type filepathInterface interface {
	WalkDir(root string, fn fs.WalkDirFunc) error
}

type osImpl struct{}
type filepathImpl struct{}

func (osImpl) OpenFile(name string, flag int, perm fs.FileMode) (fileInterface, error) {
	return os.OpenFile(name, flag, perm)
}
func (osImpl) Open(name string) (fileInterface, error) {
	return os.Open(name)
}
func (osImpl) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(name)
}
func (osImpl) Remove(name string) error {
	return os.Remove(name)
}
func (osImpl) Symlink(oldname, newname string) error {
	return os.Symlink(oldname, newname)
}
func (osImpl) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}
func (osImpl) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (filepathImpl) WalkDir(root string, fn fs.WalkDirFunc) error {
	return filepath.WalkDir(root, fn)
}

var osimpl osInterface = osImpl{}
var filepathimpl filepathInterface = filepathImpl{}
