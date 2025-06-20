package simulation

import (
	"math"
	"time"

	"github.com/ahmedtd/gogames/lib/vmath/vec3"
)

type FrameInput struct {
	Delta time.Duration
}

type RenderState struct {
	BallPosition vec3.T
}

type SimulationState struct {
	curTime time.Time

	ballPosition vec3.T
}

func (s *SimulationState) DumpToRenderState(out *RenderState) {
	out.BallPosition = s.ballPosition
}

type Simulation struct {
	prevState *SimulationState
	nextState *SimulationState
}

func New() *Simulation {
	return &Simulation{
		prevState: &SimulationState{},
		nextState: &SimulationState{},
	}
}

func (s *Simulation) Update(in *FrameInput, out *RenderState) {
	s.nextState.curTime = s.prevState.curTime.Add(in.Delta)

	s.nextState.ballPosition = s.prevState.ballPosition
	s.nextState.ballPosition[0] = math.Sin(time.Duration(s.nextState.curTime.UnixNano()).Seconds())
	s.nextState.ballPosition[1] = math.Cos(time.Duration(s.nextState.curTime.UnixNano()).Seconds())

	s.nextState.DumpToRenderState(out)

	s.nextState, s.prevState = s.prevState, s.nextState
}
