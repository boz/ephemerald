package runner

type Result interface {
	Value() interface{}
	Err() error
}

type result struct {
	value interface{}
	err   error
}

func (r result) Value() interface{} {
	return r.value
}

func (r result) Err() error {
	return r.err
}

func NewResult(value interface{}, err error) Result {
	return result{value, err}
}

func Do(fn func() Result) <-chan Result {
	ch := make(chan Result, 1)
	go func() {
		ch <- fn()
	}()
	return ch
}
