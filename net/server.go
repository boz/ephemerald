package net

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"

	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/poolset"
	"github.com/boz/ephemerald/types"
	"github.com/gorilla/mux"
)

type Server interface {
	Address() string
	Run()
}

type Opt func(*server) error

func WithAddress(address string) Opt {
	return func(s *server) error {
		s.address = address
		return nil
	}
}

func WithPoolSet(pset poolset.PoolSet) Opt {
	return func(s *server) error {
		s.pset = pset
		return nil
	}
}

type server struct {
	address string
	pset    poolset.PoolSet

	l   *net.TCPListener
	srv *http.Server
}

func New(opts ...Opt) (Server, error) {

	s := &server{
		address: DefaultListenAddress,
	}

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}

	if s.pset == nil {
		return nil, errors.New("WithPoolSet required")
	}

	r := mux.NewRouter()

	r.HandleFunc("/pool", s.handlePoolCreate).
		Methods("POST")

	r.HandleFunc("/pool/{pool-id}/checkout", s.handlePoolInstanceCheckout).
		Methods("POST")

	r.HandleFunc("/pool/{pool-id}/checkout/{instance-id}", s.handlePoolInstanceRelease).
		Methods("DELETE")

	r.HandleFunc("/pool/{pool-id}", s.handlePoolDelete).
		Methods("DELETE")

	l, err := net.Listen("tcp", s.address)
	if err != nil {
		return nil, err
	}

	s.l = l.(*net.TCPListener)

	s.srv = &http.Server{
		Handler: r,
	}

	return s, nil
}

func (s *server) Run() {
	s.srv.Serve(s.l)
}

func (s *server) Close() {
	s.l.Close()
}

func (s *server) Address() string {
	return s.l.Addr().String()
}

func (s *server) handlePoolCreate(w http.ResponseWriter, r *http.Request) {
	var cfg config.Pool
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&cfg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pool, err := s.pset.Create(r.Context(), cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	obj := pool.ID() // TODO: Model()

	buf, err := json.Marshal(obj)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", rpcContentType)
	w.Write(buf)
}

func (s *server) handlePoolDelete(w http.ResponseWriter, r *http.Request) {
	pid, ok := mux.Vars(r)["pool-id"]
	if !ok {
		http.Error(w, "pool ID required", http.StatusBadRequest)
		return
	}

	if err := s.pset.Delete(r.Context(), types.ID(pid)); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *server) handlePoolInstanceCheckout(w http.ResponseWriter, r *http.Request) {
	pid, ok := mux.Vars(r)["pool-id"]
	if !ok {
		http.Error(w, "pool ID required", http.StatusBadRequest)
		return
	}

	pool, err := s.pset.Get(r.Context(), types.ID(pid))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err = pool.Checkout(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	obj := "TODO: replace with co.model"
	buf, err := json.Marshal(obj)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", rpcContentType)
	w.Write(buf)
}

func (s *server) handlePoolInstanceRelease(w http.ResponseWriter, r *http.Request) {

	pid, ok := mux.Vars(r)["pool-id"]
	if !ok {
		http.Error(w, "pool ID required", http.StatusBadRequest)
		return
	}

	id, ok := mux.Vars(r)["id"]
	if !ok {
		http.Error(w, "Checkout ID required", http.StatusBadRequest)
		return
	}

	pool, err := s.pset.Get(r.Context(), types.ID(pid))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := pool.Release(r.Context(), types.ID(id)); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}
