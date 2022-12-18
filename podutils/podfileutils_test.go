package podutils

import (
	"errors"
	"fmt"
	"gopod/testutils"
	"io"
	"io/fs"
	"math/rand"
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

	err error
	// extra errors for single call
	removeErr  error
	symlinkErr error

	f        fileInterface
	dirlist  []fs.DirEntry
	fileInfo fs.FileInfo

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
	time time.Time
	err  error
}

// type mockedFilePath struct {
// 	filepathImpl // only define what we need

// 	walkFileList []fs.DirEntry
// }

func (mfs mockedFileSystem) OpenFile(name string, _ int, _ fs.FileMode) (fileInterface, error) {
	return mfs.f, mfs.err
}
func (mfs mockedFileSystem) Open(_ string) (fileInterface, error) { return mfs.f, mfs.err }
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
	if mfs.removeErr != nil {
		return mfs.removeErr
	}
	mfs.removeList = append(mfs.removeList, filepath.Base(name))
	return nil
}
func (mfs mockedFileSystem) Stat(name string) (fs.FileInfo, error) { return mfs.fileInfo, mfs.err }
func (mfs mockedFileSystem) Symlink(_, _ string) error {
	return mfs.symlinkErr
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
func (mfi mockedFileInfo) ModTime() time.Time { return mfi.time }
func (mfi mockedFileInfo) IsDir() bool        { return false }
func (mfi mockedFileInfo) Sys() any           { return nil }

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

// func TestCleanFilename(t *testing.T) {
// 	// not testing this, as its just a shortcut to filenamify.. fuck that
// }

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

	var makefn = func(name string, err error, isDir bool, size int64) fs.DirEntry {
		return mockedDirEntry{name: name, isDir: isDir, info: mockedFileInfo{name: name, size: size, err: err}}
	}

	for _, f := range filenames {
		defaultDirList = append(defaultDirList, makefn(f, nil, false, 1024))
	}

	// will insert into middle of the list
	var infoErrEntry = makefn("test.all.20220803_150000_infoerr.log", errors.New("foobar"), false, 1024)
	var dirEntry = makefn("test.all.20220803_150000_dir.log", nil, true, 1024)
	var sizeZeroEntry = makefn("test.all.20220803_150000_zerosize.log", nil, false, 0)

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

			testutils.AssertDiff(t, tt.expectedRemovals, tt.mockedfs.removeList)

		})
	}
}

func TestFindMostRecent(t *testing.T) {

	var makefn = func(name string, isDir bool, time time.Time) fs.DirEntry {
		return mockedDirEntry{name: name, isDir: isDir, info: mockedFileInfo{name: name, time: time}}
	}

	var today = time.Now()

	var filelist = []fs.DirEntry{
		//makefn("today.txt", false, timestamp),
		makefn("minusone.txt", false, today.AddDate(0, 0, -1)),
		makefn("minustwo.txt", false, today.AddDate(0, 0, -2)),
		makefn("minusthree.txt", false, today.AddDate(0, 0, -3)),
		makefn("minusfour.txt", false, today.AddDate(0, 0, -4)),
		makefn("minusfive.txt", false, today.AddDate(0, 0, -5)),
	}

	// randomize the filelist, for some entropy
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(filelist), func(i, j int) { filelist[i], filelist[j] = filelist[j], filelist[i] })

	var dirEntry = makefn("directory", true, today)
	var exeEntry = makefn("foo.exe", false, today)

	type args struct {
		path    string
		pattern string
	}
	tests := []struct {
		name     string
		args     args
		mockedfs *mockedFileSystem
		want     string
		wantErr  bool
	}{
		{
			"ReadDir() error",
			args{"foobar", "*.txt"},
			&mockedFileSystem{err: errors.New("foobar")},
			"",
			true,
		},
		{
			"bad pattern",
			args{"foobar", "*.[].txt"},
			&mockedFileSystem{dirlist: filelist},
			"",
			true,
		},
		{
			"nothing matches pattern",
			args{"foobar", "*.exe"},
			&mockedFileSystem{dirlist: filelist},
			"",
			true,
		},
		{
			"ignore directory",
			args{"foobar", "*.txt"},
			&mockedFileSystem{dirlist: CopyAndAppend(filelist, dirEntry)},
			filepath.Join("foobar", "minusone.txt"),
			false,
		},
		{
			"ignore not matched pattern",
			args{"foobar", "*.txt"},
			&mockedFileSystem{dirlist: CopyAndAppend(filelist, exeEntry)},
			filepath.Join("foobar", "minusone.txt"),
			false,
		},

		// not testing info returning error (file removed)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldFS := osimpl
			osimpl = tt.mockedfs
			defer func() { osimpl = oldFS }()

			got, err := FindMostRecent(tt.args.path, tt.args.pattern)

			testutils.AssertErr(t, tt.wantErr, err)
			testutils.AssertEquals(t, tt.want, got)
		})
	}
}

func TestCreateSymlink(t *testing.T) {
	type args struct {
		source  string
		symDest string
	}
	tests := []struct {
		name        string
		args        args
		mockedfs    *mockedFileSystem
		fileRemoved string
		wantErr     bool
	}{
		{
			"file exists error",
			args{"foobar\\foo", "foobar\\bar"},
			&mockedFileSystem{err: errors.New("foobar")},
			"",
			true,
		},
		{
			"remove dst error",
			args{"foobar\\foo", "foobar\\bar"},
			&mockedFileSystem{removeErr: errors.New("foobar")},
			"", // no file removed
			true,
		},
		{
			"create symlink error",
			args{"foobar\\foo", "foobar\\bar"},
			&mockedFileSystem{symlinkErr: errors.New("foobar")},
			"bar",
			true,
		},
		{
			"success + existing dst file doesn't exist",
			args{"foobar\\foo", "foobar\\bar"},
			&mockedFileSystem{err: os.ErrNotExist},
			"", // no file removed
			false,
		},
		{
			"success + existing dst file exist",
			args{"foobar\\foo", "foobar\\bar"},
			&mockedFileSystem{},
			"bar",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldFS := osimpl
			osimpl = tt.mockedfs
			defer func() { osimpl = oldFS }()

			err := CreateSymlink(tt.args.source, tt.args.symDest)

			testutils.AssertErr(t, tt.wantErr, err)
			var l = len(tt.mockedfs.removeList)
			if tt.fileRemoved == "" {
				testutils.Assert(t, l == 0, fmt.Sprint("remove list should be empty; len == ", l))
			} else {
				testutils.Assert(t, l == 1, fmt.Sprint("remove list should have one entry; len == ", l))
				testutils.AssertEquals(t, tt.fileRemoved, tt.mockedfs.removeList[0])
			}
		})
	}
}

func TestFileExists(t *testing.T) {
	type args struct {
		filename string
	}
	tests := []struct {
		name     string
		args     args
		mockedfs *mockedFileSystem
		want     bool
		wantErr  bool
	}{
		{
			"stat random error",
			args{"foobar"},
			&mockedFileSystem{err: errors.New("foobar")},
			false,
			true,
		},
		{
			"os.ErrNotExist",
			args{"foobar"},
			&mockedFileSystem{err: os.ErrNotExist},
			false,
			false,
		},
		{
			"file exists",
			args{"foobar"},
			&mockedFileSystem{fileInfo: mockedFileInfo{}},
			true,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldFS := osimpl
			osimpl = tt.mockedfs
			defer func() { osimpl = oldFS }()

			got, err := FileExists(tt.args.filename)

			testutils.AssertErr(t, tt.wantErr, err)
			testutils.AssertEquals(t, tt.want, got)
		})
	}
}
