// Package gltfloader loads triangle meshes from GLB (binary glTF 2.0) files.
//
// A GLB file is a 12-byte header followed by a JSON chunk and an optional BIN
// chunk.  The JSON describes a scene graph of nodes; nodes reference meshes,
// meshes are made of primitives, and primitive attributes (positions, normals,
// ...) and indices reference accessors.  An accessor describes how to read a
// typed array (e.g. VEC3 of float32) out of a bufferView, which is a byte
// range of a buffer.  In a GLB, buffer 0 is the BIN chunk.
//
// Only the subset needed for static geometry is supported: the default scene
// is flattened into a single mesh in its rest pose, with each node's transform
// baked into the vertices.  Materials, textures, skins, morph targets, and
// animations are ignored.
package gltfloader

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"

	"github.com/ahmedtd/gogames/lib/vmath/fvec2"
	"github.com/ahmedtd/gogames/lib/vmath/fvec3"
)

// Mesh is a flattened triangle mesh.  Unlike OBJ, glTF uses a single index
// for all of a vertex's attributes, so Normals and TextureCoords (when
// present) are parallel to Vertices.
type Mesh struct {
	Vertices      []fvec3.T
	Normals       []fvec3.T
	TextureCoords []fvec2.T
	Tris          [][3]int
}

const (
	glbMagic      = 0x46546c67 // "glTF"
	chunkTypeJSON = 0x4e4f534a // "JSON"
	chunkTypeBIN  = 0x004e4942 // "BIN\0"
)

const (
	componentTypeUnsignedByte  = 5121
	componentTypeUnsignedShort = 5123
	componentTypeUnsignedInt   = 5125
	componentTypeFloat         = 5126
)

const primitiveModeTriangles = 4

type document struct {
	ExtensionsRequired []string     `json:"extensionsRequired"`
	Scene              *int         `json:"scene"`
	Scenes             []scene      `json:"scenes"`
	Nodes              []node       `json:"nodes"`
	Meshes             []mesh       `json:"meshes"`
	Accessors          []accessor   `json:"accessors"`
	BufferViews        []bufferView `json:"bufferViews"`
	Buffers            []buffer     `json:"buffers"`
}

type scene struct {
	Nodes []int `json:"nodes"`
}

type node struct {
	Children    []int        `json:"children"`
	Mesh        *int         `json:"mesh"`
	Matrix      *[16]float64 `json:"matrix"`
	Translation *[3]float64  `json:"translation"`
	Rotation    *[4]float64  `json:"rotation"`
	Scale       *[3]float64  `json:"scale"`
}

type mesh struct {
	Primitives []primitive `json:"primitives"`
}

type primitive struct {
	Attributes map[string]int `json:"attributes"`
	Indices    *int           `json:"indices"`
	Mode       *int           `json:"mode"`
}

type accessor struct {
	BufferView    *int            `json:"bufferView"`
	ByteOffset    int             `json:"byteOffset"`
	ComponentType int             `json:"componentType"`
	Normalized    bool            `json:"normalized"`
	Count         int             `json:"count"`
	Type          string          `json:"type"`
	Sparse        json.RawMessage `json:"sparse"`
}

type bufferView struct {
	Buffer     int `json:"buffer"`
	ByteOffset int `json:"byteOffset"`
	ByteLength int `json:"byteLength"`
	ByteStride int `json:"byteStride"`
}

type buffer struct {
	URI        string `json:"uri"`
	ByteLength int    `json:"byteLength"`
}

// ParseGLB parses a GLB file into a single flattened mesh.
//
// glTF is Y-up.  If yUpToZUp is set, the mesh is rotated 90 degrees about the
// X axis so that +Y becomes +Z, mapping (x, y, z) to (x, -z, y).
//
// Texture coordinates are converted from glTF convention (v = 0 is the top row
// of the image) to OBJ convention (v = 0 is the bottom row).
func ParseGLB(glb []byte, yUpToZUp bool) (*Mesh, error) {
	jsonBytes, bin, err := splitGLB(glb)
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(bytes.NewReader(jsonBytes))
	doc := &document{}
	if err := dec.Decode(doc); err != nil {
		return nil, fmt.Errorf("while decoding JSON chunk: %w", err)
	}

	if len(doc.ExtensionsRequired) != 0 {
		return nil, fmt.Errorf("unsupported required extensions %v", doc.ExtensionsRequired)
	}

	p := &parser{doc: doc, bin: bin, out: &Mesh{}}

	sceneIndex := 0
	if doc.Scene != nil {
		sceneIndex = *doc.Scene
	}
	if sceneIndex < 0 || sceneIndex >= len(doc.Scenes) {
		return nil, fmt.Errorf("scene %d out of range", sceneIndex)
	}

	root := identityAffine()
	if yUpToZUp {
		root = affine{
			1, 0, 0, 0,
			0, 0, -1, 0,
			0, 1, 0, 0,
		}
	}
	for _, n := range doc.Scenes[sceneIndex].Nodes {
		if err := p.walkNode(n, root, 0); err != nil {
			return nil, err
		}
	}

	if p.sawTexCoords && p.sawNoTexCoords {
		return nil, fmt.Errorf("some primitives have TEXCOORD_0 and some don't")
	}

	return p.out, nil
}

func splitGLB(glb []byte) (jsonBytes, bin []byte, err error) {
	if len(glb) < 12 {
		return nil, nil, fmt.Errorf("too short for GLB header")
	}
	if magic := binary.LittleEndian.Uint32(glb[0:4]); magic != glbMagic {
		return nil, nil, fmt.Errorf("bad GLB magic %#x", magic)
	}
	if version := binary.LittleEndian.Uint32(glb[4:8]); version != 2 {
		return nil, nil, fmt.Errorf("unsupported GLB version %d", version)
	}
	length := binary.LittleEndian.Uint32(glb[8:12])
	if uint64(length) > uint64(len(glb)) {
		return nil, nil, fmt.Errorf("GLB header length %d exceeds data length %d", length, len(glb))
	}

	rest := glb[12:length]
	for i := 0; len(rest) > 0; i++ {
		if len(rest) < 8 {
			return nil, nil, fmt.Errorf("truncated chunk header")
		}
		chunkLength := binary.LittleEndian.Uint32(rest[0:4])
		chunkType := binary.LittleEndian.Uint32(rest[4:8])
		rest = rest[8:]
		if uint64(chunkLength) > uint64(len(rest)) {
			return nil, nil, fmt.Errorf("chunk %d length %d exceeds remaining data", i, chunkLength)
		}
		data := rest[:chunkLength]
		rest = rest[chunkLength:]

		switch {
		case i == 0 && chunkType == chunkTypeJSON:
			jsonBytes = data
		case i == 0:
			return nil, nil, fmt.Errorf("first chunk has type %#x, want JSON", chunkType)
		case i == 1 && chunkType == chunkTypeBIN:
			bin = data
		default:
			// Unknown chunks must be ignored.
		}
	}
	if jsonBytes == nil {
		return nil, nil, fmt.Errorf("no JSON chunk")
	}
	return jsonBytes, bin, nil
}

// maxNodeDepth bounds recursion through the node hierarchy, which glTF requires
// to be a forest but which a malformed file could make cyclic.
const maxNodeDepth = 256

type parser struct {
	doc *document
	bin []byte
	out *Mesh

	sawTexCoords, sawNoTexCoords bool
}

func (p *parser) walkNode(nodeIndex int, parent affine, depth int) error {
	if depth > maxNodeDepth {
		return fmt.Errorf("node hierarchy deeper than %d; cycle?", maxNodeDepth)
	}
	if nodeIndex < 0 || nodeIndex >= len(p.doc.Nodes) {
		return fmt.Errorf("node %d out of range", nodeIndex)
	}
	n := &p.doc.Nodes[nodeIndex]

	xform := mulAffine(parent, n.localTransform())

	if n.Mesh != nil {
		if *n.Mesh < 0 || *n.Mesh >= len(p.doc.Meshes) {
			return fmt.Errorf("node %d: mesh %d out of range", nodeIndex, *n.Mesh)
		}
		for i := range p.doc.Meshes[*n.Mesh].Primitives {
			if err := p.addPrimitive(&p.doc.Meshes[*n.Mesh].Primitives[i], xform); err != nil {
				return fmt.Errorf("node %d, mesh %d, primitive %d: %w", nodeIndex, *n.Mesh, i, err)
			}
		}
	}

	for _, c := range n.Children {
		if err := p.walkNode(c, xform, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func (p *parser) addPrimitive(prim *primitive, xform affine) error {
	if prim.Mode != nil && *prim.Mode != primitiveModeTriangles {
		return fmt.Errorf("unsupported primitive mode %d", *prim.Mode)
	}

	posIndex, ok := prim.Attributes["POSITION"]
	if !ok {
		return fmt.Errorf("no POSITION attribute")
	}
	positions, err := p.readFloats(posIndex, "VEC3")
	if err != nil {
		return fmt.Errorf("while reading POSITION: %w", err)
	}
	vertexCount := len(positions) / 3

	normIndex, ok := prim.Attributes["NORMAL"]
	if !ok {
		return fmt.Errorf("no NORMAL attribute")
	}
	normals, err := p.readFloats(normIndex, "VEC3")
	if err != nil {
		return fmt.Errorf("while reading NORMAL: %w", err)
	}
	if len(normals)/3 != vertexCount {
		return fmt.Errorf("NORMAL count %d != POSITION count %d", len(normals)/3, vertexCount)
	}

	var texCoords []float32
	if tcIndex, ok := prim.Attributes["TEXCOORD_0"]; ok {
		p.sawTexCoords = true
		texCoords, err = p.readFloats(tcIndex, "VEC2")
		if err != nil {
			return fmt.Errorf("while reading TEXCOORD_0: %w", err)
		}
		if len(texCoords)/2 != vertexCount {
			return fmt.Errorf("TEXCOORD_0 count %d != POSITION count %d", len(texCoords)/2, vertexCount)
		}
	} else {
		p.sawNoTexCoords = true
	}

	var indices []int
	if prim.Indices != nil {
		indices, err = p.readIndices(*prim.Indices)
		if err != nil {
			return fmt.Errorf("while reading indices: %w", err)
		}
	} else {
		indices = make([]int, vertexCount)
		for i := range indices {
			indices[i] = i
		}
	}
	if len(indices)%3 != 0 {
		return fmt.Errorf("index count %d is not a multiple of 3", len(indices))
	}

	normalXform := xform.normalMatrix()
	base := len(p.out.Vertices)
	for i := 0; i < vertexCount; i++ {
		p.out.Vertices = append(p.out.Vertices, xform.transformPoint(fvec3.T{positions[3*i], positions[3*i+1], positions[3*i+2]}))
		p.out.Normals = append(p.out.Normals, fvec3.Normalize(normalXform.transformVector(fvec3.T{normals[3*i], normals[3*i+1], normals[3*i+2]})))
		if texCoords != nil {
			p.out.TextureCoords = append(p.out.TextureCoords, fvec2.T{texCoords[2*i], 1 - texCoords[2*i+1]})
		}
	}

	// A mirroring transform flips the winding order, so swap two vertices to
	// keep front faces counter-clockwise.
	mirrored := xform.det() < 0
	for i := 0; i < len(indices); i += 3 {
		var tri [3]int
		for j := 0; j < 3; j++ {
			if indices[i+j] >= vertexCount {
				return fmt.Errorf("index %d out of range for %d vertices", indices[i+j], vertexCount)
			}
			tri[j] = base + indices[i+j]
		}
		if mirrored {
			tri[1], tri[2] = tri[2], tri[1]
		}
		p.out.Tris = append(p.out.Tris, tri)
	}

	return nil
}

var componentsPerType = map[string]int{
	"SCALAR": 1,
	"VEC2":   2,
	"VEC3":   3,
	"VEC4":   4,
	"MAT2":   4,
	"MAT3":   9,
	"MAT4":   16,
}

var bytesPerComponent = map[int]int{
	5120:                       1, // BYTE
	componentTypeUnsignedByte:  1,
	5122:                       2, // SHORT
	componentTypeUnsignedShort: 2,
	componentTypeUnsignedInt:   4,
	componentTypeFloat:         4,
}

// accessorElements returns the backing bytes for an accessor, along with the
// distance between consecutive elements and the size of a single element.
func (p *parser) accessorElements(accessorIndex int) (*accessor, []byte, int, int, error) {
	if accessorIndex < 0 || accessorIndex >= len(p.doc.Accessors) {
		return nil, nil, 0, 0, fmt.Errorf("accessor %d out of range", accessorIndex)
	}
	acc := &p.doc.Accessors[accessorIndex]
	if len(acc.Sparse) != 0 {
		return nil, nil, 0, 0, fmt.Errorf("sparse accessors are unsupported")
	}
	if acc.BufferView == nil {
		return nil, nil, 0, 0, fmt.Errorf("accessors without a bufferView are unsupported")
	}
	numComponents, ok := componentsPerType[acc.Type]
	if !ok {
		return nil, nil, 0, 0, fmt.Errorf("unknown accessor type %q", acc.Type)
	}
	componentSize, ok := bytesPerComponent[acc.ComponentType]
	if !ok {
		return nil, nil, 0, 0, fmt.Errorf("unknown component type %d", acc.ComponentType)
	}
	elemSize := numComponents * componentSize

	if *acc.BufferView < 0 || *acc.BufferView >= len(p.doc.BufferViews) {
		return nil, nil, 0, 0, fmt.Errorf("bufferView %d out of range", *acc.BufferView)
	}
	bv := &p.doc.BufferViews[*acc.BufferView]

	if bv.Buffer != 0 || len(p.doc.Buffers) == 0 || p.doc.Buffers[0].URI != "" {
		return nil, nil, 0, 0, fmt.Errorf("only the GLB-embedded buffer is supported")
	}
	if bv.ByteOffset < 0 || bv.ByteLength < 0 || bv.ByteOffset+bv.ByteLength > len(p.bin) {
		return nil, nil, 0, 0, fmt.Errorf("bufferView %d range [%d, +%d) exceeds BIN chunk length %d", *acc.BufferView, bv.ByteOffset, bv.ByteLength, len(p.bin))
	}
	view := p.bin[bv.ByteOffset : bv.ByteOffset+bv.ByteLength]

	stride := bv.ByteStride
	if stride == 0 {
		stride = elemSize
	}
	if acc.Count > 0 {
		end := acc.ByteOffset + (acc.Count-1)*stride + elemSize
		if acc.ByteOffset < 0 || end > len(view) {
			return nil, nil, 0, 0, fmt.Errorf("accessor %d reads past end of bufferView %d", accessorIndex, *acc.BufferView)
		}
	}
	return acc, view[acc.ByteOffset:], stride, elemSize, nil
}

// readFloats reads a float32 accessor of the given type as a flat slice.
func (p *parser) readFloats(accessorIndex int, wantType string) ([]float32, error) {
	acc, data, stride, _, err := p.accessorElements(accessorIndex)
	if err != nil {
		return nil, err
	}
	if acc.Type != wantType {
		return nil, fmt.Errorf("accessor type is %q, want %q", acc.Type, wantType)
	}
	if acc.ComponentType != componentTypeFloat {
		return nil, fmt.Errorf("component type %d unsupported, want float", acc.ComponentType)
	}

	n := componentsPerType[acc.Type]
	out := make([]float32, acc.Count*n)
	for i := 0; i < acc.Count; i++ {
		elem := data[i*stride:]
		for c := 0; c < n; c++ {
			out[i*n+c] = math.Float32frombits(binary.LittleEndian.Uint32(elem[4*c:]))
		}
	}
	return out, nil
}

// readIndices reads an unsigned integer SCALAR accessor.
func (p *parser) readIndices(accessorIndex int) ([]int, error) {
	acc, data, stride, _, err := p.accessorElements(accessorIndex)
	if err != nil {
		return nil, err
	}
	if acc.Type != "SCALAR" {
		return nil, fmt.Errorf("index accessor type is %q, want SCALAR", acc.Type)
	}

	out := make([]int, acc.Count)
	for i := range out {
		elem := data[i*stride:]
		switch acc.ComponentType {
		case componentTypeUnsignedByte:
			out[i] = int(elem[0])
		case componentTypeUnsignedShort:
			out[i] = int(binary.LittleEndian.Uint16(elem))
		case componentTypeUnsignedInt:
			out[i] = int(binary.LittleEndian.Uint32(elem))
		default:
			return nil, fmt.Errorf("index component type %d unsupported", acc.ComponentType)
		}
	}
	return out, nil
}

// affine is a 3x4 row-major affine transform; the last column is the
// translation.  Transforms are composed in float64 since node hierarchies can
// be deep.
type affine [12]float64

func identityAffine() affine {
	return affine{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
	}
}

// mulAffine returns a*b, which applies b first.
func mulAffine(a, b affine) affine {
	var out affine
	for r := 0; r < 3; r++ {
		for c := 0; c < 4; c++ {
			v := a[r*4+0]*b[0*4+c] + a[r*4+1]*b[1*4+c] + a[r*4+2]*b[2*4+c]
			if c == 3 {
				v += a[r*4+3]
			}
			out[r*4+c] = v
		}
	}
	return out
}

func (a affine) transformPoint(v fvec3.T) fvec3.T {
	x, y, z := float64(v[0]), float64(v[1]), float64(v[2])
	return fvec3.T{
		float32(a[0]*x + a[1]*y + a[2]*z + a[3]),
		float32(a[4]*x + a[5]*y + a[6]*z + a[7]),
		float32(a[8]*x + a[9]*y + a[10]*z + a[11]),
	}
}

func (a affine) transformVector(v fvec3.T) fvec3.T {
	x, y, z := float64(v[0]), float64(v[1]), float64(v[2])
	return fvec3.T{
		float32(a[0]*x + a[1]*y + a[2]*z),
		float32(a[4]*x + a[5]*y + a[6]*z),
		float32(a[8]*x + a[9]*y + a[10]*z),
	}
}

func (a affine) det() float64 {
	return a[0]*(a[5]*a[10]-a[6]*a[9]) -
		a[1]*(a[4]*a[10]-a[6]*a[8]) +
		a[2]*(a[4]*a[9]-a[5]*a[8])
}

// normalMatrix returns the cofactor matrix of a's linear part, which is the
// inverse transpose scaled by the determinant.  Normals transformed by it
// need renormalizing, and are flipped when det < 0 to stay outward-facing.
func (a affine) normalMatrix() affine {
	// Columns of the cofactor matrix are cross products of pairs of columns
	// of the linear part.
	c0 := [3]float64{a[0], a[4], a[8]}
	c1 := [3]float64{a[1], a[5], a[9]}
	c2 := [3]float64{a[2], a[6], a[10]}
	k0 := cross(c1, c2)
	k1 := cross(c2, c0)
	k2 := cross(c0, c1)
	out := affine{
		k0[0], k1[0], k2[0], 0,
		k0[1], k1[1], k2[1], 0,
		k0[2], k1[2], k2[2], 0,
	}
	if a.det() < 0 {
		for i := range out {
			out[i] = -out[i]
		}
	}
	return out
}

func cross(a, b [3]float64) [3]float64 {
	return [3]float64{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}

// localTransform returns the node's transform relative to its parent.
func (n *node) localTransform() affine {
	if n.Matrix != nil {
		// glTF matrices are column-major 4x4.
		m := n.Matrix
		return affine{
			m[0], m[4], m[8], m[12],
			m[1], m[5], m[9], m[13],
			m[2], m[6], m[10], m[14],
		}
	}

	// T * R * S.
	out := identityAffine()
	if n.Rotation != nil {
		x, y, z, w := n.Rotation[0], n.Rotation[1], n.Rotation[2], n.Rotation[3]
		out = affine{
			1 - 2*(y*y+z*z), 2 * (x*y - z*w), 2 * (x*z + y*w), 0,
			2 * (x*y + z*w), 1 - 2*(x*x+z*z), 2 * (y*z - x*w), 0,
			2 * (x*z - y*w), 2 * (y*z + x*w), 1 - 2*(x*x+y*y), 0,
		}
	}
	if n.Scale != nil {
		for r := 0; r < 3; r++ {
			for c := 0; c < 3; c++ {
				out[r*4+c] *= n.Scale[c]
			}
		}
	}
	if n.Translation != nil {
		out[3], out[7], out[11] = n.Translation[0], n.Translation[1], n.Translation[2]
	}
	return out
}
