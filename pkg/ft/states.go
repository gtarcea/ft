package ft

import "fmt"

type State struct {
	states       map[string]map[string]string
	CurrentState string
}

var ErrUnknownState = fmt.Errorf("unknown state")
var ErrInvalidNextState = fmt.Errorf("invalid next state")

func NewState() *State {
	return &State{
		states: make(map[string]map[string]string),
	}
}

func (s *State) SetStartState(state string) {
	s.CurrentState = state
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
// true if the transitionState is a valid state from the CurrentState. Otherwise
// it returns false.
func (s *State) IsValidNextState(nextState string) bool {
	valid, _ := s.IsValidNextStateWithError(nextState)
	return valid
}

// IsValidNextStateWithError takes the CurrentState and the transitionState and returns
// true, nil if the transitionState is a valid state from the CurrentState. Otherwise it
// will return false and either ErrUnknownState if CurrentState is not a known state, or
// false and ErrInvalidNextState if transitionState is not a valid next state from CurrentState.
func (s *State) IsValidNextStateWithError(nextState string) (bool, error) {
	states, ok := s.states[s.CurrentState]
	if !ok {
		return false, ErrUnknownState
	}

	if _, ok := states[nextState]; !ok {
		return false, ErrInvalidNextState
	}

	return true, nil
}

func (s *State) ValidateAndAdvanceToNextState(nextState string) error {
	if _, err := s.IsValidNextStateWithError(nextState); err != nil {
		return err
	}

	s.CurrentState = nextState
	return nil
}
