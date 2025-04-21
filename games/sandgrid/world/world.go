package world

import (
	"context"
	"log"
	"math"
	"sync"
	"time"

	"shellworld/lib/vmath/mat33"
	"shellworld/lib/vmath/vec2"
)

type RenderState struct {
	SimTime int64

	Viewport Viewport

	CursorWorldPosition vec2.T

	ResX, ResY int
	SandLayer  []int
}

type RenderPulse struct {
	RealNow time.Time

	ScreenWidthPixels  int
	ScreenHeightPixels int

	MouseButtonLeftDown   bool
	MouseButtonRightDown  bool
	CursorScreenPositionX int
	CursorScreenPositionY int

	Key1Pressed bool
	Key2Pressed bool
}

type Viewport struct {
	Center vec2.T
	Angle  float64

	PixelsPerMeter float64

	ScreenWidthPixels  int
	ScreenHeightPixels int
}

func (v *Viewport) WorldToScreenMatrix() mat33.T {
	worldToViewport := mat33.Translate2(vec2.T{-v.Center[0], -v.Center[1]})
	worldToViewport = mat33.MulMM(mat33.Rotate2(v.Angle), worldToViewport)
	worldToViewport = mat33.MulMM(mat33.Scale(v.PixelsPerMeter), worldToViewport)

	vpToScreen := mat33.Scale2(1.0, -1.0)
	vpToScreen = mat33.MulMM(mat33.Translate2(vec2.T{float64(v.ScreenWidthPixels) / 2.0, float64(v.ScreenHeightPixels) / 2.0}), vpToScreen)

	worldToScreen := mat33.MulMM(vpToScreen, worldToViewport)
	return worldToScreen
}

func (v *Viewport) WorldPositionToScreenPosition(worldPos vec2.T) vec2.T {
	return mat33.MulMV2(v.WorldToScreenMatrix(), worldPos)
}

func (v *Viewport) ScreenPositionToWorldPosition(screenPos vec2.T) vec2.T {
	screenToWorld := mat33.Inverse(v.WorldToScreenMatrix())
	worldPos := mat33.MulMV2(screenToWorld, screenPos)
	return worldPos
}

type World struct {
	shutdownChan chan struct{}

	// inbox contains RenderPulse messages from the renderer.
	inbox               chan *RenderPulse
	renderPulsePoolLock sync.Mutex
	renderPulsePool     []*RenderPulse

	// outbox contains RenderState messages to the renderer.
	outboxLock          sync.Mutex
	outbox              *RenderState
	renderStatePoolLock sync.Mutex
	renderStatePool     []*RenderState

	prevRealTime time.Time
	curRealTime  time.Time

	prevSimTime int64
	curSimTime  int64
	curSimDelta float64

	// Raw inputs.
	mouseButtonLeftDown   bool
	mouseButtonRightDown  bool
	cursorScreenPositionX int
	cursorScreenPositionY int

	key1Pressed bool
	key2Pressed bool

	viewport Viewport

	cursorWorldPosition vec2.T

	resX             int
	resY             int
	sandCurLayer     []int
	sandScratchLayer []int

	sandTickCounter int64
}

func New(gridResolutionX, gridResolutionY int) *World {
	w := &World{
		shutdownChan: make(chan struct{}),
		inbox:        make(chan *RenderPulse, 8),
		outbox:       nil,

		viewport: Viewport{
			Center:         vec2.T{0, -3},
			PixelsPerMeter: 50,
		},

		resX:             gridResolutionX,
		resY:             gridResolutionY,
		sandCurLayer:     make([]int, gridResolutionX*gridResolutionY),
		sandScratchLayer: make([]int, gridResolutionX*gridResolutionY),

		renderPulsePool: make([]*RenderPulse, 10),
		renderStatePool: make([]*RenderState, 10),
	}

	for i := 0; i < 10; i++ {
		w.renderPulsePool[i] = &RenderPulse{}
		w.renderStatePool[i] = &RenderState{}
	}

	return w
}

func (w *World) postRenderState(rs *RenderState) {
	w.outboxLock.Lock()
	oldRS := w.outbox
	w.outbox = rs
	w.outboxLock.Unlock()

	if oldRS != nil {
		w.ReleaseRenderState(rs)
	}
}

// PollOutbox pulls the next message from the outbox.
func (w *World) PollOutbox() *RenderState {
	w.outboxLock.Lock()
	defer w.outboxLock.Unlock()

	rs := w.outbox
	w.outbox = nil
	return rs
}

func (w *World) getRenderState() *RenderState {
	w.renderStatePoolLock.Lock()
	defer w.renderStatePoolLock.Unlock()

	for i := 0; i < len(w.renderStatePool); i++ {
		if w.renderStatePool[i] != nil {
			v := w.renderStatePool[i]
			w.renderStatePool[i] = nil
			return v
		}
	}

	return nil
}

// ReleaseRenderState releases a RenderState from the renderer back into the
// pool, so the world can pick it up and reuse it.
func (w *World) ReleaseRenderState(rs *RenderState) {
	w.renderStatePoolLock.Lock()
	defer w.renderStatePoolLock.Unlock()

	for i := 0; i < len(w.renderStatePool); i++ {
		if w.renderStatePool[i] == nil {
			w.renderStatePool[i] = rs
			return
		}
	}

	log.Printf("No spot found in renderStatePool.  Should be impossible.")
}

func (w *World) GetRenderPulse() *RenderPulse {
	w.renderPulsePoolLock.Lock()
	defer w.renderPulsePoolLock.Unlock()

	for i := 0; i < len(w.renderPulsePool); i++ {
		if w.renderPulsePool[i] != nil {
			v := w.renderPulsePool[i]
			w.renderPulsePool[i] = nil
			return v
		}
	}

	return nil
}

func (w *World) releaseRenderPulse(rp *RenderPulse) {
	w.renderPulsePoolLock.Lock()
	defer w.renderPulsePoolLock.Unlock()

	for i := 0; i < len(w.renderPulsePool); i++ {
		if w.renderPulsePool[i] == nil {
			w.renderPulsePool[i] = rp
			return
		}
	}

	log.Printf("No spot found in renderPulsePool.  Should be impossible.")
}

// Pulse puts a RenderPulse in the inbox.
//
// The game simulation is released and runs up to the real time specified in the
// RenderPulse.
//
// Returns false if the inbox is full, indicating that the renderer has run way
// ahead of the world simulation.
func (w *World) Pulse(rp *RenderPulse) bool {
	select {
	case w.inbox <- rp:
		return true
	default:
		return false
	}
}

func (w *World) Run(ctx context.Context) {
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case rp := <-w.inbox:
			rs := w.getRenderState()
			if rs == nil {
				log.Printf("No renderstate available")
				w.releaseRenderPulse(rp)
				continue
			}
			w.update(rp, rs)
			w.postRenderState(rs)
			w.releaseRenderPulse(rp)
		}
	}

	close(w.shutdownChan)
}

func (w *World) ShutdownChan() chan struct{} {
	return w.shutdownChan
}

func (w *World) update(rp *RenderPulse, rs *RenderState) {
	w.prevRealTime = w.curRealTime
	w.curRealTime = rp.RealNow

	w.prevSimTime = w.curSimTime
	w.curSimTime += 1000000000 / 60
	simDelta := w.curSimTime - w.prevSimTime
	w.curSimDelta = float64(simDelta/1e9) + float64(simDelta%1e9)/1e9

	w.mouseButtonLeftDown = rp.MouseButtonLeftDown
	w.mouseButtonRightDown = rp.MouseButtonRightDown
	w.cursorScreenPositionX = rp.CursorScreenPositionX
	w.cursorScreenPositionY = rp.CursorScreenPositionY

	w.key1Pressed = rp.Key1Pressed
	w.key2Pressed = rp.Key2Pressed

	w.viewport.ScreenWidthPixels = rp.ScreenWidthPixels
	w.viewport.ScreenHeightPixels = rp.ScreenHeightPixels
	w.cursorWorldPosition = w.viewport.ScreenPositionToWorldPosition(vec2.T{float64(w.cursorScreenPositionX), float64(w.cursorScreenPositionY)})

	w.simulateSand(simDelta)

	rs.SimTime = w.curSimTime
	rs.Viewport.Center[0] = w.viewport.Center[0]
	rs.Viewport.Center[1] = w.viewport.Center[1]
	rs.Viewport.Angle = w.viewport.Angle
	rs.Viewport.PixelsPerMeter = w.viewport.PixelsPerMeter
	rs.Viewport.ScreenWidthPixels = w.viewport.ScreenWidthPixels
	rs.Viewport.ScreenHeightPixels = w.viewport.ScreenHeightPixels
	rs.CursorWorldPosition[0] = w.cursorWorldPosition[0]
	rs.CursorWorldPosition[1] = w.cursorWorldPosition[1]
	rs.ResX = w.resX
	rs.ResY = w.resY
	if len(rs.SandLayer) != w.resX*w.resY {
		rs.SandLayer = make([]int, w.resX*w.resY)
	}
	for cx := 0; cx < w.resX; cx++ {
		for cy := 0; cy < w.resY; cy++ {
			rs.SandLayer[cy*w.resX+cx] = w.sandCurLayer[cy*w.resX+cx]
		}
	}
}

func (w *World) simulateSand(simDelta int64) {

	w.sandTickCounter++

	// Zero out scratch
	for cx := 0; cx < w.resX; cx++ {
		for cy := 0; cy < w.resY; cy++ {
			w.sandScratchLayer[cy*w.resX+cx] = 0
		}
	}

	if w.sandTickCounter == 0 {
		w.sandScratchLayer[(w.resY-10)*w.resX+(w.resX/2)] += 1000
	}

	if w.mouseButtonLeftDown {
		cursorX, cursorY, ok := w.worldPositionToCellIndex(w.cursorWorldPosition)
		if ok {
			w.sandScratchLayer[cursorY*w.resX+cursorX] += 1000
		}
	}

	// Apply sand forces
	for cx := 1; cx < w.resX-1; cx++ {
		for cy := 1; cy < w.resY-1; cy++ {
			sandInThisCell := w.sandCurLayer[cy*w.resX+cx]
			roomBelow := w.roomInCellAllowingOverpressure(cx, cy+1)
			if roomBelow > 0 {
				if sandInThisCell < roomBelow {
					w.sandScratchLayer[(cy+1)*w.resX+(cx+0)] += sandInThisCell
				} else {
					w.sandScratchLayer[(cy+0)*w.resX+(cx+0)] += sandInThisCell - roomBelow
					w.sandScratchLayer[(cy+1)*w.resX+(cx+0)] += roomBelow
				}
			} else {
				w.sandScratchLayer[(cy+0)*w.resX+(cx+0)] += sandInThisCell
			}
		}
	}

	// Apply boundary conditions -- keep sand out of the edge cells.
	for cx := 0; cx < w.resX; cx++ {
		w.moveScratchSand(cx, 0, cx, 1)
		w.moveScratchSand(cx, w.resY-1, cx, w.resY-2)
	}
	for cy := 0; cy < w.resY; cy++ {
		w.moveScratchSand(0, cy, 1, cy)
		w.moveScratchSand(w.resX-1, cy, w.resX-2, cy)
	}

	// Exchange scratch and cur
	w.sandCurLayer, w.sandScratchLayer = w.sandScratchLayer, w.sandCurLayer

	for i := 0; i < 10; i++ {
		w.relaxSand()
	}
}

func (w *World) roomInCellAllowingOverpressure(cx, cy int) int {
	if cx == 0 || cy == 0 || cx == w.resX-1 || cy == w.resY-1 {
		return 0
	}

	sandInCell := w.sandCurLayer[cy*w.resX+cx]
	if sandInCell < 1500 {
		return 1500 - sandInCell
	}
	return 0
}

func (w *World) relaxSand() {
	// Zero out scratch
	for cx := 0; cx < w.resX; cx++ {
		for cy := 0; cy < w.resY; cy++ {
			w.sandScratchLayer[cy*w.resX+cx] = 0
		}
	}

	// Spread out sand (into scratch).  Use gutters --- don't iterate over the
	// edges, so we don't have to have a bunch of special cases.
	for cx := 1; cx < w.resX-1; cx++ {
		for cy := 1; cy < w.resY-1; cy++ {

			// TODO: Freeze if no overpressure is detected around point?

			budget := w.sandCurLayer[cy*w.resX+cx]
			retain := 0
			spread := 0
			if budget <= 1000 {
				retain = budget
				spread = 0
				w.sandScratchLayer[(cy+0)*w.resX+(cx+0)] += retain
			} else if budget < 1400 {
				retain = 1000
				spread = budget - 1000

				perCellSpread := spread / 24
				lastCellSpread := spread - (7 * perCellSpread)
				w.sandScratchLayer[(cy+0)*w.resX+(cx+0)] += retain
				w.sandScratchLayer[(cy-1)*w.resX+(cx-1)] += perCellSpread
				w.sandScratchLayer[(cy+0)*w.resX+(cx-1)] += perCellSpread
				w.sandScratchLayer[(cy+1)*w.resX+(cx-1)] += perCellSpread
				w.sandScratchLayer[(cy+1)*w.resX+(cx+0)] += perCellSpread
				w.sandScratchLayer[(cy+1)*w.resX+(cx+1)] += perCellSpread
				w.sandScratchLayer[(cy+0)*w.resX+(cx+1)] += perCellSpread
				w.sandScratchLayer[(cy-1)*w.resX+(cx+1)] += perCellSpread
				w.sandScratchLayer[(cy-1)*w.resX+(cx+0)] += lastCellSpread
			} else {
				retain = 1000
				spread = budget - 1000

				// Central cell
				perCellSpread := spread / 16
				lastCellSpread := spread - (7 * perCellSpread)
				w.sandScratchLayer[(cy+0)*w.resX+(cx+0)] += retain
				w.sandScratchLayer[(cy-1)*w.resX+(cx-1)] += perCellSpread
				w.sandScratchLayer[(cy+0)*w.resX+(cx-1)] += perCellSpread
				w.sandScratchLayer[(cy+1)*w.resX+(cx-1)] += perCellSpread
				w.sandScratchLayer[(cy+1)*w.resX+(cx+0)] += perCellSpread
				w.sandScratchLayer[(cy+1)*w.resX+(cx+1)] += perCellSpread
				w.sandScratchLayer[(cy+0)*w.resX+(cx+1)] += perCellSpread
				w.sandScratchLayer[(cy-1)*w.resX+(cx+1)] += perCellSpread
				w.sandScratchLayer[(cy-1)*w.resX+(cx+0)] += lastCellSpread
			}
		}
	}

	// Apply boundary conditions -- keep sand out of the edge cells.
	for cx := 0; cx < w.resX; cx++ {
		w.moveScratchSand(cx, 0, cx, 1)
		w.moveScratchSand(cx, w.resY-1, cx, w.resY-2)
	}
	for cy := 0; cy < w.resY; cy++ {
		w.moveScratchSand(0, cy, 1, cy)
		w.moveScratchSand(w.resX-1, cy, w.resX-2, cy)
	}

	// Exchange scratch and cur
	w.sandCurLayer, w.sandScratchLayer = w.sandScratchLayer, w.sandCurLayer
}

func (w *World) moveScratchSand(srcX, srcY, dstX, dstY int) {
	w.sandScratchLayer[dstY*w.resX+dstX] += w.sandScratchLayer[srcY*w.resX+srcX]
	w.sandScratchLayer[srcY*w.resX+srcX] = 0
}

func (w *World) worldPositionToCellIndex(wp vec2.T) (int, int, bool) {
	tileWidthX := 0.1
	tileWidthY := 0.1

	totalWidthX := float64(w.resX) * tileWidthX
	totalWidthY := float64(w.resY) * tileWidthY

	// Top left of top left tile on the world grid.
	left := -totalWidthX / 2.0
	right := +totalWidthX / 2.0
	top := totalWidthY / 2.0
	bottom := -totalWidthY / 2.0

	if !(wp[0] >= left && wp[0] < right && wp[1] >= bottom && wp[1] < top) {
		return 0, 0, false
	}

	cx := int(math.Floor((wp[0] - left) / tileWidthX))
	cy := int(math.Floor((top - wp[1]) / tileWidthY))

	return cx, cy, true
}

func (w *World) inBounds(cx, cy int) bool {
	return cx >= 0 && cx < w.resX && cy >= 0 && cy < w.resY
}
