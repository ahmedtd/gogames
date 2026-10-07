package gltfloader

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/ahmedtd/gogames/lib/vmath/fvec2"
	"github.com/ahmedtd/gogames/lib/vmath/fvec3"
)

// buildGLB assembles a GLB from a JSON document and a BIN chunk.  If the
// document has buffers but no byteLength set, it's filled in.
func buildGLB(t *testing.T, doc map[string]any, bin []byte) []byte {
	t.Helper()
	jsonBytes, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("while marshaling JSON: %v", err)
	}
	for len(jsonBytes)%4 != 0 {
		jsonBytes = append(jsonBytes, ' ')
	}
	for len(bin)%4 != 0 {
		bin = append(bin, 0)
	}

	var out bytes.Buffer
	binary.Write(&out, binary.LittleEndian, uint32(glbMagic))
	binary.Write(&out, binary.LittleEndian, uint32(2))
	binary.Write(&out, binary.LittleEndian, uint32(12+8+len(jsonBytes)+8+len(bin)))
	binary.Write(&out, binary.LittleEndian, uint32(len(jsonBytes)))
	binary.Write(&out, binary.LittleEndian, uint32(chunkTypeJSON))
	out.Write(jsonBytes)
	binary.Write(&out, binary.LittleEndian, uint32(len(bin)))
	binary.Write(&out, binary.LittleEndian, uint32(chunkTypeBIN))
	out.Write(bin)
	return out.Bytes()
}

func f32Bytes(fs ...float32) []byte {
	out := make([]byte, 4*len(fs))
	for i, f := range fs {
		binary.LittleEndian.PutUint32(out[4*i:], math.Float32bits(f))
	}
	return out
}

// triangleDoc returns a document with a single triangle in the XY plane,
// facing +Z, stored as interleaved POSITION/NORMAL/TEXCOORD_0 with u16
// indices.  The caller supplies the node list.
func triangleDoc(nodes []map[string]any, rootNodes []int) (map[string]any, []byte) {
	var bin []byte
	verts := [][8]float32{
		{0, 0, 0, 0, 0, 1, 0, 0},
		{1, 0, 0, 0, 0, 1, 1, 0},
		{0, 1, 0, 0, 0, 1, 0, 0.25},
	}
	for _, v := range verts {
		bin = append(bin, f32Bytes(v[:]...)...)
	}
	vertexBytes := len(bin)
	for _, i := range []uint16{0, 1, 2} {
		bin = binary.LittleEndian.AppendUint16(bin, i)
	}

	doc := map[string]any{
		"asset":  map[string]any{"version": "2.0"},
		"scene":  0,
		"scenes": []any{map[string]any{"nodes": rootNodes}},
		"nodes":  nodes,
		"meshes": []any{map[string]any{"primitives": []any{map[string]any{
			"attributes": map[string]any{"POSITION": 0, "NORMAL": 1, "TEXCOORD_0": 2},
			"indices":    3,
		}}}},
		"accessors": []any{
			map[string]any{"bufferView": 0, "byteOffset": 0, "componentType": componentTypeFloat, "count": 3, "type": "VEC3"},
			map[string]any{"bufferView": 0, "byteOffset": 12, "componentType": componentTypeFloat, "count": 3, "type": "VEC3"},
			map[string]any{"bufferView": 0, "byteOffset": 24, "componentType": componentTypeFloat, "count": 3, "type": "VEC2"},
			map[string]any{"bufferView": 1, "componentType": componentTypeUnsignedShort, "count": 3, "type": "SCALAR"},
		},
		"bufferViews": []any{
			map[string]any{"buffer": 0, "byteOffset": 0, "byteLength": vertexBytes, "byteStride": 32},
			map[string]any{"buffer": 0, "byteOffset": vertexBytes, "byteLength": 6},
		},
		"buffers": []any{map[string]any{"byteLength": len(bin)}},
	}
	return doc, bin
}

func approxVec3(a, b fvec3.T) bool {
	for i := range a {
		if math.Abs(float64(a[i]-b[i])) > 1e-5 {
			return false
		}
	}
	return true
}

// geometricNormal returns the unnormalized normal implied by a triangle's
// counter-clockwise winding.
func geometricNormal(m *Mesh, tri [3]int) fvec3.T {
	a, b, c := m.Vertices[tri[0]], m.Vertices[tri[1]], m.Vertices[tri[2]]
	return fvec3.CProd(fvec3.SubVV(b, a), fvec3.SubVV(c, a))
}

func TestParseGLB(t *testing.T) {
	testCases := []struct {
		name       string
		nodes      []map[string]any
		rootNodes  []int
		yUpToZUp   bool
		wantVerts  []fvec3.T
		wantNormal fvec3.T
	}{
		{
			name:       "identity",
			nodes:      []map[string]any{{"mesh": 0}},
			rootNodes:  []int{0},
			wantVerts:  []fvec3.T{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
			wantNormal: fvec3.T{0, 0, 1},
		},
		{
			name: "translation hierarchy",
			nodes: []map[string]any{
				{"children": []int{1}, "translation": []float64{1, 2, 3}},
				{"mesh": 0, "translation": []float64{10, 0, 0}},
			},
			rootNodes:  []int{0},
			wantVerts:  []fvec3.T{{11, 2, 3}, {12, 2, 3}, {11, 3, 3}},
			wantNormal: fvec3.T{0, 0, 1},
		},
		{
			name: "rotation then scale then translation",
			nodes: []map[string]any{{
				"mesh": 0,
				// 90 degrees about +Z.
				"rotation":    []float64{0, 0, math.Sqrt2 / 2, math.Sqrt2 / 2},
				"scale":       []float64{2, 3, 1},
				"translation": []float64{0, 0, 5},
			}},
			rootNodes: []int{0},
			// Scale takes (1,0,0) to (2,0,0) and (0,1,0) to (0,3,0); rotation
			// then takes those to (0,2,0) and (-3,0,0).
			wantVerts:  []fvec3.T{{0, 0, 5}, {0, 2, 5}, {-3, 0, 5}},
			wantNormal: fvec3.T{0, 0, 1},
		},
		{
			name: "column-major matrix",
			nodes: []map[string]any{{
				"mesh":   0,
				"matrix": []float64{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 7, 8, 9, 1},
			}},
			rootNodes:  []int{0},
			wantVerts:  []fvec3.T{{7, 8, 9}, {8, 8, 9}, {7, 9, 9}},
			wantNormal: fvec3.T{0, 0, 1},
		},
		{
			name:       "mirror keeps winding consistent with normal",
			nodes:      []map[string]any{{"mesh": 0, "scale": []float64{1, 1, -1}}},
			rootNodes:  []int{0},
			wantVerts:  []fvec3.T{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
			wantNormal: fvec3.T{0, 0, -1},
		},
		{
			name:       "y-up to z-up",
			nodes:      []map[string]any{{"mesh": 0}},
			rootNodes:  []int{0},
			yUpToZUp:   true,
			wantVerts:  []fvec3.T{{0, 0, 0}, {1, 0, 0}, {0, 0, 1}},
			wantNormal: fvec3.T{0, -1, 0},
		},
		{
			name:       "instanced twice",
			nodes:      []map[string]any{{"mesh": 0}, {"mesh": 0, "translation": []float64{0, 0, 1}}},
			rootNodes:  []int{0, 1},
			wantVerts:  []fvec3.T{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0, 0, 1}, {1, 0, 1}, {0, 1, 1}},
			wantNormal: fvec3.T{0, 0, 1},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			doc, bin := triangleDoc(tc.nodes, tc.rootNodes)
			m, err := ParseGLB(buildGLB(t, doc, bin), tc.yUpToZUp)
			if err != nil {
				t.Fatalf("ParseGLB: %v", err)
			}

			if len(m.Vertices) != len(tc.wantVerts) {
				t.Fatalf("got %d vertices, want %d", len(m.Vertices), len(tc.wantVerts))
			}
			for i := range m.Vertices {
				if !approxVec3(m.Vertices[i], tc.wantVerts[i]) {
					t.Errorf("vertex %d: got %v, want %v", i, m.Vertices[i], tc.wantVerts[i])
				}
			}

			for _, tri := range m.Tris {
				for _, vi := range tri {
					if !approxVec3(m.Normals[vi], tc.wantNormal) {
						t.Errorf("normal %d: got %v, want %v", vi, m.Normals[vi], tc.wantNormal)
					}
				}
				if g := geometricNormal(m, tri); fvec3.IProd(g, tc.wantNormal) <= 0 {
					t.Errorf("tri %v: winding normal %v disagrees with vertex normal %v", tri, g, tc.wantNormal)
				}
			}

			wantUVs := []fvec2.T{{0, 1}, {1, 1}, {0, 0.75}}
			for i := range m.TextureCoords {
				if m.TextureCoords[i] != wantUVs[i%3] {
					t.Errorf("uv %d: got %v, want %v (flipped to OBJ convention)", i, m.TextureCoords[i], wantUVs[i%3])
				}
			}
		})
	}
}

func TestParseGLBErrors(t *testing.T) {
	testCases := []struct {
		name    string
		mutate  func(doc map[string]any)
		wantErr string
	}{
		{
			name:    "non-triangle mode",
			mutate:  func(doc map[string]any) { primitiveOf(doc)["mode"] = 1 },
			wantErr: "unsupported primitive mode",
		},
		{
			name: "external buffer",
			mutate: func(doc map[string]any) {
				doc["buffers"].([]any)[0].(map[string]any)["uri"] = "foo.bin"
			},
			wantErr: "only the GLB-embedded buffer",
		},
		{
			name: "sparse accessor",
			mutate: func(doc map[string]any) {
				doc["accessors"].([]any)[0].(map[string]any)["sparse"] = map[string]any{"count": 1}
			},
			wantErr: "sparse accessors",
		},
		{
			name: "accessor past end of view",
			mutate: func(doc map[string]any) {
				doc["accessors"].([]any)[0].(map[string]any)["count"] = 4
			},
			wantErr: "reads past end",
		},
		{
			name: "index out of range",
			mutate: func(doc map[string]any) {
				doc["accessors"].([]any)[1].(map[string]any)["count"] = 2
				doc["accessors"].([]any)[2].(map[string]any)["count"] = 2
				doc["accessors"].([]any)[0].(map[string]any)["count"] = 2
			},
			wantErr: "out of range",
		},
		{
			name: "node cycle",
			mutate: func(doc map[string]any) {
				doc["nodes"] = []map[string]any{{"children": []int{0}}}
			},
			wantErr: "cycle",
		},
		{
			name:    "required extension",
			mutate:  func(doc map[string]any) { doc["extensionsRequired"] = []string{"KHR_draco_mesh_compression"} },
			wantErr: "unsupported required extensions",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			doc, bin := triangleDoc([]map[string]any{{"mesh": 0}}, []int{0})
			tc.mutate(doc)
			_, err := ParseGLB(buildGLB(t, doc, bin), false)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("got error %v, want one containing %q", err, tc.wantErr)
			}
		})
	}

	t.Run("bad magic", func(t *testing.T) {
		doc, bin := triangleDoc([]map[string]any{{"mesh": 0}}, []int{0})
		glb := buildGLB(t, doc, bin)
		glb[0] = 'x'
		if _, err := ParseGLB(glb, false); err == nil || !strings.Contains(err.Error(), "magic") {
			t.Errorf("got error %v, want bad magic", err)
		}
	})

	t.Run("truncated", func(t *testing.T) {
		doc, bin := triangleDoc([]map[string]any{{"mesh": 0}}, []int{0})
		glb := buildGLB(t, doc, bin)
		if _, err := ParseGLB(glb[:len(glb)-8], false); err == nil {
			t.Errorf("got nil error for truncated GLB")
		}
	})
}

func primitiveOf(doc map[string]any) map[string]any {
	return doc["meshes"].([]any)[0].(map[string]any)["primitives"].([]any)[0].(map[string]any)
}
