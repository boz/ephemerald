package ui

type pstate string

const (
	pstateInit     pstate = "initializing"
	pstateErr      pstate = "error"
	pstateRunning  pstate = "running"
	pstateDraining pstate = "draining"
	pstateStopped  pstate = "stopped"

	pstateMaxLen = len(pstateInit)
)

type cstate string

const (
	cstateCreated   cstate = "created"
	cstateStarted   cstate = "started"
	cstateLive      cstate = "live"
	cstateReady     cstate = "ready"
	cstateResetting cstate = "resetting"
	cstateExiting   cstate = "exiting"
	cstateExited    cstate = "exited"

	cstateMaxLen = len(cstateResetting)
)

type pool struct {
	name  string
	state pstate

	numItems   int
	numPending int
	numReady   int

	err error
}

type container struct {
	pname string
	id    string

	state cstate

	lifecycleName  string
	actionName     string
	actionAttempt  int
	actionAttempts int
	actionError    error
}
