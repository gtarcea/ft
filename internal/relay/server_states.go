package relay

import "github.com/gtarcea/ft/pkg/ft"

var serverStates *ft.State

func init() {
	serverStates = ft.NewState()
	serverStates.AddState("start", "pake")
	serverStates.AddState("pake", "hello")
	serverStates.AddState("hello", "external_ips", "go")
	serverStates.AddState("external_ips", "go")
	// After go message there are no more messages that can be sent to the relay server
}
