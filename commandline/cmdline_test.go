package commandline

import (
	"errors"
	"fmt"
	. "gopod/podutils"
	"gopod/testutils"
	"testing"
)

// integration tests

func TestInitCommandLine(t *testing.T) {

	var allFlags = []string{
		"--config", "barfoo.toml",
		"--feed=foo",
		"--debug",
		"--proxy=barfoo",
		"--simulate",
		"--force",
		"--userecent",
		"--archive",
		"--rename",
		"--collision",
	}

	type args struct {
		args     []string
		blankCfg bool
		cfgDNE   bool
	}
	type expect struct {
		cmdline CommandLine
		errStr  string
	}
	tests := []struct {
		name   string
		args   args
		expect expect
	}{
		// errors
		{"unknown command", args{args: []string{"foobar"}}, expect{errStr: "command not recognized"}},
		{"blank config", args{args: []string{"update"}, blankCfg: true}, expect{errStr: "config cannot be blank"}},
		{"config not exist", args{args: []string{"update"}, cfgDNE: true}, expect{errStr: "cannot find config file"}},
		{"empty args", args{args: []string{}}, expect{errStr: "command not recognized"}},
		{"help called", args{args: []string{"help"}}, expect{errStr: "help called"}},
		{"help command called", args{args: []string{"help", "update"}}, expect{errStr: "help called"}},

		// success results.. don't use named parameters, so that changes to underlying struct
		// will make this test fail to compile
		{"global false", args{args: []string{"update"}},
			expect{cmdline: CommandLine{"defaultConfig", Update, "", "",
				CommandLineOptions{false, UpdateOpt{false, false, false}, CheckDownloadOpt{false, false, false}}}},
		},
		{"global true", args{args: []string{"update", "--feed=foo", "--debug", "--proxy=barfoo"}},
			expect{cmdline: CommandLine{"defaultConfig", Update, "foo", "barfoo",
				CommandLineOptions{true, UpdateOpt{false, false, false}, CheckDownloadOpt{false, false, false}}}},
		},

		// flags dependent on command, regardless on whether they're on the commandline or not
		{"update dependant", args{args: CopyAndAppend([]string{"update"}, allFlags...)},
			expect{cmdline: CommandLine{"barfoo.toml", Update, "foo", "barfoo",
				CommandLineOptions{true, UpdateOpt{true, true, true}, CheckDownloadOpt{false, false, false}}}},
		},
		{"check downloads dependant", args{args: CopyAndAppend([]string{"checkdownloads"}, allFlags...)},
			expect{cmdline: CommandLine{"barfoo.toml", CheckDownloaded, "foo", "barfoo",
				CommandLineOptions{true, UpdateOpt{false, false, false}, CheckDownloadOpt{true, true, true}}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var oldFE = fileExists
			fileExists = func(string) (bool, error) {
				return tt.args.cfgDNE == false, Tern(tt.args.cfgDNE, errors.New("foobar"), nil)
			}
			defer func() { fileExists = oldFE }()

			retCmdLine, err := InitCommandLine(Tern(tt.args.blankCfg, "", "defaultConfig"), tt.args.args)

			testutils.AssertErrContains(t, tt.expect.errStr, err)
			testutils.Assert(t, (retCmdLine == nil) == (err != nil),
				fmt.Sprintf("wtf, retcmdline: '%v', err: '%v'", retCmdLine, err))

			if retCmdLine != nil {
				testutils.AssertEquals(t, tt.expect.cmdline, *retCmdLine)
			}
		})
	}
}
