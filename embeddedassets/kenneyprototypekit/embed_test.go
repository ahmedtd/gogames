package kenneyprototypekit

import "testing"

func TestLoadAll(t *testing.T) {
	names := Names()
	if len(names) == 0 {
		t.Fatalf("no models embedded")
	}
	for _, name := range names {
		r, err := Load(name)
		if err != nil {
			t.Errorf("Load(%q): %v", name, err)
			continue
		}
		if len(r.Tris) == 0 {
			t.Errorf("Load(%q): no triangles", name)
		}
	}
}
