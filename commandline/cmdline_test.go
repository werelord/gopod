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
		// global options
		"--config", "barfoo.toml",
		"--feed=foo",
		"--backup-db",
		"--debug",
		"--proxy=barfoo",
		"--log=debug",

		// update specific options
		"--simulate",
		"--force",
		"--userecent",
		"--set-downloaded",
		"--download-after", "2023-04-01",

		// check download options
		"--archive",
		"--rename",
		"--collision",
		"--savecollision",

		// export options
		"--include-deleted",
		// "--format=db",	// will be in commandline directly
		"--export-path=foo",
	}

	var (
		globalTrue = GlobalOpt{BackupDb: true, Debug: true, LogLevelStr: "debug"}
		updateTrue = UpdateOpt{Simulate: true, ForceUpdate: true, UseMostRecentXml: true,
			MarkDownloaded: true, DownloadAfter: "2023-04-01"}
		checkdlTrue   = CheckDownloadOpt{DoArchive: true, DoRename: true, SaveCollision: true, DoCollision: true}
		exportDefTrue = ExportOpt{IncludeDeleted: true, ExportFormat: ExportJson, ExportPath: "foo"}
		exportJson    = exportDefTrue
		exportDB      = ExportOpt{IncludeDeleted: true, ExportFormat: ExportDB, ExportPath: "foo"}
	)

	ex, _ := os.Executable()
	var barFooConfig = filepath.Join(filepath.Dir(ex), "barfoo.toml")

	type args struct {
		args     []string
		blankCfg bool
		cfgDNE   bool
	}
	type exp struct {
		cmdline CommandLine
		errStr  string
	}
	tests := []struct {
		name   string
		args   args
		expect exp
	}{
		// errors
		{"unknown command", args{args: []string{"foobar", "--config", "barfoo.toml"}}, exp{errStr: "command not recognized"}},
		{"blank config", args{args: []string{"update"}, blankCfg: true}, exp{errStr: "config required"}},
		{"config not exist", args{args: []string{"update", "--config", "barfoo.toml"}, cfgDNE: true},
			exp{errStr: "cannot find config file"}},
		{"empty args", args{args: []string{}}, exp{errStr: "config required"}},
		{"empty args (minus config)", args{args: []string{"--config", "barfoo.toml"}}, exp{errStr: "command not recognized"}},
		{"help called", args{args: []string{"help"}}, exp{errStr: "help called"}},
		{"help command called", args{args: []string{"help", "update"}}, exp{errStr: "help called"}},

		// success results.. don't use named parameters, so that changes to underlying struct
		// will make this test fail to compile
		{"global false",
			args{args: []string{"update", "--config", "barfoo.toml"}},
			exp{cmdline: CommandLine{barFooConfig, Update, "", "",
				CommandLineOptions{GlobalOpt: GlobalOpt{LogLevelStr: "info"}}}},
		},
		{"global true",
			args{args: []string{"update", "--config", "barfoo.toml", "--feed=foo", "--backup-db", "--debug", "--proxy=barfoo", "--log=debug"}},
			exp{cmdline: CommandLine{barFooConfig, Update, "foo", "barfoo",
				CommandLineOptions{GlobalOpt: globalTrue}},
			},
		},

		// flags dependent on command, regardless on whether they're on the commandline or not
		{"update dependant",
			args{args: CopyAndAppend([]string{"update"}, allFlags...)},
			exp{cmdline: CommandLine{barFooConfig, Update, "foo", "barfoo",
				CommandLineOptions{GlobalOpt: globalTrue, UpdateOpt: updateTrue}}},
		},
		{"check downloads dependant",
			args{args: CopyAndAppend([]string{"checkdownloads"}, allFlags...)},
			exp{cmdline: CommandLine{barFooConfig, CheckDownloaded, "foo", "barfoo",
				CommandLineOptions{GlobalOpt: globalTrue, CheckDownloadOpt: checkdlTrue}}},
		},
		{"preview dependant", args{args: CopyAndAppend([]string{"preview"}, allFlags...)},
			exp{cmdline: CommandLine{barFooConfig, Preview, "foo", "barfoo",
				// also uses useRecent
				CommandLineOptions{GlobalOpt: globalTrue, UpdateOpt: UpdateOpt{UseMostRecentXml: true}}}},
		},
		// various export specfic tests
		{"export err unrecognized format", args{args: CopyAndAppend([]string{"export", "--format=foo"}, allFlags...)},
			exp{errStr: "unrecognized export format"},
		},
		{"export dependant (default)", args{args: CopyAndAppend([]string{"export"}, allFlags...)},
			exp{cmdline: CommandLine{barFooConfig, Export, "foo", "barfoo",
				CommandLineOptions{GlobalOpt: globalTrue, ExportOpt: exportDefTrue}}},
		},
		{"export json", args{args: CopyAndAppend([]string{"export", "--format=json"}, allFlags...)},
			exp{cmdline: CommandLine{barFooConfig, Export, "foo", "barfoo",
				CommandLineOptions{GlobalOpt: globalTrue, ExportOpt: exportJson}}},
		},
		{"export db", args{args: CopyAndAppend([]string{"export", "--format=db"}, allFlags...)},
			exp{cmdline: CommandLine{barFooConfig, Export, "foo", "barfoo",
				CommandLineOptions{GlobalOpt: globalTrue, ExportOpt: exportDB}}},
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
