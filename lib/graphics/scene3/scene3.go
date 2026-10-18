package scene3

import (
	"math"

	"github.com/ahmedtd/gogames/lib/graphics/renderable3"
	"github.com/ahmedtd/gogames/lib/vmath/fmat44"
	"github.com/ahmedtd/gogames/lib/vmath/fvec2"
	"github.com/ahmedtd/gogames/lib/vmath/fvec3"
	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
	"github.com/barnex/fmath"
)

type RenderTarget struct {
	Width, Height int
	ColorBuf      []float32
	DepthBuf      []float32
}

func MakeRenderTarget(width, height int) *RenderTarget {
	return &RenderTarget{
		Width:    width,
		Height:   height,
		ColorBuf: make([]float32, width*height*4),
		DepthBuf: make([]float32, width*height),
	}
}

func (rt *RenderTarget) Clear() {
	for i := 0; i < rt.Width*rt.Height*4; i += 4 {
		rt.ColorBuf[i+0] = 0.0
		rt.ColorBuf[i+1] = 0.0
		rt.ColorBuf[i+2] = 0.0
		rt.ColorBuf[i+3] = 1.0
	}

	minf := float32(math.Inf(-1))
	for i := range rt.Width * rt.Height {
		rt.DepthBuf[i] = float32(minf)
	}
}

func (rt *RenderTarget) QuantizeColor(buf []byte) {
	// Simple automatic exposure control.  Divide all pixels by the maximum RGB
	// power if it's greater than 1.0.
	maxMag := float32(0)
	for j := 0; j < rt.Width*rt.Height*4; j += 4 {
		thisMag := float32(0)
		thisMag += rt.ColorBuf[j+0] * rt.ColorBuf[j+0]
		thisMag += rt.ColorBuf[j+1] * rt.ColorBuf[j+1]
		thisMag += rt.ColorBuf[j+2] * rt.ColorBuf[j+2]
		if thisMag > maxMag {
			maxMag = thisMag
		}
	}
	if maxMag < float32(1) {
		maxMag = 1
	}
	exposureDivisor := 1 / fmath.Sqrt(maxMag)

	for j := 0; j < rt.Width*rt.Height*4; j += 4 {
		r := rt.ColorBuf[j+0] * exposureDivisor
		g := rt.ColorBuf[j+1] * exposureDivisor
		b := rt.ColorBuf[j+2] * exposureDivisor

		buf[j+0] = float32ToByte(r)
		buf[j+1] = float32ToByte(g)
		buf[j+2] = float32ToByte(b)
		buf[j+3] = float32ToByte(1)
	}
}

func float32ToByte(pow float32) byte {
	if pow >= 1 {
		return 255
	} else if pow < 0 {
		return 0
	} else {
		return byte(pow * 256)
	}
}

type Triangle struct {
	Vs            [3]fvec3.T
	TextureCoords [3]fvec2.T

	Diffuse *renderable3.UncompressedImage
}

func (tri *Triangle) Render(camMatrix fmat44.T, target *RenderTarget) {
	w := float32(target.Width)
	h := float32(target.Height)

	// Compute normalized device coordinates for the triangle.
	var ndcs [3]fvec4.T
	for i := range 3 {
		ndcs[i] = fmat44.MulMV(&camMatrix, fvec4.T{tri.Vs[i][0], tri.Vs[i][1], tri.Vs[i][2], 1.0})
		ndcs[i][3] = 1
	}

	var viewportCoords [3]fvec4.T
	for i := range 3 {
		viewportCoords[i][0] = ndcs[i][0]*w/2 + w/2
		viewportCoords[i][1] = ndcs[i][1]*h/2 + h/2
		viewportCoords[i][2] = ndcs[i][2]
	}

	// Note: Cannot just follow the guide here.  We need to do some fancy stuff
	// to make sure that interpolation of texture coordinates in screen space is
	// perspective-correct. See
	// https://www.cs.cornell.edu/~srm/fcg3/updated-11.3.1.pdf

	totalArea := signedTriangleArea(
		viewportCoords[0][0], viewportCoords[0][1],
		viewportCoords[1][0], viewportCoords[1][1],
		viewportCoords[2][0], viewportCoords[2][1],
	)

	if totalArea <= 0 {
		return
	}

	// Compute a screen-coordinate bounding box
	minX := viewportCoords[0][0]
	minY := viewportCoords[0][1]
	maxX := viewportCoords[0][0]
	maxY := viewportCoords[0][1]
	for i := 1; i < 3; i++ {
		if viewportCoords[i][0] < minX {
			minX = viewportCoords[i][0]
		}
		if viewportCoords[i][0] > maxX {
			maxX = viewportCoords[i][0]
		}
		if viewportCoords[i][1] < minY {
			minY = viewportCoords[i][1]
		}
		if viewportCoords[i][1] > maxY {
			maxY = viewportCoords[i][1]
		}
	}

	// Clip bounding box to the viewport.
	if minX < 0 {
		minX = 0
	}
	if maxX >= w {
		maxX = w - 1
	}
	if minY < 0 {
		minY = 0
	}
	if maxY >= h {
		maxY = h - 1
	}

	// Grow the bounding box to pixel boundaries
	minX = fmath.Floor(minX)
	maxX = fmath.Ceil(maxX)
	minY = fmath.Floor(minY)
	maxY = fmath.Ceil(maxY)

	for x := int(minX); x < int(maxX); x++ {
		for y := int(minY); y < int(maxY); y++ {
			fragCenter := fvec4.T{float32(x) + 0.0, float32(y) + 0.0, 0, 1}

			b0 := signedTriangleArea(
				fragCenter[0], fragCenter[1],
				viewportCoords[1][0], viewportCoords[1][1],
				viewportCoords[2][0], viewportCoords[2][1],
			) / totalArea
			b1 := signedTriangleArea(
				fragCenter[0], fragCenter[1],
				viewportCoords[2][0], viewportCoords[2][1],
				viewportCoords[0][0], viewportCoords[0][1],
			) / totalArea
			b2 := signedTriangleArea(
				fragCenter[0], fragCenter[1],
				viewportCoords[0][0], viewportCoords[0][1],
				viewportCoords[1][0], viewportCoords[1][1],
			) / totalArea
			if b0 < 0 || b1 < 0 || b2 < 0 {
				continue
			}

			// Depth test
			fragDepth := b0*viewportCoords[0][2] + b1*viewportCoords[1][2] + b2*viewportCoords[2][2]
			if fragDepth <= target.DepthBuf[y*target.Width+x] {
				continue
			}
			target.DepthBuf[y*target.Width+x] = fragDepth

			textureU := int(b0*tri.TextureCoords[0][0] + b1*tri.TextureCoords[1][0] + b2*tri.TextureCoords[2][0])
			textureV := int(b0*tri.TextureCoords[0][1] + b1*tri.TextureCoords[1][1] + b2*tri.TextureCoords[2][1])
			fragr := tri.Diffuse.Samples[textureV*tri.Diffuse.Width*4+textureU*4+0]
			fragg := tri.Diffuse.Samples[textureV*tri.Diffuse.Width*4+textureU*4+1]
			fragb := tri.Diffuse.Samples[textureV*tri.Diffuse.Width*4+textureU*4+2]
			// fraga := tri.Diffuse.Samples[textureV*tri.Diffuse.Width*4+textureU*4+3]

			target.ColorBuf[y*target.Width*4+x*4+0] = fragr
			target.ColorBuf[y*target.Width*4+x*4+1] = fragg
			target.ColorBuf[y*target.Width*4+x*4+2] = fragb
			target.ColorBuf[y*target.Width*4+x*4+3] = 1
		}
	}

}

func signedTriangleArea(a0, a1, b0, b1, c0, c1 float32) float32 {
	ab0 := b0 - a0
	ac1 := c1 - a1
	ab1 := b1 - a1
	ac0 := c0 - a0
	return 0.5 * (ab0*ac1 - ab1*ac0)
}

type MeshInstance struct {
	Mesh         *renderable3.T
	ModelToWorld fmat44.T
}

func (m *MeshInstance) Render(camMatrix fmat44.T, target *RenderTarget) {
	for _, tri := range m.Mesh.Tris {
		concreteTri := Triangle{
			Diffuse: m.Mesh.Diffuse,
		}
		for i := range 3 {
			ind := tri.VertexIndices[i]
			concreteTri.Vs[i] = m.Mesh.Vertices[ind]

			textureInd := tri.TextureIndices[i]
			concreteTri.TextureCoords[i] = m.Mesh.TextureCoords[textureInd]
			concreteTri.TextureCoords[i][0] *= float32(concreteTri.Diffuse.Width)
			concreteTri.TextureCoords[i][1] = 1 - concreteTri.TextureCoords[i][1]
			concreteTri.TextureCoords[i][1] *= float32(concreteTri.Diffuse.Height)
		}

		concreteTri.Render(camMatrix, target)
	}
}

type Scene struct {
	MeshInstances []*MeshInstance
}

func (s *Scene) Render(camMatrix fmat44.T, target *RenderTarget) {
	for _, mi := range s.MeshInstances {
		mi.Render(camMatrix, target)
	}
}
