package mat33

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMulMM(t *testing.T) {
	testCases := []struct {
		desc string
		a    T
		b    T
		want T
	}{
		{
			desc: "identity",
			a:    Identity(),
			b:    Identity(),
			want: Identity(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			got := MulMM(tc.a, tc.b)
			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Fatalf("Bad; diff (-got +want):\n%s", diff)
			}
		})
	}
}
