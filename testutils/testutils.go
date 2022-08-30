package testutils

import (
	"reflect"
	"strings"
	"testing"
)

// inspired by https://github.com/benbjohnson/testing

// assert fails the test if the condition is false.
func Assert(tb testing.TB, condition bool, msg string) {
	if condition == false {
		tb.Errorf("\033[31m %s \033[39m\n\n", msg)
	}
}

// equals fails the test if exp is not equal to act.
func AssertEquals(tb testing.TB, exp, act interface{}) {
	tb.Helper()
	if !reflect.DeepEqual(exp, act) {
		tb.Errorf("\033[31m \nexp: %#v \ngot: %#v \033[39m", exp, act)
	}
}

// checks if error is nil
func AssertErr(tb testing.TB, wantErr bool, e error) {
	tb.Helper()
	if wantErr && e == nil {
		tb.Error("\033[31m exp:NotNil nil got:nil \033[39m")

	} else if wantErr == false && e != nil {
		tb.Errorf("\033[31m exp:nil got: %s \033[39m", e)
	}
}

// check if error is nil via error string
func AssertErrContains(tb testing.TB, contains string, e error) {
	tb.Helper()
	AssertErr(tb, contains != "", e)
	if (contains != "") && (strings.Contains(e.Error(), contains) == false) {
		tb.Errorf("\033[31m expected contains '%s' got: '%s' \033[39m", contains, e)
	}
}
