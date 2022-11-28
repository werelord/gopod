package inputoption

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

type YesNo rune

const (
	YES  YesNo = 'y'
	NO   YesNo = 'n'
	Skip rune  = '\n'
)

type InputOption struct {
	Title   string
	Rune    rune
	Default bool
}

type Config struct {
	// todo: here
	OneLine          bool
	DefaultRuneUpper bool

	input io.Reader
}

// config with default values
var defaultconfig = Config{input: os.Stdin}

// helper function for generating input options
func GenOption(desc string, r rune, def bool) *InputOption {
	return &InputOption{desc, r, def}
}

// use to make changes to input option configuration; only needed if default values need to be changed
func WithConfig(cfg Config) Config {
	// make sure stdin is set
	cfg.input = os.Stdin
	return cfg
}

// perform option selection with the default configurations
func RunSelection(description string, opts ...*InputOption) (*InputOption, error) {
	return defaultconfig.RunSelection(description, opts...)
}

func RunRuneSelection(description string, opts ...*InputOption) (rune, error) {
	return defaultconfig.RunRuneSelection(description, opts...)
}

// perform yes/no selection with the default configuration
func RunYesNoSelection(description string, def YesNo) (YesNo, error) {
	return defaultconfig.RunYesNoSelection(description, def)
}

// perform option selection, returning rune associated with selected option
func (cfg Config) RunRuneSelection(description string, opts ...*InputOption) (rune, error) {
	ret, err := cfg.RunSelection(description, opts...)
	return ret.Rune, err
}

// perform yes/no selection.. one line and default rune upper-case is always set
func (cfg Config) RunYesNoSelection(description string, def YesNo) (YesNo, error) {
	cfg.OneLine = true
	cfg.DefaultRuneUpper = true

	var (
		yesOpt = GenOption("", rune(YES), def == YES)
		noOpt  = GenOption("", rune(NO), def != YES)
	)

	if ret, err := cfg.RunSelection(description, yesOpt, noOpt); err != nil {
		return def, err
	} else {
		// make it explicit; only two valid outputs (plus default)
		switch ret.Rune {
		case rune(YES):
			return YES, nil
		case rune(NO):
			return NO, nil
		default:
			return def, nil
		}
	}
}

// perform option selection with given options
func (cfg Config) RunSelection(description string, opts ...*InputOption) (*InputOption, error) {
	if len(opts) < 2 {
		return nil, errors.New("not enough options given")
	}

	var (
		optStr = cfg.buildOptionString(description, opts)
	)
	if r, err := cfg.runSelection(optStr); err != nil {
		return nil, err
	} else {
		return cfg.getSelectedOption(r, opts)
	}
}

func (cfg Config) buildOptionString(desc string, opt []*InputOption) string {
	var ret string
	var runeList string
	for _, o := range opt {
		var runeStr string
		if o.Rune == Skip {
			runeStr = "<skip>"
		} else {
			runeStr = string(o.Rune)
		}
		if cfg.DefaultRuneUpper && o.Default {
			runeStr = strings.ToUpper(runeStr)
		}
		if cfg.OneLine == false {
			ret += fmt.Sprintf("'%v' (%v)\n", o.Title, runeStr)
		}
		if len(runeList) > 0 {
			runeList += "|"
		}
		runeList += runeStr
	}
	ret += fmt.Sprintf("%v (%v): ", desc, runeList)
	return ret
}

func (cfg Config) runSelection(optStr string) (rune, error) {
	scanner := bufio.NewScanner(cfg.input) // defaults to stdin
	fmt.Print(optStr)
	if scanner.Scan(); scanner.Err() != nil {
		return 0, scanner.Err()
	}
	
	r, _ := utf8.DecodeRuneInString(scanner.Text())
	return r, nil
}

func (cfg Config) getSelectedOption(r rune, opts []*InputOption) (*InputOption, error) {
	var (
		defOpt *InputOption
	)
	for _, opt := range opts {
		if opt.Default {
			defOpt = opt
		}
		if r == opt.Rune {
			return opt, nil
		}
	}
	if defOpt == nil {
		return defOpt, errors.New("no valid selection and no default found")
	} else {
		return defOpt, nil
	}
}
