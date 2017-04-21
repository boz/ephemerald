package ui

func NewNoopUI() UI {
	return noopUI{}
}

type noopUI struct{}

func (noopUI) Emitter() Emitter {
	return NewNoopEmitter()
}
func (noopUI) Stop() {}

type noopEmitter struct{}

func NewNoopEmitter() Emitter {
	return noopEmitter{}
}

func (e noopEmitter) ForPool(_ string) PoolEmitter { return e }

func (e noopEmitter) ForContainer(_ string) ContainerEmitter { return e }
func (e noopEmitter) EmitInitializing()                      {}
func (e noopEmitter) EmitInitializeError(_ error)            {}
func (e noopEmitter) EmitRunning()                           {}
func (e noopEmitter) EmitDraining()                          {}
func (e noopEmitter) EmitDone()                              {}
func (e noopEmitter) EmitNumItems(int)                       {}
func (e noopEmitter) EmitNumPending(int)                     {}
func (e noopEmitter) EmitNumReady(int)                       {}

func (e noopEmitter) EmitCreated()                                     {}
func (e noopEmitter) EmitStarted()                                     {}
func (e noopEmitter) EmitLive()                                        {}
func (e noopEmitter) EmitReady()                                       {}
func (e noopEmitter) EmitResetting()                                   {}
func (e noopEmitter) EmitExiting()                                     {}
func (e noopEmitter) EmitExited()                                      {}
func (e noopEmitter) EmitActionAttempt(string, string, int, int)       {}
func (e noopEmitter) EmitActionResult(string, string, int, int, error) {}
