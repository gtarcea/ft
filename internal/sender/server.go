package sender

import "github.com/gtarcea/ft/pkg/ft"

type Server struct {
	address string
	states  *ft.State
}

func NewServer(address string) *Server {
	server := &Server{
		address: address,
		states:  ft.NewState(),
	}

	server.states.AddState("start", "pake")
	server.states.AddState("pake", "finfo")
	server.states.AddState("finfo", "finfo", "file-chunk", "file-done")
	server.states.AddState("file-chunk", "file-chunk", "file-done")
	server.states.AddState("file-done", "finfo", "done")

	return server
}
