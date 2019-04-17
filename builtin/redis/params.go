package redis

import "github.com/boz/ephemerald/params"

type redisParams struct {
	Database string
}

func (p redisParams) ParamConfig() params.Config {
	pc := make(map[string]string)
	if p.Database != "" {
		pc["database"] = p.Database
	}
	return pc
}

func defaultParamConfig() params.Config {
	return map[string]string{
		"database": "0",
	}
}
