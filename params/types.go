package params

import "github.com/boz/ephemerald/types"

type Config map[string]string

type State struct {
	ID        types.ID `json:"id"`
	Host      string   `json:"host"`
	Port      string   `json:"port"`
	NumResets int      `json:"num-resets"`
}
