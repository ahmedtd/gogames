package objloader

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/ahmedtd/gogames/lib/vmath/fvec2"
	"github.com/ahmedtd/gogames/lib/vmath/fvec3"
)

type Tri struct {
	VertexIndices  [3]int
	TextureIndices [3]int
	NormalIndices  [3]int
	Groups         []string
}

type Mesh struct {
	Material      string
	Vertices      []fvec3.T
	TextureCoords []fvec2.T
	Normals       []fvec3.T
	Tris          []Tri
}

func Parse(objBytes []byte) (*Mesh, error) {

	mesh := &Mesh{}
	curGroups := []string{"default"}

	for lineBytes := range bytes.Lines(objBytes) {
		line := string(lineBytes)

		if line[0] == '#' {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		switch parts[0] {
		case "mtllib":
			// Unimplemented.
		case "g":
			if len(parts) < 2 {
				return nil, fmt.Errorf("need at least 1 entry in g")
			}

			curGroups = []string{}
			for _, newGroupName := range parts[1:] {
				curGroups = append(curGroups, newGroupName)
			}

		case "v":
			if len(parts) < 4 {
				return nil, fmt.Errorf("need at least 3 entries in vertex")
			}

			x, err := strconv.ParseFloat(parts[1], 32)
			if err != nil {
				return nil, fmt.Errorf("while parsing v: failed to parse x: %w", err)
			}
			y, err := strconv.ParseFloat(parts[2], 32)
			if err != nil {
				return nil, fmt.Errorf("while parsing v: failed to parse y: %w", err)
			}
			z, err := strconv.ParseFloat(parts[3], 32)
			if err != nil {
				return nil, fmt.Errorf("while parsing v: failed to parse z: %w", err)
			}
			mesh.Vertices = append(mesh.Vertices, fvec3.T{float32(x), float32(y), float32(z)})
		case "vn":
			if len(parts) < 4 {
				return nil, fmt.Errorf("need at least 3 entries in normal")
			}

			x, err := strconv.ParseFloat(parts[1], 32)
			if err != nil {
				return nil, fmt.Errorf("while parsing vn: failed to parse x: %w", err)
			}
			y, err := strconv.ParseFloat(parts[2], 32)
			if err != nil {
				return nil, fmt.Errorf("while parsing vn: failed to parse y: %w", err)
			}
			z, err := strconv.ParseFloat(parts[3], 32)
			if err != nil {
				return nil, fmt.Errorf("while parsing vn: failed to parse z: %w", err)
			}
			mesh.Normals = append(mesh.Normals, fvec3.T{float32(x), float32(y), float32(z)})
		case "vt":
			if len(parts) < 3 {
				return nil, fmt.Errorf("need at least 2 entries in texture coord")
			}

			u, err := strconv.ParseFloat(parts[1], 32)
			if err != nil {
				return nil, fmt.Errorf("while parsing vt: failed to parse x: %w", err)
			}
			v, err := strconv.ParseFloat(parts[2], 32)
			if err != nil {
				return nil, fmt.Errorf("while parsing vt: failed to parse y: %w", err)
			}
			mesh.TextureCoords = append(mesh.TextureCoords, fvec2.T{float32(u), float32(v)})
		case "f":
			if len(parts) == 5 {
				return nil, fmt.Errorf("quads unsupported")
			} else if len(parts) != 4 {
				return nil, fmt.Errorf("need 3 entries in face def")
			}

			tri := Tri{
				Groups: curGroups[:],
			}
			var err error
			for i := 0; i < 3; i++ {
				tri.VertexIndices[i], tri.TextureIndices[i], tri.NormalIndices[i], err = splitFaceEntry(parts[i+1])
				if err != nil {
					return nil, fmt.Errorf("while splitting face entry: %w", err)
				}
			}
			mesh.Tris = append(mesh.Tris, tri)
		case "usemtl":
			// TODO: Does usemtl interact with groups?
			if len(parts) != 2 {
				return nil, fmt.Errorf("need name in usemtl")
			}
			mesh.Material = parts[1]
		default:
			return nil, fmt.Errorf("unsupported type %q", parts[0])
		}

	}

	return mesh, nil

}

func splitFaceEntry(data string) (int, int, int, error) {
	parts := strings.Split(data, "/")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("wrong number of indices")
	}

	vInd, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("while parsing v index: %w", err)
	}

	tInd, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("while parsing t index: %w", err)
	}

	nInd, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("while parsing n index: %w", err)
	}

	return vInd - 1, tInd - 1, nInd - 1, nil
}
