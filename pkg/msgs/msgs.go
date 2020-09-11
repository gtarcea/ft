package msgs

const RoleReceiver = "receiver"
const RoleSender = "sender"

type Hello struct {
	RelayKey       string `json:"relay_key"`
	ConnectionType string `json:"connection_type"`
}

type Welcome struct {
	PakeMsg []byte `json:"pake_msg"`
}

type Goodbye struct {
	Reason string `json:"reason"`
}

type Pake struct {
	Body []byte `json:"body"`
}
