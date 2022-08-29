package podutils

import (
	"gopod/testutils"
	"reflect"
	"testing"
)

func TestCopyAndAppend(t *testing.T) {
	type args struct {
		src []string
		add []string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			"test strings",
			args{src: []string{"one", "two", "three"}, add: []string{"four", "five"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyAndAppend(tt.args.src, tt.args.add...)

			testutils.AssertEquals(t, len(got), (len(tt.args.src) + len(tt.args.add)))
			testutils.Assert(t, reflect.ValueOf(got).Pointer() != reflect.ValueOf(tt.args.src).Pointer(),
				"pointers should not match")
			testutils.Assert(t, reflect.ValueOf(got).Pointer() != reflect.ValueOf(tt.args.add).Pointer(),
				"pointers should not match")

			for i, s := range tt.args.src {
				testutils.AssertEquals(t, s, got[i])
			}
			for i, s := range tt.args.add {
				testutils.AssertEquals(t, s, got[len(tt.args.src)+i])
			}
		})
	}
}
