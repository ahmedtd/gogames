package world

import (
	"time"

	"shellworld/americana/resource/imagestamp"
	"shellworld/americana/vmath/mat33"
	"shellworld/americana/vmath/vec2"
)

type UIInput struct {
	UpdatedAt time.Time

	ScreenWidthPixels  int
	ScreenHeightPixels int

	KeyAPressed bool
	KeyDPressed bool
	KeySPressed bool
	KeyWPressed bool
}

type WorldRawInput struct {
	KeyAPressed bool
	KeyDPressed bool
	KeySPressed bool
	KeyWPressed bool
}

type Viewport struct {
	Center             vec2.T
	Angle              float64
	PixelsPerMeter     float64
	ScreenWidthPixels  int
	ScreenHeightPixels int
}

type RenderState struct {
	SimTime int64

	Viewport Viewport

	HumanRenderStates []HumanRenderState
	RatRenderStates   []RatRenderState
}

func (r *RenderState) FindHumanRenderStateForID(id int64) *HumanRenderState {
	for i := range r.HumanRenderStates {
		if r.HumanRenderStates[i].ID == id {
			return &r.HumanRenderStates[i]
		}
	}

	r.HumanRenderStates = append(r.HumanRenderStates, HumanRenderState{})
	return &r.HumanRenderStates[len(r.HumanRenderStates)-1]
}

func (r *RenderState) FindRatRenderStateForID(id int64) *RatRenderState {
	for i := range r.RatRenderStates {
		if r.RatRenderStates[i].ID == id {
			return &r.RatRenderStates[i]
		}
	}

	r.RatRenderStates = append(r.RatRenderStates, RatRenderState{})
	return &r.RatRenderStates[len(r.RatRenderStates)-1]
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
	nextID int64

	curStep   int64
	nanoDelta int64
	secDelta  float64

	prevRealTime time.Time
	curRealTime  time.Time

	prevSimTime int64
	curSimTime  int64

	Actors        []Actor
	PlayerControl int

	FlockingController *FlockingController

	worldRawInput WorldRawInput

	// TODO: Right now there is unclear ownership of the Viewport.
	Viewport *Viewport
}

func (w *World) Update(uiInput *UIInput, rs *RenderState) {
	newRealTime := uiInput.UpdatedAt

	w.curStep++
	w.nanoDelta = 1000000000 / 60
	w.secDelta = 1.0 / 60.0

	w.prevRealTime = w.curRealTime
	w.curRealTime = newRealTime

	w.prevSimTime = w.curSimTime
	w.curSimTime = w.prevSimTime + w.nanoDelta

	w.Viewport.ScreenWidthPixels = uiInput.ScreenWidthPixels
	w.Viewport.ScreenHeightPixels = uiInput.ScreenHeightPixels

	w.worldRawInput.KeyAPressed = uiInput.KeyAPressed
	w.worldRawInput.KeyDPressed = uiInput.KeyDPressed
	w.worldRawInput.KeySPressed = uiInput.KeySPressed
	w.worldRawInput.KeyWPressed = uiInput.KeyWPressed

	// Boil input down to player actions.
	playerDesiredMove := vec2.T{}
	wPressed := w.worldRawInput.KeyWPressed
	aPressed := w.worldRawInput.KeyAPressed
	sPressed := w.worldRawInput.KeySPressed
	dPressed := w.worldRawInput.KeyDPressed
	countPressed := 0
	if wPressed {
		countPressed++
	}
	if aPressed {
		countPressed++
	}
	if sPressed {
		countPressed++
	}
	if dPressed {
		countPressed++
	}
	if countPressed == 1 {
		if wPressed {
			playerDesiredMove = vec2.T{0.0, 1.0}
		}
		if aPressed {
			playerDesiredMove = vec2.T{-1.0, 0.0}
		}
		if sPressed {
			playerDesiredMove = vec2.T{0.0, -1.0}
		}
		if dPressed {
			playerDesiredMove = vec2.T{1.0, 0.0}
		}
	}
	if countPressed == 2 {
		if wPressed && aPressed {
			playerDesiredMove = vec2.T{-1.0, 1.0}.Normalize()
		}
		if wPressed && dPressed {
			playerDesiredMove = vec2.T{1.0, 1.0}.Normalize()
		}
		if sPressed && aPressed {
			playerDesiredMove = vec2.T{-1.0, -1.0}.Normalize()
		}
		if sPressed && dPressed {
			playerDesiredMove = vec2.T{1.0, -1.0}.Normalize()
		}
	}

	player := w.Actors[w.PlayerControl].(*Human)
	player.DesiredMove = playerDesiredMove

	w.FlockingController.Update()

	for _, a := range w.Actors {
		a.Update()
	}

	// Update render state.
	rs.SimTime = w.curSimTime
	rs.Viewport.Center[0] = w.Viewport.Center[0]
	rs.Viewport.Center[1] = w.Viewport.Center[1]
	rs.Viewport.Angle = w.Viewport.Angle
	rs.Viewport.PixelsPerMeter = w.Viewport.PixelsPerMeter
	rs.Viewport.ScreenWidthPixels = w.Viewport.ScreenWidthPixels
	rs.Viewport.ScreenHeightPixels = w.Viewport.ScreenHeightPixels
	for _, a := range w.Actors {
		a.UpdateRenderState(rs)
	}
}

func (w *World) TrackActor(actor Actor) int64 {
	id := w.nextID
	w.nextID++
	w.Actors = append(w.Actors, actor)
	return id
}

type Actor interface {
	GetID() int64
	JoinWorld(world *World) int64
	Update()

	UpdateRenderState(rs *RenderState)
}

type Gender int

const (
	Female Gender = iota
	Male
)

type Human struct {
	ID    int64
	world *World

	Name   string
	Gender Gender

	Location vec2.T
	Facing   vec2.T
	Velocity vec2.T

	DesiredMove vec2.T

	LeftFootLocation  vec2.T
	LeftFootFacing    vec2.T
	RightFootLocation vec2.T
	RightFootFacing   vec2.T

	HeldInLeftHand  int64
	HeldInRightHand int64

	RenderTint imagestamp.Tint
}

type HumanRenderState struct {
	ID int64

	Location vec2.T
	Facing   vec2.T

	LeftFootLocation vec2.T
	LeftFootFacing   vec2.T

	RightFootLocation vec2.T
	RightFootFacing   vec2.T

	RenderTint imagestamp.Tint
}

var _ Actor = &Human{}

func (h *Human) GetID() int64 {
	return h.ID
}

func (h *Human) JoinWorld(world *World) int64 {
	h.world = world
	h.ID = world.TrackActor(h)
	return h.ID
}

func (h *Human) Update() {
	h.Velocity[0] = 1.5 * h.DesiredMove[0]
	h.Velocity[1] = 1.5 * h.DesiredMove[1]

	h.Facing[0] += 0.2 * h.Velocity[0]
	h.Facing[1] += 0.2 * h.Velocity[1]
	h.Facing = h.Facing.Normalize()

	h.Location[0] += h.Velocity[0] * h.world.secDelta
	h.Location[1] += h.Velocity[1] * h.world.secDelta

	h.updateFeet()
}

func (h *Human) updateFeet() {
	strideLength := 0.7
	stanceWidth := 0.3

	left := vec2.T{
		-h.Facing[1],
		h.Facing[0],
	}

	if h.Velocity.Norm() > 0.5 {
		// There's an active input; the human is moving.  Run the walk cycle.
		if vec2.Sub(h.Location, h.LeftFootLocation).Norm() > 0.75*strideLength {
			forward := vec2.Add(h.Location, vec2.MulSV(0.25, h.Facing))
			h.LeftFootLocation = vec2.Add(forward, vec2.MulSV(0.5*stanceWidth, left))
			h.LeftFootFacing = h.Facing
		}
		if vec2.Sub(h.Location, h.RightFootLocation).Norm() > 0.75*strideLength {
			forward := vec2.Add(h.Location, vec2.MulSV(0.25, h.Facing))
			h.RightFootLocation = vec2.Add(forward, vec2.MulSV(-0.5*stanceWidth, left))
			h.RightFootFacing = h.Facing
		}
	} else {
		// There's no active input, the human is stopped. Run the stand cycle.

		h.LeftFootLocation[0] = h.Location[0] + left[0]*0.5*stanceWidth
		h.LeftFootLocation[1] = h.Location[1] + left[1]*0.5*stanceWidth
		h.LeftFootFacing = h.Facing

		h.RightFootLocation[0] = h.Location[0] - left[0]*0.5*stanceWidth
		h.RightFootLocation[1] = h.Location[1] - left[1]*0.5*stanceWidth
		h.RightFootFacing = h.Facing
	}
}

func (h *Human) GetLocation() vec2.T {
	return h.Location
}

func (h *Human) UpdateRenderState(rs *RenderState) {
	hrs := rs.FindHumanRenderStateForID(h.ID)

	hrs.ID = h.ID
	hrs.Location = h.Location
	hrs.Facing = h.Facing
	hrs.LeftFootLocation = h.LeftFootLocation
	hrs.LeftFootFacing = h.LeftFootFacing
	hrs.RightFootLocation = h.RightFootLocation
	hrs.RightFootFacing = h.RightFootFacing

	hrs.RenderTint = h.RenderTint
}

type Rat struct {
	id    int64
	world *World

	Location vec2.T
	Facing   vec2.T
	Velocity vec2.T

	MaxSpeed float64

	// Flocking inputs, set by the flocking controller.
	FlockingForce vec2.T

	RenderTint imagestamp.Tint
}

type RatRenderState struct {
	ID int64

	Location vec2.T
	Facing   vec2.T

	RenderTint imagestamp.Tint
}

var _ Actor = &Rat{}

func (r *Rat) GetID() int64 {
	return r.id
}

func (r *Rat) JoinWorld(world *World) int64 {
	r.world = world
	r.id = world.TrackActor(r)
	return r.id
}

func (r *Rat) Update() {
	totalForce := vec2.T{}

	distanceFromCenter := r.Location.Norm()
	directionFromCenter := r.Location.Normalize()
	if distanceFromCenter < 10.0 {
		totalForce[0] = r.FlockingForce[0]
		totalForce[1] = r.FlockingForce[1]
	} else if distanceFromCenter < 15.0 {
		t := (distanceFromCenter - 10.0) / 5.0
		totalForce[0] = t*-directionFromCenter[0] + (1.0-t)*r.FlockingForce[0]
		totalForce[1] = t*-directionFromCenter[1] + (1.0-t)*r.FlockingForce[1]
	} else {
		totalForce[0] = -directionFromCenter[0]
		totalForce[1] = -directionFromCenter[1]
	}

	// totalForce[0] = r.FlockingForce[0]
	// totalForce[1] = r.FlockingForce[1]

	r.Velocity[0] += totalForce[0] * r.world.secDelta
	r.Velocity[1] += totalForce[1] * r.world.secDelta

	// Clamp velocity
	vel := r.Velocity.Norm()
	if vel > r.MaxSpeed {
		r.Velocity = r.Velocity.Normalize()
		r.Velocity[0] *= (r.MaxSpeed * r.MaxSpeed)
		r.Velocity[1] *= (r.MaxSpeed * r.MaxSpeed)
	}

	r.Location[0] += r.Velocity[0] * r.world.secDelta
	r.Location[1] += r.Velocity[1] * r.world.secDelta

	r.Facing[0] += 0.2 * r.Velocity[0]
	r.Facing[1] += 0.2 * r.Velocity[1]
	r.Facing = r.Facing.Normalize()
}

func (r *Rat) UpdateRenderState(rs *RenderState) {
	rrs := rs.FindRatRenderStateForID(r.id)

	rrs.ID = r.id
	rrs.Location = r.Location
	rrs.Facing = r.Facing
	rrs.RenderTint = r.RenderTint
}

type FlockingController struct {
	World *World
}

func (c *FlockingController) Update() {
	rats := []*Rat{}
	for _, a := range c.World.Actors {
		r, ok := a.(*Rat)
		if !ok {
			continue
		}

		rats = append(rats, r)
	}

	for _, r1 := range rats {
		separation := vec2.T{}
		neighborAverageVelocity := vec2.T{}
		neighborAveragePosition := vec2.T{}
		neighborsConsidered := 0

		for _, r2 := range rats {
			if r1 == r2 {
				continue
			}

			between := vec2.T{}
			between[0] = r2.Location[0] - r1.Location[0]
			between[1] = r2.Location[1] - r1.Location[1]

			distance := between.Norm()
			if distance > 5.0 {
				continue
			}

			neighborsConsidered++

			neighborAveragePosition[0] += r2.Location[0]
			neighborAveragePosition[1] += r2.Location[1]

			neighborAverageVelocity[0] += r2.Velocity[0]
			neighborAverageVelocity[1] += r2.Velocity[1]

			separation[0] += -1.0 / between[0]
			separation[1] += -1.0 / between[1]
		}

		if neighborsConsidered == 0 {
			continue
		}

		neighborAveragePosition[0] /= float64(neighborsConsidered)
		neighborAveragePosition[1] /= float64(neighborsConsidered)

		neighborAverageVelocity[0] /= float64(neighborsConsidered)
		neighborAverageVelocity[1] /= float64(neighborsConsidered)

		cohesion := vec2.T{}
		cohesion[0] = neighborAveragePosition[0] - r1.Location[0]
		cohesion[1] = neighborAveragePosition[1] - r1.Location[1]
		cohesion = cohesion.Normalize()

		alignment := vec2.T{}
		alignment[0] = neighborAverageVelocity[0] - r1.Velocity[0]
		alignment[1] = neighborAverageVelocity[1] - r1.Velocity[1]

		r1.FlockingForce[0] = separation[0] + 1.0*cohesion[0] + 5.0*alignment[0]
		r1.FlockingForce[1] = separation[1] + 1.0*cohesion[1] + 5.0*alignment[1]
	}
}
