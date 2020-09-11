package msgs

const RoleReceiver = "receiver"
const RoleSender = "sender"

type Hello struct {
	PakeMsg []byte `json:"pake_msg"`
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
