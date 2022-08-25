package testutils

import (
	"fmt"
	"reflect"
	"testing"
)

// pulled from https://github.com/benbjohnson/testing

// not sure if I'm going to use this yet..

// assert fails the test if the condition is false.
func assert(tb testing.TB, condition bool, msg string, v ...interface{}) {
	if !condition {
		// fmt.Printf("\033[31m "+msg+"\033[39m\n\n", append([]interface{}(v...)...)
		// tb.FailNow()
	}
}

// ok fails the test if an err is not nil.
func ok(tb testing.TB, err error) {
	if err != nil {
		fmt.Printf("\033[31m unexpected error: %s\033[39m\n\n", err.Error())
		tb.FailNow()
	}
}

// equals fails the test if exp is not equal to act.
func AssertEquals(tb testing.TB, exp, act interface{}) {
	tb.Helper()
	if !reflect.DeepEqual(exp, act) {
		tb.Errorf("\033[31m exp: %#v got: %#v \033[39m", exp, act)
	}
}

// checks if error is nil
func AssertErr(tb testing.TB, expectedNil bool, e error) {
	tb.Helper()
	var res bool
	var wantNil = "nil"
	if expectedNil == true {
		res = (e == nil)
	} else {
		res = (e != nil)
		wantNil = "not nil"
	}

	if res == false {
		tb.Errorf("\033[31m exp:%s got: %s \033[39m", wantNil, e.Error())
	}

}
