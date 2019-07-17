package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"

	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/log"
	enet "github.com/boz/ephemerald/net"
	"github.com/boz/ephemerald/poolset"
	"github.com/boz/ephemerald/types"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type Server interface {
	Address() string
	Run() error
	Close()
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

func WithLog(log logrus.FieldLogger) Opt {
	return func(s *server) error {
		s.log = log
		return nil
	}
}

type server struct {
	address string
	pset    poolset.PoolSet

	log      logrus.FieldLogger
	listener *net.TCPListener
	srv      *http.Server
}

func New(opts ...Opt) (Server, error) {

	s := &server{
		address: enet.DefaultListenAddress,
	}

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}

	if s.pset == nil {
		return nil, errors.New("WithPoolSet required")
	}

	if s.log == nil {
		s.log = log.Default().WithField("cmp", "net/server")
	}

	r := mux.NewRouter()

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := log.NewContext(req.Context(), s.log.WithFields(logrus.Fields{
				"uri":    req.RequestURI,
				"method": req.Method,
			}))
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	r.HandleFunc("/pools", s.handlePoolList).
		Methods("GET")

	r.HandleFunc("/pool", s.handlePoolCreate).
		Methods("POST")

	r.HandleFunc("/pool/{pool-id}", s.handlePoolGet).
		Methods("GET")

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

	s.listener = l.(*net.TCPListener)

	s.srv = &http.Server{
		Handler: r,
	}

	return s, nil
}

func (s *server) Run() error {
	if err := s.srv.Serve(s.listener); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *server) Close() {
	if err := s.srv.Shutdown(context.TODO()); err != nil {
		s.log.WithError(err).Warn("closing down")
	}
}

func (s *server) Address() string {
	return s.listener.Addr().String()
}

func (s *server) handlePoolList(w http.ResponseWriter, r *http.Request) {
	pools, err := s.pset.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	models := make([]types.Pool, 0, len(pools))
	for _, pool := range pools {
		model, err := pool.Model(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		models = append(models, *model)
	}

	writeJSON(r.Context(), w, models)
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

	obj, err := pool.Model(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(r.Context(), w, obj)
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

func (s *server) handlePoolGet(w http.ResponseWriter, r *http.Request) {
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

	obj, err := pool.Model(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(r.Context(), w, obj)
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

	obj, err := pool.Checkout(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(r.Context(), w, obj)
}

func (s *server) handlePoolInstanceRelease(w http.ResponseWriter, r *http.Request) {

	pid, ok := mux.Vars(r)["pool-id"]
	if !ok {
		http.Error(w, "pool ID required", http.StatusBadRequest)
		return
	}

	id, ok := mux.Vars(r)["instance-id"]
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

func writeJSON(ctx context.Context, w http.ResponseWriter, obj interface{}) {
	buf, err := json.Marshal(obj)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("marshal response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(buf); err != nil {
		log.FromContext(ctx).WithError(err).Error("writing response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", enet.RPCContentType)
}
