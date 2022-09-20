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
func AssertEquals(tb testing.TB, exp, act any) {
	tb.Helper()
	if !reflect.DeepEqual(exp, act) {
		tb.Errorf("\033[31m \nexp: %#v \ngot: %#v \033[39m", exp, act)
	}
}

func AssertNotEquals(tb testing.TB, exp, act any) {
	tb.Helper()
	if reflect.DeepEqual(exp, act) {
		tb.Errorf("\033[31m \nexp: %#v \ngot: %#v \033[39m", exp, act)
	}
}

// checks if error is nil based on wantErr
// returns whether err == nil, for continuation checks
func AssertErr(tb testing.TB, wantErr bool, e error) bool {
	tb.Helper()
	if wantErr && e == nil {
		tb.Error("\033[31m exp:NotNil nil got:nil \033[39m")

	} else if wantErr == false && e != nil {
		tb.Errorf("\033[31m exp:nil got: %s \033[39m", e)
	}
	return e == nil
}

// check if error is nil via error string
// returns whether err == nil, for continuation checks
func AssertErrContains(tb testing.TB, contains string, e error) bool {
	tb.Helper()
	var isNil = AssertErr(tb, contains != "", e)
	if isNil == false {
		if (contains != "") && (strings.Contains(e.Error(), contains) == false) {
			tb.Errorf("\033[31m wanted error '%s' got: '%s' \033[39m", contains, e)
		}
	}
	return e == nil
}

// copies an entry (usually struct) by value, returns pointer to new copy
func Cp[T any](orig T) *T {
	// allocate new, copy entry values, return reference to new
	var cpy = new(T)
	*cpy = orig
	return cpy
}
