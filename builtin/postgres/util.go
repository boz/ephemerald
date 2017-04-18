package postgres

import (
	"database/sql"
	"fmt"
	"net/url"

	"github.com/boz/ephemerald/lifecycle"
	"github.com/boz/ephemerald/params"
	_ "github.com/lib/pq"
)

func openDB(e lifecycle.Env, p params.Params) (*sql.DB, error) {
	url := pgURL(p)
	e.Log().WithField("url", url).Debug("open")
	db, err := sql.Open("postgres", url)
	if err != nil {
		e.Log().WithError(err).Error("open")
	}
	return db, err
}

func pgURL(p params.Params) string {
	ui := url.UserPassword(p.Username, p.Password)
	return fmt.Sprintf("postgres://%v@%v:%v/%v?sslmode=disable",
		ui.String(),
		url.QueryEscape(p.Hostname),
		url.QueryEscape(p.Port),
		url.QueryEscape(p.Database))
}
