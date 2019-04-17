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
	url, err := pgURL(p)
	if err != nil {
		return nil, err
	}
	e.Log().WithField("url", url).Debug("open")
	db, err := sql.Open("postgres", url)
	if err != nil {
		e.Log().WithError(err).Error("open")
	}
	return db, err
}

func pgURL(p params.Params) (string, error) {
	username, err := p.Get("username")
	if err != nil {
		return "", err
	}
	password, err := p.Get("password")
	if err != nil {
		return "", err
	}
	database, err := p.Get("database")
	if err != nil {
		return "", err
	}

	// TODO: make a param

	ui := url.UserPassword(username, password)
	return fmt.Sprintf("postgres://%v@%v:%v/%v?sslmode=disable",
		ui.String(),
		url.QueryEscape(p.Host()),
		url.QueryEscape(p.Port()),
		url.QueryEscape(database)), nil
}
