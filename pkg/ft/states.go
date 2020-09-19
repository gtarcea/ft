package ft

import "fmt"

type State struct {
	states map[string]map[string]string
}

var ErrUnknownState = fmt.Errorf("unknown state")
var ErrInvalidNextState = fmt.Errorf("invalid next state")

func NewState() *State {
	return &State{
		states: make(map[string]map[string]string),
	}
}

// AddState will create a new state if it doesn't exist and will add the
// transitions as valid transitions for that state. If the state already
// exists it will add the transitions to the state.
func (s *State) AddState(state string, transitions ...string) {
	states, ok := s.states[state]
	if !ok {
		states = make(map[string]string)
		s.states[state] = states
	}
	for _, transition := range transitions {
		states[transition] = transition
	}
}

// AddTransitionsToState will add transitions to a state. If the state does not
// exist it will be created.
func (s *State) AddTransitionsToState(state string, transitions ...string) {
	s.AddState(state, transitions...)
}

// IsValidNextState takes the current state and the transitionState and returns
// true if the transitionState is a valid state from the currentState. Otherwise
// it returns false.
func (s *State) IsValidNextState(currentState, transitionState string) bool {
	valid, _ := s.IsValidNextStateWithError(currentState, transitionState)
	return valid
}

// IsValidNextStateWithError takes the currentState and the transitionState and returns
// true, nil if the transitionState is a valid state from the currentState. Otherwise it
// will return false and either ErrUnknownState if currentState is not a known state, or
// false and ErrInvalidNextState if transitionState is not a valid next state from currentState.
func (s *State) IsValidNextStateWithError(currentState, transitionState string) (bool, error) {
	states, ok := s.states[currentState]
	if !ok {
		return false, ErrUnknownState
	}

	if _, ok := states[transitionState]; !ok {
		return false, ErrInvalidNextState
	}

	return true, nil
}
