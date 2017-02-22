package cpool

/*
func NewPG(size int) (Pool, error) {
	return NewPool(NewPGResource(), size)
}

func NewPGResource() *pgresource {
	return &pgresource{}
}

type ResourceConfig struct {
}

type pgresource struct {
}

func (r *pgresource) Config() *Config {
	return NewConfig().
		WithImage("postgres").
		ExposePort("tcp", 5432)
}

func (r *pgresource) Create() Item {
	return &pgitem{}
}

type pgitem struct {
}

func (i *pgitem) ID() string {
	return ""
}

func (i *pgitem) Configure() {
}

func (i *pgitem) Initialize() {
}

func (i *pgitem) Teardown() {
}
*/
