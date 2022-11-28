package inputoption

import (
	"gopod/testutils"
	"strings"
	"testing"
)

/*
func TestRunSelection(t *testing.T) {

	type args struct {
		description string
		opts        []*InputOption
	}
	tests := []struct {
		name    string
		args    args
		want    *InputOption
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RunSelection(tt.args.description, tt.args.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("RunSelection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RunSelection() = %v, want %v", got, tt.want)
			}
		})
	}
}
*/
/*
func TestRunRuneSelection(t *testing.T) {

	type args struct {
		description string
		opts        []*InputOption
	}
	tests := []struct {
		name    string
		args    args
		want    rune
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RunRuneSelection(tt.args.description, tt.args.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("RunRuneSelection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("RunRuneSelection() = %v, want %v", got, tt.want)
			}
		})
	}
}
*/
/*

func TestRunYesNoSelection(t *testing.T) {

	type args struct {
		description string
		def         YesNo
	}
	tests := []struct {
		name    string
		args    args
		want    YesNo
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RunYesNoSelection(tt.args.description, tt.args.def)
			if (err != nil) != tt.wantErr {
				t.Errorf("RunYesNoSelection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RunYesNoSelection() = %v, want %v", got, tt.want)
			}
		})
	}
}
*/
/*

func TestConfig_RunRuneSelection(t *testing.T) {

	type args struct {
		description string
		opts        []*InputOption
	}
	tests := []struct {
		name    string
		cfg     Config
		args    args
		want    rune
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.cfg.RunRuneSelection(tt.args.description, tt.args.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.RunRuneSelection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Config.RunRuneSelection() = %v, want %v", got, tt.want)
			}
		})
	}
}
*/
/*

func TestConfig_RunYesNoSelection(t *testing.T) {

	type args struct {
		description string
		def         YesNo
	}
	tests := []struct {
		name    string
		cfg     Config
		args    args
		want    YesNo
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.cfg.RunYesNoSelection(tt.args.description, tt.args.def)
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.RunYesNoSelection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Config.RunYesNoSelection() = %v, want %v", got, tt.want)
			}
		})
	}
}
*/
/*

func TestConfig_RunSelection(t *testing.T) {

	type args struct {
		description string
		opts        []*InputOption
	}
	tests := []struct {
		name    string
		cfg     Config
		args    args
		want    *InputOption
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.cfg.RunSelection(tt.args.description, tt.args.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.RunSelection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Config.RunSelection() = %v, want %v", got, tt.want)
			}
		})
	}
}
*/
/*

func TestConfig_buildOptionString(t *testing.T) {

	type args struct {
		desc string
		opt  []*InputOption
	}
	tests := []struct {
		name string
		cfg  Config
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.buildOptionString(tt.args.desc, tt.args.opt); got != tt.want {
				t.Errorf("Config.buildOptionString() = %v, want %v", got, tt.want)
			}
		})
	}
}
*/

func TestConfig_runSelection(t *testing.T) {

	type args struct {
		input string
	}
	type exp struct {
		expRune rune
		errStr  string
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		{"single character", args{input: "f"}, exp{expRune: 'f'}},
		{"single character 2", args{input: "b"}, exp{expRune: 'b'}},

		{"multi char", args{input: "foo"}, exp{expRune: 'f'}},
		{"multi char 2", args{input: "bar"}, exp{expRune: 'b'}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var testCfg = Config{
				input: strings.NewReader(tt.p.input),
			}

			// because vscode parsing doesn't handle output well when testme is printed on the same line as the output
			// include newline on that shit..
			r, err := testCfg.runSelection("testme\n")
			testutils.AssertErrContains(t, tt.e.errStr, err)
			testutils.AssertEquals(t, string(r), string(tt.e.expRune))
		})
	}
}

func TestConfig_getSelectedOption(t *testing.T) {

	var (
		optFoo = GenOption("foo", 'f', false)
		optBar = GenOption("bar", 'b', false)
		optArm = GenOption("arm", 'a', false)
		optLeg = GenOption("leg", 'l', false)
		optMeh = GenOption("meh", 'm', true)

		optsNoDefault   = []*InputOption{optFoo, optBar, optArm, optLeg}
		optsWithDefault = []*InputOption{optFoo, optBar, optArm, optMeh}
	)

	type args struct {
		r    rune
		opts []*InputOption
	}
	type exp struct {
		opt    *InputOption
		errStr string
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		{"no selection, no default", args{opts: optsNoDefault}, exp{errStr: "no valid selection and no default found"}},
		{"no selection, default set", args{opts: optsWithDefault}, exp{opt: optMeh}},
		{"invalid rune, no default", args{opts: optsNoDefault, r: '6'}, exp{errStr: "no valid selection and no default found"}},
		{"invalid rune, default set", args{opts: optsWithDefault, r: '6'}, exp{opt: optMeh}},
		{"select first with default", args{opts: optsNoDefault, r: 'f'}, exp{opt: optFoo}},
		{"select last no default", args{opts: optsWithDefault, r: 'a'}, exp{opt: optArm}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ret, err := defaultconfig.getSelectedOption(tt.p.r, tt.p.opts)
			testutils.AssertErrContains(t, tt.e.errStr, err)
			testutils.AssertEquals(t, ret, tt.e.opt)
		})
	}
}
