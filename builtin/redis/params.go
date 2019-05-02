package redis

type redisParams struct {
	Database string
}

func (p redisParams) overrides() map[string]string {
	pc := make(map[string]string)
	if p.Database != "" {
		pc["database"] = p.Database
	}
	return pc
}

func defaultParamConfig() map[string]string {
	return map[string]string{
		"database": "0",
	}
}
