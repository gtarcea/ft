package msgs

const RoleReceiver = "receiver"
const RoleSender = "sender"

type Hello struct {
	ExternalIPs []string `json:"external_ips"`
	Role        string   `json:"role"`
	Key         string   `json:"key"`
}

type Welcome struct {
	Messages []string `json:"messages"`
	First    bool     `json:"first"`
}

type Goodbye struct {
	Reason string `json:"reason"`
}
