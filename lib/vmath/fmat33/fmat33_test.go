package fmat33

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestOrthonormalize(t *testing.T) {
	testCases := []struct {
		In   T
		Want T
	}{
		{
			In: T{
				1, 0, 0,
				0, 1, 0,
				0, 0, 1,
			},
			Want: T{
				1, 0, 0,
				0, 1, 0,
				0, 0, 1,
			},
		},
		{
			In: T{
				2, 0, 0,
				0, 2, 0,
				0, 0, 2,
			},
			Want: T{
				1, 0, 0,
				0, 1, 0,
				0, 0, 1,
			},
		},
		{
			In: T{
				0.01, 0, 0,
				0, 0.01, 0,
				0, 0, 0.01,
			},
			Want: T{
				1, 0, 0,
				0, 1, 0,
				0, 0, 1,
			},
		},
		{
			In: T{
				1, 2, 0,
				0, 1, 0,
				0, 0, 1,
			},
			Want: T{
				1, 0, 0,
				0, 1, 0,
				0, 0, 1,
			},
		},
		{
			In: T{
				1, 2, 0,
				0, 0, 1,
				0, 1, 0,
			},
			Want: T{
				1, 0, 0,
				0, 0, 1,
				0, 1, 0,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v", tc.In), func(t *testing.T) {
			got := Orthonormalize(tc.In)
			if diff := cmp.Diff(got, tc.Want); diff != "" {
				t.Errorf("Bad result; diff (-got +want)\n%s", diff)
			}
		})
	}
}
