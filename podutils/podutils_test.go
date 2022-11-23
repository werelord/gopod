package podutils

import (
	"fmt"
	"gopod/testutils"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/go-test/deep"
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

func TestTern(t *testing.T) {
	type args struct {
		cond     bool
		trueval  string
		falseval string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"true", args{true, "foo", "bar"}, "foo"},
		{"false", args{false, "foo", "bar"}, "bar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Tern(tt.args.cond, tt.args.trueval, tt.args.falseval)
			testutils.AssertEquals(t, tt.want, got)
		})
	}
}

func TestChunk(t *testing.T) {

	rand.Seed(time.Now().UnixNano())
	type args struct {
		nilSlice  bool
		sliceSize int
		chunksize int
	}
	type exp struct {
		numChunks    int
		lenLastChunk int
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		{"nil slice", args{nilSlice: true}, exp{0, 0}},
		{"empty slice", args{}, exp{0, 0}},
		{"slice < chunk", args{sliceSize: 3, chunksize: 5}, exp{1, 3}},
		{"slice = chunk", args{sliceSize: 5, chunksize: 5}, exp{1, 0}},
		{"slice = chunk * x", args{sliceSize: 8, chunksize: 4}, exp{2, 0}},
		{"slice = chunk * x + y", args{sliceSize: 20, chunksize: 3}, exp{7, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// populate slice as needed
			var sl []int

			if tt.p.nilSlice {
				sl = nil
			} else {
				sl = make([]int, 0, tt.p.sliceSize)

				for i := 0; i < tt.p.sliceSize; i++ {
					sl = append(sl, rand.Intn(100))
				}
			}

			var ret = Chunk(sl, tt.p.chunksize)

			testutils.Assert(t, len(ret) == tt.e.numChunks,
				fmt.Sprintf("expected num chunks == %v, got %v", tt.e.numChunks, len(ret)))

			for idx, chunk := range ret {

				if (tt.e.lenLastChunk > 0) && (idx == (len(ret) - 1)) { // last chunk
					testutils.Assert(t, len(chunk) == tt.e.lenLastChunk,
						fmt.Sprintf("expected last chunk len == %v, got %v", tt.e.lenLastChunk, len(chunk)))

					diff := deep.Equal(chunk, sl[idx*tt.p.chunksize:])
					testutils.Assert(t, len(diff) == 0, fmt.Sprintf("difference: %v", diff))

				} else {
					testutils.Assert(t, len(chunk) == tt.p.chunksize,
						fmt.Sprintf("expected chunk %v len == %v, got %v", idx, tt.p.chunksize, len(chunk)))

					diff := deep.Equal(chunk, sl[idx*tt.p.chunksize:(idx+1)*tt.p.chunksize])
					testutils.Assert(t, len(diff) == 0, fmt.Sprintf("difference: %v", diff))

				}
			}
		})
	}
}
