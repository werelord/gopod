package inputoption

import (
	"fmt"
	"gopod/testutils"
	"strings"
	"testing"
)

func TestConfig_RunYesNoSelection(t *testing.T) {

	type args struct {
		input string
		def   YesNo
	}
	type exp struct {
		ret YesNo
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		{"bad input, default yes", args{input: "foo", def: YES}, exp{ret: YES}},
		{"bad input, default no", args{input: "foo", def: YES}, exp{ret: YES}},
		{"yes selected", args{input: "y", def: NO}, exp{ret: YES}},
		{"no selected", args{input: "n", def: YES}, exp{ret: NO}},
		{"uppercase yes selected", args{input: "Y", def: NO}, exp{ret: YES}},
		{"uppercase no selected", args{input: "N", def: YES}, exp{ret: NO}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var testCfg = Config{
				input: strings.NewReader(tt.p.input),
			}

			ret, err := testCfg.runYesNoSelection("testing", tt.p.def)
			fmt.Printf("\n")
			testutils.Assert(t, err == nil, fmt.Sprintf("expecting error nil, got %v", err))
			testutils.AssertEquals(t, tt.e.ret, ret)
		})
	}
}

func TestConfig_RunSelection(t *testing.T) {
	var (
		optFoo = GenOption("foo", 'f', false)
		optBar = GenOption("bar", 'b', false)
		optArm = GenOption("arm", 'a', false)
		optLeg = GenOption("leg", 'l', true)

		opts = []*InputOption{optFoo, optBar, optArm, optLeg}
	)

	type args struct {
		input string
		opts  []*InputOption
	}
	type exp struct {
		exp    *InputOption
		errStr string
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		{"no opts", args{input: "foobar"}, exp{errStr: "not enough options"}},
		{"one opt", args{input: "foobar", opts: opts[:1]}, exp{errStr: "not enough options"}},
		{"no default, bad input", args{input: "meh", opts: opts[:3]}, exp{errStr: "no valid selection and no default found"}},
		{"default, bad input", args{input: "meh", opts: opts}, exp{exp: optLeg}},
		{"select something", args{input: "b", opts: opts}, exp{exp: optBar}},
		{"select something uppercase", args{input: "A", opts: opts}, exp{exp: optArm}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var testCfg = Config{
				input: strings.NewReader(tt.p.input),
			}

			// because vscode parsing doesn't handle output well when testme is printed on the same line as the output
			// include newline after

			opt, err := testCfg.RunSelection("foobar", tt.p.opts...)
			fmt.Print("\n")

			testutils.AssertErrContains(t, tt.e.errStr, err)
			testutils.AssertEquals(t, tt.e.exp, opt)
		})
	}
}

func TestConfig_buildOptionString(t *testing.T) {

	var (
		optsYesDefault = []*InputOption{GenOption("", rune(YES), true), GenOption("", rune(NO), false)}
		optsNoDefault  = []*InputOption{GenOption("", rune(YES), false), GenOption("", rune(NO), true)}

		optFoo  = GenOption("foo", 'f', false)
		optBar  = GenOption("bar", 'b', false)
		optArm  = GenOption("arm", 'a', true)
		optLeg  = GenOption("leg", 'l', false)
		optSkip = GenOption("skipOption", '\n', false)

		opts         = []*InputOption{optFoo, optBar, optArm, optLeg}
		optsWithSkip = []*InputOption{optFoo, optBar, optArm, optLeg, optSkip}
	)

	type args struct {
		oneLine  bool
		defUpper bool
		desc     string
		opt      []*InputOption
	}
	tests := []struct {
		name string
		p    args
		exp  string
	}{
		{"two options", args{desc: "foobar", opt: opts[:2]},
			"'foo' (f)\n'bar' (b)\nfoobar (f|b): "},
		{"four options", args{desc: "barfoo", opt: opts},
			"'foo' (f)\n'bar' (b)\n'arm' (a)\n'leg' (l)\nbarfoo (f|b|a|l): "},
		{"yes rune uppercase", args{oneLine: true, defUpper: true, desc: "foobar:", opt: optsYesDefault},
			"foobar: (Y|n): "},
		{"no rune uppercase", args{oneLine: true, defUpper: true, desc: "barfoo - ", opt: optsNoDefault},
			"barfoo -  (y|N): "},
		{"one line option", args{oneLine: true, desc: "meh:", opt: opts},
			"meh: (f|b|a|l): "},
		{"skip option", args{desc: "meh", opt: optsWithSkip},
			"'foo' (f)\n'bar' (b)\n'arm' (a)\n'leg' (l)\n'skipOption' (<skip>)\nmeh (f|b|a|l|<skip>): "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var cfg = Config{
				OneLine:          tt.p.oneLine,
				DefaultRuneUpper: tt.p.defUpper,
			}
			var ret = cfg.buildOptionString(tt.p.desc, tt.p.opt)
			testutils.AssertEquals(t, tt.exp, ret)
		})
	}
}

func TestConfig_selectionInput(t *testing.T) {

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
			r, err := testCfg.selectionInput("testme\n")
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

		{"uppercase with default", args{opts: optsNoDefault, r: 'F'}, exp{opt: optFoo}},
		{"uppercase no default", args{opts: optsWithDefault, r: 'A'}, exp{opt: optArm}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ret, err := defaultconfig.getSelectedOption(tt.p.r, tt.p.opts)
			testutils.AssertErrContains(t, tt.e.errStr, err)
			testutils.AssertEquals(t, ret, tt.e.opt)
		})
	}
}
