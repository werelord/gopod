package podutils

import (
	"errors"
	"gopod/testutils"
	"os"
	"testing"
)

func TestLoadFile(t *testing.T) {
	type args struct {
		filename string
	}

	tests := []struct {
		name         string
		args         args
		mockedfs     *mockedFileSystem
		expect       string
		wantNilError bool
	}{
		{
			"open error",
			args{filename: "open error"},
			&mockedFileSystem{err: os.ErrNotExist},
			"",
			false,
		},
		{
			"file error",
			args{filename: "file error"},
			&mockedFileSystem{err: nil, f: &mockedFile{ret: "", err: errors.New("foobar")}},
			"",
			false,
		},
		{
			"success",
			args{filename: "succes"},
			&mockedFileSystem{f: &mockedFile{ret: "foobar"}},
			"foobar",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldFS := osfs
			//mockedfs := tt.mockedfs
			osfs = tt.mockedfs
			// restore after the tests
			defer func() {
				osfs = oldFS
			}()

			res, err := LoadFile(tt.args.filename)

			testutils.AssertEquals(t, tt.expect, string(res))
			testutils.AssertErr(t, tt.wantNilError, err)
		})
	}
}
