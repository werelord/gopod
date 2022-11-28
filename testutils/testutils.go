package testutils

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-test/deep"
	"golang.org/x/exp/slices"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// inspired by https://github.com/benbjohnson/testing

// assert fails the test if the condition is false.
func Assert(tb testing.TB, condition bool, msg string) {
	tb.Helper()
	if condition == false {
		tb.Errorf("\033[31m %s \033[39m\n\n", msg)
	}
}

// equals fails the test if exp is not equal to act.
func AssertEquals[T any](tb testing.TB, exp, act T) {
	tb.Helper()
	// if !reflect.DeepEqual(exp, act) {
	// 	tb.Errorf("\033[31m \nexp: %#v \ngot: %#v \033[39m", exp, act)
	// }
	if diff := deep.Equal(exp, act); diff != nil {
		str := "\033[31m\nObjects not equal:\033[39m\n"
		for _, d := range diff {
			str += fmt.Sprintf("\033[31m\t%v\033[39m\n", d)
		}
		tb.Error(str)
	}
}

func AssertNotEquals[T any](tb testing.TB, exp, act T) {
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

// --------------------------------------------------------------------------
// copies an entry (usually struct) by value, returns pointer to new copy
func Cp[T any](orig T) *T {
	// allocate new, copy entry values, return reference to new
	var cpy = new(T)
	*cpy = orig
	return cpy
}

// --------------------------------------------------------------------------
// takes in any type, returns a map of type names
func ListTypes(ty ...any) []string {
	var typelist = make([]string, 0, len(ty))
	for _, t := range ty {
		typelist = append(typelist, fmt.Sprintf("%T", t))
	}
	return typelist
}

// --------------------------------------------------------------------------
// checks all keys in want, make sure it exists in got; any not existing is returned in []missing
// checks all keys in got, make sure it exists in want, any not existing is returned in []extra
func AssertDiff[T comparable](tb testing.TB, wantList, gotList []T) {
	tb.Helper()

	// todo: rather than use slices.Contains, write our own contains func using deep.Equal()
	var (
		missing = make([]T, 0)
		extra   = make([]T, 0)
	)

	// for every in want, make sure it exists in got
	for _, want := range wantList {
		if slices.Contains(gotList, want) == false {
			missing = append(missing, want)
		}
	}
	// and vice versa
	for _, got := range gotList {
		if slices.Contains(wantList, got) == false {
			extra = append(extra, got)
		}
	}

	Assert(tb, len(missing) == 0, fmt.Sprintf("Missing types in gotList: %v", missing))
	Assert(tb, len(extra) == 0, fmt.Sprintf("Extra types in gotList: %v", extra))
}

func RandStringBytes(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func AssertDiffFunc[T comparable](tb testing.TB, wantList, gotList []T, comp func(T, T) bool) {
	tb.Helper()

	var (
		missing = make([]T, 0)
		extra   = make([]T, 0)
	)

	var indexOf = func(s []T, v T) int {
		for i, e := range s {
			if comp(e, v) {
				return i
			}
		}
		return -1
	}

	// for every in want, make sure it exists in got
	for _, want := range wantList {
		if indexOf(gotList, want) < 0 {
			missing = append(missing, want)
		}
	}
	// and vice versa
	for _, got := range gotList {
		if indexOf(wantList, got) < 0 {
			extra = append(extra, got)
		}
	}
	Assert(tb, len(missing) == 0, fmt.Sprintf("Missing types in gotList: %v", missing))
	Assert(tb, len(extra) == 0, fmt.Sprintf("Extra types in gotList: %v", extra))
}
