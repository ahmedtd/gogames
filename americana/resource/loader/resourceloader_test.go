package loader

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseResourcePath(t *testing.T) {
	testCases := []struct {
		resourceID string
		wantDir    string
		wantTarget string
		wantError  error
	}{
		{
			resourceID: "//foo:bar",
			wantDir:    "foo",
			wantTarget: "bar",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.resourceID, func(t *testing.T) {
			gotDir, gotTarget, err := parseResourcePath(tc.resourceID)
			if diff := cmp.Diff(err, tc.wantError); diff != "" {
				t.Fatalf("Unexpected error output; diff (-got +want)\n%s", diff)
			}

			if gotDir != tc.wantDir {
				t.Errorf("Got resource dir %q, want %q", gotDir, tc.wantDir)
			}

			if gotTarget != tc.wantTarget {
				t.Errorf("Got target %q, want %q", gotTarget, tc.wantTarget)
			}
		})
	}
}
