package ui

type Emitter interface {
	ForPool(name string) PoolEmitter
}

type PoolEmitter interface {
	ForContainer(id string) ContainerEmitter

	EmitInitializing()
	EmitInitializeError(error)
	EmitRunning()
	EmitDraining()
	EmitDone()

	EmitNumItems(int)
	EmitNumPending(int)
	EmitNumReady(int)
}

type ContainerEmitter interface {
	EmitCreated()
	EmitStarted()
	EmitLive()
	EmitReady()
	EmitResetting()
	EmitExiting()
	EmitExited()

	EmitActionAttempt(string, string, int, int)
	EmitActionResult(string, string, int, int, error)
}

func newEmitter(processor *processor) Emitter {
	return &processorEmitter{processor}
}

type processorEmitter struct {
	processor *processor
}

type processorPoolEmitter struct {
	processor *processor
	poolName  string
}

type processorContainerEmitter struct {
	processor   *processor
	poolName    string
	containerId string
}

func (e *processorEmitter) ForPool(name string) PoolEmitter {
	return &processorPoolEmitter{
		processor: e.processor,
		poolName:  name,
	}
}

func (e *processorPoolEmitter) EmitInitializing() {
	e.sendEvent(pevent{peventInit, e.poolName, nil, 0})
}
func (e *processorPoolEmitter) EmitInitializeError(err error) {
	e.sendEvent(pevent{peventInitErr, e.poolName, err, 0})
}
func (e *processorPoolEmitter) EmitRunning() {
	e.sendEvent(pevent{peventRunning, e.poolName, nil, 0})
}
func (e *processorPoolEmitter) EmitDraining() {
	e.sendEvent(pevent{peventDraining, e.poolName, nil, 0})
}
func (e *processorPoolEmitter) EmitDone() {
	e.sendEvent(pevent{peventDone, e.poolName, nil, 0})
}
func (e *processorPoolEmitter) EmitNumItems(count int) {
	e.sendEvent(pevent{peventNumItems, e.poolName, nil, count})
}
func (e *processorPoolEmitter) EmitNumPending(count int) {
	e.sendEvent(pevent{peventNumPending, e.poolName, nil, count})
}
func (e *processorPoolEmitter) EmitNumReady(count int) {
	e.sendEvent(pevent{peventNumReady, e.poolName, nil, count})
}

func (e *processorPoolEmitter) sendEvent(event pevent) {
	e.processor.sendPoolEvent(event)
}

func (e *processorPoolEmitter) ForContainer(containerId string) ContainerEmitter {
	return &processorContainerEmitter{
		processor:   e.processor,
		poolName:    e.poolName,
		containerId: containerId,
	}
}

func (e *processorContainerEmitter) EmitCreated() {
	e.sendEvent(cevent{ceventCreated, e.containerId, e.poolName, "", "", 0, 0, nil})
}
func (e *processorContainerEmitter) EmitStarted() {
	e.sendEvent(cevent{ceventStarted, e.containerId, e.poolName, "", "", 0, 0, nil})
}
func (e *processorContainerEmitter) EmitLive() {
	e.sendEvent(cevent{ceventLive, e.containerId, e.poolName, "", "", 0, 0, nil})
}
func (e *processorContainerEmitter) EmitReady() {
	e.sendEvent(cevent{ceventReady, e.containerId, e.poolName, "", "", 0, 0, nil})
}
func (e *processorContainerEmitter) EmitResetting() {
	e.sendEvent(cevent{ceventResetting, e.containerId, e.poolName, "", "", 0, 0, nil})
}
func (e *processorContainerEmitter) EmitExiting() {
	e.sendEvent(cevent{ceventExiting, e.containerId, e.poolName, "", "", 0, 0, nil})
}
func (e *processorContainerEmitter) EmitExited() {
	e.sendEvent(cevent{ceventExited, e.containerId, e.poolName, "", "", 0, 0, nil})
}
func (e *processorContainerEmitter) EmitActionAttempt(lname string,
	name string, attempt int, attempts int) {
	e.sendEvent(cevent{ceventAction, e.containerId, e.poolName, lname, name, attempt, attempts, nil})
}
func (e *processorContainerEmitter) EmitActionResult(lname string,
	name string, attempt int, attempts int, err error) {
	e.sendEvent(cevent{ceventResult, e.containerId, e.poolName, lname, name, attempt, attempts, err})
}
func (e *processorContainerEmitter) sendEvent(evt cevent) {
	e.processor.sendContainerEvent(evt)
}
