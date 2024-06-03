package world

import (
	"math"
	"math/rand"
	"time"

	"shellworld/vmath/mat33"
	"shellworld/vmath/vec2"
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

	AntRenderStates          []AntRenderState
	AntRenderStatesByID      map[int64]int
	AntRenderStatesFreeSlots []int

	FoodRenderStates          []FoodRenderState
	FoodRenderStatesByID      map[int64]int
	FoodRenderStatesFreeSlots []int

	ScentRenderStates []ScentRenderState
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
	curStep   int64
	nanoDelta int64
	secDelta  float64

	prevRealTime time.Time
	curRealTime  time.Time

	prevSimTime int64
	curSimTime  int64

	NextID int64

	PrevAntStates []AntSimState
	NextAntStates []AntSimState
	FreeAntSlots  []int

	TimeTillNextAnt int64

	PrevFoodStates []FoodSimState
	NextFoodStates []FoodSimState
	FreeFoodSlots  []int

	TimeTillNextFood int64

	worldRawInput WorldRawInput

	Viewport *Viewport
}

func (w *World) GetNextID() int64 {
	id := w.NextID
	w.NextID++
	return id
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

	w.updateAnts()
	w.updateFood()
	// w.updateScents()

	// Update render state.
	rs.SimTime = w.curSimTime
	rs.Viewport.Center[0] = w.Viewport.Center[0]
	rs.Viewport.Center[1] = w.Viewport.Center[1]
	rs.Viewport.Angle = w.Viewport.Angle
	rs.Viewport.PixelsPerMeter = w.Viewport.PixelsPerMeter
	rs.Viewport.ScreenWidthPixels = w.Viewport.ScreenWidthPixels
	rs.Viewport.ScreenHeightPixels = w.Viewport.ScreenHeightPixels

	w.renderAnts(rs)
	w.renderFood(rs)
	// w.renderScents(rs)

	w.PrevAntStates, w.NextAntStates = w.NextAntStates, w.PrevAntStates
}

func (w *World) updateAnts() {
	if w.TimeTillNextAnt <= 0 {
		w.TimeTillNextAnt = int64(30 * time.Second)

		newAnt := AntSimState{
			ID:       w.GetNextID(),
			Location: vec2.T{rand.NormFloat64() * 10.0, rand.NormFloat64() * 10.0},
			Facing:   vec2.T{1.0, 0.0},
			Velocity: vec2.T{0.0, 0.0},
		}

		if len(w.FreeAntSlots) > 0 {
			slot := w.FreeAntSlots[len(w.FreeAntSlots)-1]
			w.FreeAntSlots = w.FreeAntSlots[:len(w.FreeAntSlots)-1]
			newAnt.Slot = slot
			w.PrevAntStates[slot] = newAnt
			w.NextAntStates[slot] = newAnt
		} else {
			newAnt.Slot = len(w.PrevAntStates)
			w.PrevAntStates = append(w.PrevAntStates, newAnt)
			w.NextAntStates = append(w.NextAntStates, newAnt)
		}
	} else {
		w.TimeTillNextAnt -= w.nanoDelta
	}

	for i := 0; i < len(w.PrevAntStates); i++ {
		prevAnt := &w.PrevAntStates[i]
		nextAnt := &w.NextAntStates[i]

		nextAnt.ID = prevAnt.ID
		nextAnt.Slot = prevAnt.Slot
		nextAnt.ActiveInSimulation = prevAnt.ActiveInSimulation

		if !prevAnt.ActiveInSimulation {
			continue
		}

		nextAnt.Location = prevAnt.Location
		nextAnt.Facing = prevAnt.Facing
		nextAnt.Velocity = prevAnt.Velocity

		closestFoodSlot := -1
		closestFoodVec := vec2.T{}
		closestFoodDistance := math.Inf(1)
		for j := 0; j < len(w.PrevFoodStates); j++ {
			if !w.PrevFoodStates[i].ActiveInSimulation {
				continue
			}

			dif := vec2.Sub(nextAnt.Location, w.PrevFoodStates[j].Location)
			closestFoodVec = dif
			dist := vec2.Dot(dif, dif)
			if dist < closestFoodDistance {
				closestFoodSlot = j
				closestFoodDistance = dist
			}
		}
		if closestFoodSlot != -1 {
			if closestFoodDistance >= 0.01 {
				nextAnt.Velocity[0] = 0.1 * closestFoodVec[0] / closestFoodDistance
				nextAnt.Velocity[1] = 0.1 * closestFoodVec[1] / closestFoodDistance
			}
		}

		nextAnt.Location[0] = prevAnt.Location[0] + w.secDelta*prevAnt.Velocity[0]
		nextAnt.Location[1] = prevAnt.Location[1] + w.secDelta*prevAnt.Velocity[1]
	}
}

func (w *World) updateFood() {
	if w.TimeTillNextFood <= 0 {
		w.TimeTillNextFood = int64(5 * time.Second)

		newFood := FoodSimState{
			ID:       w.GetNextID(),
			Location: vec2.T{rand.NormFloat64() * 10.0, rand.NormFloat64() * 10.0},
			Facing:   vec2.T{1.0, 0.0},
		}

		if len(w.FreeFoodSlots) > 0 {
			slot := w.FreeFoodSlots[len(w.FreeFoodSlots)-1]
			w.FreeFoodSlots = w.FreeFoodSlots[:len(w.FreeFoodSlots)-1]
			newFood.Slot = slot
			w.PrevFoodStates[slot] = newFood
			w.NextFoodStates[slot] = newFood
		} else {
			newFood.Slot = len(w.PrevFoodStates)
			w.PrevFoodStates = append(w.PrevFoodStates, newFood)
			w.NextFoodStates = append(w.NextFoodStates, newFood)
		}
	} else {
		w.TimeTillNextFood -= w.nanoDelta
	}

	for i := 0; i < len(w.PrevFoodStates); i++ {
		prevFood := &w.PrevFoodStates[i]
		nextFood := &w.NextFoodStates[i]

		nextFood.ID = prevFood.ID
		nextFood.Slot = prevFood.Slot
		nextFood.ActiveInSimulation = prevFood.ActiveInSimulation

		if !prevFood.ActiveInSimulation {
			continue
		}

		nextFood.Location = prevFood.Location
		nextFood.Facing = prevFood.Facing
	}
}

func (w *World) renderAnts(rs *RenderState) {
	for i := 0; i < len(w.NextAntStates); i++ {
		simState := &w.NextAntStates[i]

		var renderState *AntRenderState
		slot, ok := rs.AntRenderStatesByID[simState.ID]
		if ok {
			renderState = &rs.AntRenderStates[slot]
		} else if len(rs.AntRenderStatesFreeSlots) != 0 {
			// TODO: Pick free slot
			panic("unimplemented")
		} else {
			if rs.AntRenderStatesByID == nil {
				rs.AntRenderStatesByID = map[int64]int{}
			}

			rs.AntRenderStates = append(rs.AntRenderStates, AntRenderState{Live: true})
			rs.AntRenderStatesByID[simState.ID] = len(rs.AntRenderStates) - 1
			renderState = &rs.AntRenderStates[len(rs.AntRenderStates)-1]
		}

		renderState.Live = true
		renderState.ID = simState.ID
		renderState.Location = simState.Location
		renderState.Facing = simState.Facing
	}
}

func (w *World) renderFood(rs *RenderState) {
	for i := 0; i < len(w.NextFoodStates); i++ {
		simState := &w.NextFoodStates[i]

		var renderState *FoodRenderState
		slot, ok := rs.FoodRenderStatesByID[simState.ID]
		if ok {
			renderState = &rs.FoodRenderStates[slot]
		} else if len(rs.FoodRenderStatesFreeSlots) != 0 {
			// TODO: Pick free slot
			panic("unimplemented")
		} else {
			if rs.FoodRenderStatesByID == nil {
				rs.FoodRenderStatesByID = map[int64]int{}
			}

			rs.FoodRenderStates = append(rs.FoodRenderStates, FoodRenderState{Live: true})
			rs.FoodRenderStatesByID[simState.ID] = len(rs.FoodRenderStates) - 1
			renderState = &rs.FoodRenderStates[len(rs.FoodRenderStates)-1]
		}

		renderState.Live = true
		renderState.ID = simState.ID
		renderState.Location = simState.Location
		renderState.Facing = simState.Facing
	}
}

type AntSimState struct {
	ID                 int64
	Slot               int
	ActiveInSimulation bool

	Location vec2.T
	Facing   vec2.T
	Velocity vec2.T
}

type AntRenderState struct {
	// Is this renderstate live, or can it be reused for another entity?
	Live bool

	ID int64

	Location vec2.T
	Facing   vec2.T
}

type FoodSimState struct {
	ID                 int64
	Slot               int
	ActiveInSimulation bool

	Location vec2.T
	Facing   vec2.T
}

type FoodRenderState struct {
	// Is this renderstate live, or can it be reused for another entity?
	Live bool

	ID int64

	Location vec2.T
	Facing   vec2.T
}

type ScentSimState struct {
	ID   int64
	Slot int

	Location vec2.T
	Radius   float32
}

type ScentRenderState struct {
	Live bool
	ID   int64

	Location vec2.T
	Radius   float32
}
