package podutils

import (
	"io"
	"io/fs"
)

// --------------------------------------------------------------------------
// Mocks for filesystem / files
type mockedFileSystem struct {
	osFS // embed so we only need to override what is used

	err error
	f   file
}

type mockedFile struct {
	file

	ret string
	err error
}

func (mfs mockedFileSystem) OpenFile(name string, _ int, _ fs.FileMode) (file, error) {
	if mfs.err != nil {
		return nil, mfs.err
	} else {
		return mfs.f, nil
	}
}

func (mfs mockedFileSystem) Open(_ string) (file, error) {
	if mfs.err != nil {
		return nil, mfs.err
	} else {
		return mfs.f, nil
	}
}

func (mf mockedFile) Read(b []byte) (int, error) {
	if mf.err != nil {
		return 0, mf.err
	} else {
		copy(b, []byte(mf.ret))
		return len(mf.ret), io.EOF
	}
}

func (mf mockedFile) Close() error { return nil }
