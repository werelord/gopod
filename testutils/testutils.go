package testutils

import (
	"reflect"
	"testing"
)

// inspired by https://github.com/benbjohnson/testing

// assert fails the test if the condition is false.
func Assert(tb testing.TB, condition bool, msg string) {
	if condition == false {
		tb.Errorf("\033[31m %s \033[39m\n\n", msg)
	}
}

// ok fails the test if an err is not nil.
// func ok(tb testing.TB, err error) {
// 	if err != nil {
// 		fmt.Printf("\033[31m unexpected error: %s\033[39m\n\n", err.Error())
// 		tb.FailNow()
// 	}
// }

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
