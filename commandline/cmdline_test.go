package commandline

import (
	"fmt"
	. "gopod/podutils"
	"gopod/testutils"
	"os"
	"path/filepath"
	"testing"
)

// integration tests

func TestInitCommandLine(t *testing.T) {

	var allFlags = []string{
		"--config", "barfoo.toml",
		"--feed=foo",
		"--backup-db",
		"--debug",
		"--proxy=barfoo",
		"--simulate",
		"--force",
		"--userecent",
		"--archive",
		"--rename",
		"--collision",
		"--savecollision",
		"--set-downloaded",
	}

	ex, _ := os.Executable()
	var barFooConfig = filepath.Join(filepath.Dir(ex), "barfoo.toml")

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
		{"unknown command", args{args: []string{"foobar", "--config", "barfoo.toml"}}, expect{errStr: "command not recognized"}},
		{"blank config", args{args: []string{"update"}, blankCfg: true}, expect{errStr: "config required"}},
		{"config not exist", args{args: []string{"update", "--config", "barfoo.toml"}, cfgDNE: true},
			expect{errStr: "cannot find config file"}},
		{"empty args", args{args: []string{}}, expect{errStr: "config required"}},
		{"empty args (minus config)", args{args: []string{"--config", "barfoo.toml"}}, expect{errStr: "command not recognized"}},
		{"help called", args{args: []string{"help"}}, expect{errStr: "help called"}},
		{"help command called", args{args: []string{"help", "update"}}, expect{errStr: "help called"}},

		// success results.. don't use named parameters, so that changes to underlying struct
		// will make this test fail to compile
		{"global false", args{args: []string{"update", "--config", "barfoo.toml"}},
			expect{cmdline: CommandLine{barFooConfig, Update, "", "",
				CommandLineOptions{false, false, UpdateOpt{false, false, false, false}, CheckDownloadOpt{false, false, false, false}}}},
		},
		{"global true", args{args: []string{"update", "--config", "barfoo.toml", "--feed=foo", "--backup-db", "--debug", "--proxy=barfoo"}},
			expect{cmdline: CommandLine{barFooConfig, Update, "foo", "barfoo",
				CommandLineOptions{true, true, UpdateOpt{false, false, false, false}, CheckDownloadOpt{false, false, false, false}}}},
		},

		// flags dependent on command, regardless on whether they're on the commandline or not
		{"update dependant", args{args: CopyAndAppend([]string{"update"}, allFlags...)},
			expect{cmdline: CommandLine{barFooConfig, Update, "foo", "barfoo",
				CommandLineOptions{true, true, UpdateOpt{true, true, true, true}, CheckDownloadOpt{false, false, false, false}}}},
		},
		{"check downloads dependant", args{args: CopyAndAppend([]string{"checkdownloads"}, allFlags...)},
			expect{cmdline: CommandLine{barFooConfig, CheckDownloaded, "foo", "barfoo",
				CommandLineOptions{true, true, UpdateOpt{false, false, false, false}, CheckDownloadOpt{true, true, true, true}}}},
		},
		{"preview dependant", args{args: CopyAndAppend([]string{"preview"}, allFlags...)},
			expect{cmdline: CommandLine{barFooConfig, Preview, "foo", "barfoo",
				// also uses useRecent
				CommandLineOptions{true, true, UpdateOpt{false, false, true, false}, CheckDownloadOpt{false, false, false, false}}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var oldFE = fileExists
			fileExists = func(string) (bool, error) {
				return tt.args.cfgDNE == false, nil
			}
			defer func() { fileExists = oldFE }()

			retCmdLine, err := InitCommandLine( /*Tern(tt.args.blankCfg, "", "defaultConfig"), */ tt.args.args)

			testutils.AssertErrContains(t, tt.expect.errStr, err)
			testutils.Assert(t, (retCmdLine == nil) == (err != nil),
				fmt.Sprintf("wtf, retcmdline: '%v', err: '%v'", retCmdLine, err))

			if retCmdLine != nil {
				testutils.AssertEquals(t, tt.expect.cmdline, *retCmdLine)
			}
		})
	}
}
