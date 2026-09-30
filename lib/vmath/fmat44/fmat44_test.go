package fmat44

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

func TestMulMV(t *testing.T) {
	testCases := []struct {
		A    T
		B    fvec4.T
		Want fvec4.T
	}{
		{
			A:    Identity(),
			B:    fvec4.T{1, 2, 3, 4},
			Want: fvec4.T{1, 2, 3, 4},
		},
		{
			A: T{
				2, 0, 0, 0,
				0, 2, 0, 0,
				0, 0, 2, 0,
				0, 0, 0, 2,
			},
			B:    fvec4.T{1, 2, 3, 4},
			Want: fvec4.T{2, 4, 6, 8},
		},
		{
			// A permutation matrix, so each output element depends on a
			// different single input element -- catches row/column mixups.
			A: T{
				0, 1, 0, 0,
				0, 0, 1, 0,
				0, 0, 0, 1,
				1, 0, 0, 0,
			},
			B:    fvec4.T{10, 20, 30, 40},
			Want: fvec4.T{20, 30, 40, 10},
		},
		{
			A: T{
				1, 2, 3, 4,
				5, 6, 7, 8,
				9, 10, 11, 12,
				13, 14, 15, 16,
			},
			B:    fvec4.T{1, 1, 1, 1},
			Want: fvec4.T{10, 26, 42, 58},
		},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v*%v", tc.A, tc.B), func(t *testing.T) {
			got := MulMV(tc.A, tc.B)
			if diff := cmp.Diff(got, tc.Want); diff != "" {
				t.Errorf("Bad result; diff (-got +want)\n%s", diff)
			}
		})
	}
}
