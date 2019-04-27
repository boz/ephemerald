package postgres

import "github.com/boz/ephemerald/params"

type pgParams struct {
	Username string
	Password string
	Database string
}

func (pgp pgParams) ParamConfig() params.Config {
	p := make(map[string]string)
	if pgp.Username != "" {
		p["username"] = pgp.Username
	}
	if pgp.Password != "" {
		p["password"] = pgp.Password
	}
	if pgp.Database != "" {
		p["database"] = pgp.Database
	}
	return p
}

func defaultParamConfig() params.Config {
	return map[string]string{
		"username": "postgres",
		"password": "postgres",
		"database": "postgres",
	}
}