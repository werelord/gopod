package podutils

import (
	"errors"
	"fmt"
	"gopod/testutils"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// --------------------------------------------------------------------------
// Mocks for filesystem / files
type mockedFileSystem struct {
	osImpl // embed so we only need to override what is used

	err     error
	f       fileInterface
	dirlist []fs.DirEntry

	removeList []string
}

type mockedFile struct {
	ret string
	err error
}

type mockedDirEntry struct {
	name  string
	isDir bool
	info  mockedFileInfo
}

type mockedFileInfo struct {
	name string
	size int64
	err  error
}

func (mfs mockedFileSystem) OpenFile(name string, _ int, _ fs.FileMode) (fileInterface, error) {
	if mfs.err != nil {
		return nil, mfs.err
	} else {
		return mfs.f, nil
	}
}
func (mfs mockedFileSystem) Open(_ string) (fileInterface, error) {
	if mfs.err != nil {
		return nil, mfs.err
	} else {
		return mfs.f, nil
	}
}
func (mfs mockedFileSystem) ReadDir(name string) ([]fs.DirEntry, error) {
	if mfs.err != nil {
		return nil, mfs.err
	} else {
		// sorted list returned
		sort.Slice(mfs.dirlist, func(i, j int) bool { return mfs.dirlist[i].Name() < mfs.dirlist[j].Name() })
		return mfs.dirlist, nil
	}
}
func (mfs *mockedFileSystem) Remove(name string) error {
	// since errors are just logged, just collect the removeal list
	mfs.removeList = append(mfs.removeList, filepath.Base(name))
	return nil
}

func (mf mockedFile) Read(b []byte) (int, error) {
	if mf.err != nil {
		return 0, mf.err
	} else {
		copy(b, []byte(mf.ret))
		return len(mf.ret), io.EOF
	}
}
func (mf mockedFile) Write(p []byte) (int, error) {
	if mf.err != nil {
		return 0, mf.err
	} else {
		return len(p), nil
	}
}
func (mf mockedFile) Close() error { return nil }

func (mde mockedDirEntry) Name() string               { return mde.name }
func (mde mockedDirEntry) Info() (fs.FileInfo, error) { return mde.info, mde.info.err }
func (mde mockedDirEntry) Type() fs.FileMode          { return 0000 }
func (mde mockedDirEntry) IsDir() bool                { return mde.isDir }

func (mfi mockedFileInfo) Name() string       { return mfi.name }
func (mfi mockedFileInfo) Size() int64        { return mfi.size }
func (mfi mockedFileInfo) Mode() fs.FileMode  { return 0000 }
func (mfi mockedFileInfo) ModTime() time.Time { return time.Now() }
func (mfi mockedFileInfo) IsDir() bool        { return false }
func (mfi mockedFileInfo) Sys() any           { return nil }

func makeMockDirEntry(name string, err error, isDir bool, size int64) fs.DirEntry {
	return mockedDirEntry{
		name:  name,
		isDir: isDir,
		info: mockedFileInfo{
			name: name,
			size: size,
			err:  err,
		},
	}
}

// end of mocks
//--------------------------------------------------------------------------

func TestSaveToFile(t *testing.T) {
	type args struct {
		buf      []byte
		filename string
	}
	var argimp = args{buf: []byte("armleg"), filename: "barfoo"}
	tests := []struct {
		name     string
		args     args
		mockedfs *mockedFileSystem
		wantErr  bool
	}{
		{
			"open error",
			argimp,
			&mockedFileSystem{err: errors.New("foobar"), f: &mockedFile{}},
			true,
			// TODO: Add test cases.
		}, {
			"write error",
			argimp,
			&mockedFileSystem{f: &mockedFile{err: errors.New("foobar")}},
			true,
		}, {
			"success",
			argimp,
			&mockedFileSystem{f: &mockedFile{}},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldFS := osimpl
			//mockedfs := tt.mockedfs
			osimpl = tt.mockedfs
			// restore after the tests
			defer func() {
				osimpl = oldFS
			}()

			if err := SaveToFile(tt.args.buf, tt.args.filename); (err != nil) != tt.wantErr {
				t.Errorf("SaveToFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// --------------------------------------------------------------------------
func TestLoadFile(t *testing.T) {
	type args struct {
		filename string
	}

	tests := []struct {
		name     string
		args     args
		mockedfs *mockedFileSystem
		expect   string
		wantErr  bool
	}{
		{
			"open error",
			args{filename: "open error"},
			&mockedFileSystem{err: os.ErrNotExist},
			"",
			true,
		},
		{
			"file error",
			args{filename: "file error"},
			&mockedFileSystem{err: nil, f: &mockedFile{ret: "", err: errors.New("foobar")}},
			"",
			true,
		},
		{
			"success",
			args{filename: "succes"},
			&mockedFileSystem{f: &mockedFile{ret: "foobar"}},
			"foobar",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldFS := osimpl
			//mockedfs := tt.mockedfs
			osimpl = tt.mockedfs
			// restore after the tests
			defer func() {
				osimpl = oldFS
			}()

			res, err := LoadFile(tt.args.filename)

			testutils.AssertEquals(t, tt.expect, string(res))
			testutils.AssertErr(t, tt.wantErr, err)
		})
	}
}

func TestCleanFilename(t *testing.T) {
	// not testing this, as its just a shortcut to filenamify.. fuck that

	// type args struct {
	// 	filename string
	// }
	// tests := []struct {
	// 	name string
	// 	args args
	// 	want string
	// }{
	// 	// TODO: Add test cases.
	// }
	// for _, tt := range tests {
	// 	t.Run(tt.name, func(t *testing.T) {
	// 		if got := CleanFilename(tt.args.filename); got != tt.want {
	// 			t.Errorf("CleanFilename() = %v, want %v", got, tt.want)
	// 		}
	// 	})
	// }
}

func TestRotateFiles(t *testing.T) {

	var filenames = []string{
		"test.all.20220801_185226.log",
		"test.all.20220802_185226.log",
		"test.all.20220803_185226.log",
		"test.all.20220804_185226.log",
		"test.all.20220805_185226.log",
		"test.all.20220806_185226.log",
		"test.all.20220807_185226.log",
		"test.all.20220808_185226.log",
	}
	var defaultDirList = make([]fs.DirEntry, 0, len(filenames))

	for _, f := range filenames {
		defaultDirList = append(defaultDirList, makeMockDirEntry(f, nil, false, 1024))
	}

	// will insert into middle of the list
	var infoErrEntry = makeMockDirEntry("test.all.20220803_150000_infoerr.log", errors.New("foobar"), false, 1024)
	var dirEntry = makeMockDirEntry("test.all.20220803_150000_dir.log", nil, true, 1024)
	var sizeZeroEntry = makeMockDirEntry("test.all.20220803_150000_zerosize.log", nil, false, 0)

	type args struct {
		path      string
		pattern   string
		numToKeep uint
	}

	tests := []struct {
		name             string
		args             args
		mockedfs         *mockedFileSystem
		expectedRemovals []string
		wantErr          bool
	}{
		{
			"blank path",
			args{numToKeep: 2},
			&mockedFileSystem{dirlist: defaultDirList},
			nil,
			true,
		},
		{
			"empty pattern",
			args{path: "foo", numToKeep: 2},
			&mockedFileSystem{dirlist: defaultDirList},
			nil,
			true,
		},
		{
			"readdir error",
			args{path: "foo", pattern: "test.all.*.log", numToKeep: 2},
			&mockedFileSystem{err: errors.New("foobar"), dirlist: defaultDirList},
			nil,
			true,
		},
		{
			"bad pattern",
			args{path: "foo", pattern: "test.[].log", numToKeep: 2},
			&mockedFileSystem{dirlist: defaultDirList},
			nil,
			true,
		},
		{
			"numToKeep == 0 (still keep 1)",
			args{path: "foo", pattern: "test.all.*.log", numToKeep: 0},
			&mockedFileSystem{dirlist: CopyAndAppend(defaultDirList, infoErrEntry)},
			filenames[:7],
			false,
		},
		{
			"info() returns error",
			args{path: "foo", pattern: "test.all.*.log", numToKeep: 2},
			&mockedFileSystem{dirlist: CopyAndAppend(defaultDirList, infoErrEntry)},
			filenames[:6],
			false,
		},
		{
			"dir matches pattern",
			args{path: "foo", pattern: "test.all.*.log", numToKeep: 2},
			&mockedFileSystem{dirlist: CopyAndAppend(defaultDirList, dirEntry)},
			filenames[:6],
			false,
		},
		{
			"zero size entry",
			args{path: "foo", pattern: "test.all.*.log", numToKeep: 2},
			&mockedFileSystem{dirlist: CopyAndAppend(defaultDirList, sizeZeroEntry)},
			CopyAndAppend(filenames[:6], sizeZeroEntry.Name()),
			false,
		},
		{
			"no removals (numkeep > len(list)",
			args{path: "foo", pattern: "test.all.*.log", numToKeep: 10},
			&mockedFileSystem{dirlist: defaultDirList},
			nil,
			false,
		},
		{
			"normal success",
			args{path: "foo", pattern: "test.all.*.log", numToKeep: 2},
			&mockedFileSystem{dirlist: defaultDirList},
			filenames[:len(filenames)-2],
			false,
		},

		// {
		// 	"blank path"
		// },
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			oldFS := osimpl
			//mockedfs := tt.mockedfs
			osimpl = tt.mockedfs
			// restore after the tests
			defer func() {
				osimpl = oldFS
			}()

			err := RotateFiles(tt.args.path, tt.args.pattern, tt.args.numToKeep)
			testutils.AssertErr(t, tt.wantErr, err)

			var exp = len(tt.expectedRemovals)
			var got = len(tt.mockedfs.removeList)
			testutils.Assert(t, exp == got,
				fmt.Sprintf("remove list lenght does not match; exp: %v got:%v", exp, got))

			// make sure these are sorted for comparisons
			sort.Strings(tt.expectedRemovals)
			sort.Strings(tt.mockedfs.removeList)

			testutils.AssertEquals(t, tt.expectedRemovals, tt.mockedfs.removeList)

		})
	}
}

func TestFindMostRecent(t *testing.T) {
	type args struct {
		path    string
		pattern string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindMostRecent(tt.args.path, tt.args.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindMostRecent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("FindMostRecent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateSymlink(t *testing.T) {
	type args struct {
		source  string
		symDest string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CreateSymlink(tt.args.source, tt.args.symDest); (err != nil) != tt.wantErr {
				t.Errorf("CreateSymlink() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFileExists(t *testing.T) {
	type args struct {
		filename string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FileExists(tt.args.filename); got != tt.want {
				t.Errorf("FileExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMkDirAll(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := MkDirAll(tt.args.path); (err != nil) != tt.wantErr {
				t.Errorf("MkDirAll() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
