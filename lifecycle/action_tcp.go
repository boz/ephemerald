package lifecycle

import (
	"encoding/json"
	"net"

	"github.com/boz/ephemerald/params"
)

func init() {
	MakeActionPlugin("tcp.connect", actionTCPConnectParse)
}

type actionTCPConnect struct {
	ActionConfig
}

func actionTCPConnectParse(buf []byte) (Action, error) {
	action := &actionTCPConnect{
		ActionConfig: DefaultActionConfig(),
	}
	return action, json.Unmarshal(buf, action)
}

func (a *actionTCPConnect) Do(e Env, p params.Params) error {
	address := net.JoinHostPort(p.Hostname, p.Port)
	con, err := net.DialTimeout("tcp", address, a.Timeout)
	defer con.Close()
	return err
}
