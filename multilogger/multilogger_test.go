package multilogger

import (
	"fmt"
	"gopod/testutils"
	"strings"
	"testing"

	charmlog "github.com/charmbracelet/log"
)

type logMsg struct {
	lvl, strStd, strFmt, one, two string

	affected []*strings.Builder
	procStd  func(logMsg)
}

func (lm logMsg) simStd(withkey, withval string) {

	var exp string
	if withkey == "" {
		exp = fmt.Sprintf("%s %s=%s\n", lm.strStd, lm.one, lm.two)
	} else {
		exp = fmt.Sprintf("%s %s=%s %s=%s\n", lm.strStd, withkey, withval, lm.one, lm.two)
	}

	if lm.lvl != "" {
		exp = fmt.Sprintf("%s %s", lm.lvl, exp)
	}
	for _, b := range lm.affected {
		b.WriteString(exp)
	}
}

func (lm logMsg) simFmt(withkey, withval string) {
	var exp string
	if withkey == "" {
		exp = fmt.Sprintf(lm.strFmt+"\n", lm.one, lm.two)
	} else {
		// format, the key/vals will copme after the string format
		exp = fmt.Sprintf(lm.strFmt+" %s=%s\n", lm.one, lm.two, withkey, withval)
	}
	if lm.lvl != "" {
		exp = fmt.Sprintf("%s %s", lm.lvl, exp)
	}
	for _, b := range lm.affected {
		b.WriteString(exp)
	}
}

func Test_Multilogger_global(t *testing.T) {

	var (
		console, debug, info, warn, err  strings.Builder
		expDbg, expInfo, expWarn, expErr strings.Builder
	)

	// not testing charmlog; just use basic options
	SetConsoleWithOptions(&console, charmlog.Options{Level: charmlog.DebugLevel})
	AddWithOptions(&debug, charmlog.Options{Level: charmlog.DebugLevel})
	AddWithOptions(&info, charmlog.Options{Level: charmlog.InfoLevel})
	AddWithOptions(&warn, charmlog.Options{Level: charmlog.WarnLevel})
	AddWithOptions(&err, charmlog.Options{Level: charmlog.ErrorLevel})

	var logs = []logMsg{
		{"DEBU", "debug test", "debug test_%s:%s", "foo", "bar",
			[]*strings.Builder{&expDbg},
			func(lm logMsg) { Debug(lm.strStd, lm.one, lm.two); Debugf(lm.strFmt, lm.one, lm.two) },
		},
		{"INFO", "info test", "info test_%s:%s", "arm", "leg",
			[]*strings.Builder{&expDbg, &expInfo},
			func(lm logMsg) { Info(lm.strStd, lm.one, lm.two); Infof(lm.strFmt, lm.one, lm.two) },
		},
		{"WARN", "Warn test", "Warn test_%s:%s", "bar", "foo",
			[]*strings.Builder{&expDbg, &expInfo, &expWarn},
			func(lm logMsg) { Warn(lm.strStd, lm.one, lm.two); Warnf(lm.strFmt, lm.one, lm.two) },
		},
		{"ERRO", "error test", "error test_%s:%s", "me", "too",
			[]*strings.Builder{&expDbg, &expInfo, &expWarn, &expErr},
			func(lm logMsg) { Error(lm.strStd, lm.one, lm.two); Errorf(lm.strFmt, lm.one, lm.two) },
		},
		{"", "print test", "print test_%s:%s", "print", "you",
			[]*strings.Builder{&expDbg, &expInfo, &expWarn, &expErr},
			func(lm logMsg) { Print(lm.strStd, lm.one, lm.two); Printf(lm.strFmt, lm.one, lm.two) },
		},
	}

	// run funcs
	for _, l := range logs {
		l.simStd("", "")
		l.simFmt("", "")
		l.procStd(l)
	}

	// check writes
	testutils.AssertEquals(t, expDbg.String(), console.String())
	testutils.AssertEquals(t, expDbg.String(), debug.String())
	testutils.AssertEquals(t, expInfo.String(), info.String())
	testutils.AssertEquals(t, expWarn.String(), warn.String())
	testutils.AssertEquals(t, expErr.String(), err.String())
}

func Test_Multilogger_member(t *testing.T) {

	var (
		console, debug, info, warn, err  strings.Builder
		expDbg, expInfo, expWarn, expErr strings.Builder
	)

	// not testing charmlog; just use basic options
	SetConsoleWithOptions(&console, charmlog.Options{Level: charmlog.DebugLevel})
	AddWithOptions(&debug, charmlog.Options{Level: charmlog.DebugLevel})
	AddWithOptions(&info, charmlog.Options{Level: charmlog.InfoLevel})
	AddWithOptions(&warn, charmlog.Options{Level: charmlog.WarnLevel})
	AddWithOptions(&err, charmlog.Options{Level: charmlog.ErrorLevel})

	var withKey = "id"
	var withVal = "42"
	var with = With(withKey, withVal)

	var logs = []logMsg{
		{"DEBU", "debug test", "debug test_%s:%s", "foo", "bar",
			[]*strings.Builder{&expDbg},
			func(lm logMsg) { with.Debug(lm.strStd, lm.one, lm.two); with.Debugf(lm.strFmt, lm.one, lm.two) },
		},
		{"INFO", "info test", "info test_%s:%s", "arm", "leg",
			[]*strings.Builder{&expDbg, &expInfo},
			func(lm logMsg) { with.Info(lm.strStd, lm.one, lm.two); with.Infof(lm.strFmt, lm.one, lm.two) },
		},
		{"WARN", "Warn test", "Warn test_%s:%s", "bar", "foo",
			[]*strings.Builder{&expDbg, &expInfo, &expWarn},
			func(lm logMsg) { with.Warn(lm.strStd, lm.one, lm.two); with.Warnf(lm.strFmt, lm.one, lm.two) },
		},
		{"ERRO", "error test", "error test_%s:%s", "me", "too",
			[]*strings.Builder{&expDbg, &expInfo, &expWarn, &expErr},
			func(lm logMsg) { with.Error(lm.strStd, lm.one, lm.two); with.Errorf(lm.strFmt, lm.one, lm.two) },
		},
		{"", "print test", "print test_%s:%s", "print", "you",
			[]*strings.Builder{&expDbg, &expInfo, &expWarn, &expErr},
			func(lm logMsg) { with.Print(lm.strStd, lm.one, lm.two); with.Printf(lm.strFmt, lm.one, lm.two) },
		},
	}

	// run funcs
	for _, l := range logs {
		l.simStd(withKey, withVal)
		l.simFmt(withKey, withVal)
		l.procStd(l)
	}

	// check writes
	testutils.AssertEquals(t, expDbg.String(), console.String())
	testutils.AssertEquals(t, expDbg.String(), debug.String())
	testutils.AssertEquals(t, expInfo.String(), info.String())
	testutils.AssertEquals(t, expWarn.String(), warn.String())
	testutils.AssertEquals(t, expErr.String(), err.String())
}
