package net

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/boz/ephemerald"
	"github.com/boz/ephemerald/params"
	"github.com/gorilla/mux"
)

type Server struct {
	l   *net.TCPListener
	srv *http.Server

	pools ephemerald.PoolSet

	closech chan bool
}

type ServerBuilder struct {
	address string
	pools   ephemerald.PoolSet
}

func NewServerBuilder() *ServerBuilder {
	return &ServerBuilder{
		address: DefaultListenAddress,
	}
}

func (sb *ServerBuilder) WithPoolSet(pools ephemerald.PoolSet) *ServerBuilder {
	sb.pools = pools
	return sb
}

func (sb *ServerBuilder) WithAddress(address string) *ServerBuilder {
	sb.address = address
	return sb
}

func (sb *ServerBuilder) WithPort(port int) *ServerBuilder {
	address, _, _ := net.SplitHostPort(sb.address)
	sb.address = net.JoinHostPort(address, strconv.Itoa(port))
	return sb
}

func (sb *ServerBuilder) Create() (*Server, error) {
	server := &Server{
		closech: make(chan bool),
		pools:   sb.pools,
	}

	r := mux.NewRouter()

	r.HandleFunc(rpcCheckoutPath, server.handleCheckoutBatch).
		Methods("PUT")
	r.HandleFunc(rpcCheckoutPath+"/{pool}", server.handleCheckoutPool).
		Methods("PUT")

	r.HandleFunc(rpcReturnPath, server.handleReturnBatch).
		Headers("Content-Type", rpcContentType).
		Methods("DELETE")
	r.HandleFunc(rpcReturnPath+"/{pool}/{id}", server.handleReturn).
		Methods("DELETE")

	l, err := net.Listen("tcp", sb.address)
	if err != nil {
		return nil, err
	}

	server.l = l.(*net.TCPListener)

	server.srv = &http.Server{
		Handler: r,
	}

	return server, nil
}

func (s *Server) Run() {
	defer close(s.closech)
	s.srv.Serve(s.l)
	s.stopPools()
}

func (s *Server) Close() {
	s.l.Close()
}

func (s *Server) ServerCloseNotify() chan bool {
	return s.closech
}

func (s *Server) Address() string {
	return s.l.Addr().String()
}

func (s *Server) Port() int {
	_, port, _ := net.SplitHostPort(s.Address())
	p, _ := strconv.Atoi(port)
	return p
}

func (s *Server) stopPools() {
	s.pools.Stop()
}

func (s *Server) handleCheckoutBatch(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		http.Error(w, fmt.Sprint(err), http.StatusBadRequest)
		return
	}

	ps, err := s.pools.CheckoutWith(r.Context())

	for name, p := range ps {
		p2, e := p.ForHost(host)
		if e != nil {
			err = e
			break
		}
		ps[name] = p2
	}

	if err != nil {
		s.pools.ReturnAll(ps)
		http.Error(w, fmt.Sprint(err), http.StatusRequestTimeout)
		return
	}

	buf, err := json.Marshal(ps)
	if err != nil {
		http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
		s.pools.ReturnAll(ps)
		return
	}

	w.Header().Set("Content-Type", rpcContentType)
	w.Write(buf)
}

func (s *Server) handleCheckoutPool(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		http.Error(w, fmt.Sprint(err), http.StatusBadRequest)
		return
	}

	poolName := mux.Vars(r)["pool"]
	if poolName == "" {
		http.Error(w, "Invalid pool name", http.StatusBadRequest)
		return
	}

	ps, err := s.pools.CheckoutWith(r.Context(), poolName)
	if err != nil {
		s.pools.ReturnAll(ps)
		http.Error(w, fmt.Sprint(err), http.StatusRequestTimeout)
		return
	}

	params, ok := ps[poolName]
	if !ok {
		s.pools.ReturnAll(ps)
		http.Error(w, "Pool not found", http.StatusInternalServerError)
		return
	}

	params, err = params.ForHost(host)
	if err != nil {
		s.pools.ReturnAll(ps)
		http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
		return
	}

	buf, err := json.Marshal(params)
	if err != nil {
		s.pools.ReturnAll(ps)
		http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", rpcContentType)
	w.Write(buf)
}

func (s *Server) handleReturnBatch(w http.ResponseWriter, r *http.Request) {
	ps := params.Set{}

	dec := json.NewDecoder(r.Body)

	if err := dec.Decode(&ps); err != nil {
		http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
		return
	}
	s.pools.ReturnAll(ps)

	w.Header().Set("Content-Type", rpcContentType)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleReturn(w http.ResponseWriter, r *http.Request) {
	pool := mux.Vars(r)["pool"]
	if pool == "" {
		http.Error(w, "Invalid pool name", http.StatusBadRequest)
		return
	}

	id := mux.Vars(r)["id"]
	if id == "" {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	s.pools.Return(pool, itemID(id))

	w.Header().Set("Content-Type", rpcContentType)
	w.WriteHeader(http.StatusOK)
}

type itemID string

func (i itemID) ID() string {
	return string(i)
}
