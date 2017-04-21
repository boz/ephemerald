package ui

import (
	"fmt"
	"io"
)

type writer interface {
	updatePool(pool)

	updateContainer(container)
	deleteContainer(container)

	stop()
}

type ioWriter struct {
	w io.Writer
}

func newIOWriter(w io.Writer) writer {
	return &ioWriter{w}
}

func (w *ioWriter) updatePool(p pool) {
	w.writePool(p)
}

func (w *ioWriter) updateContainer(c container) {
	w.writeContainer(c)
}

func (w *ioWriter) deleteContainer(c container) {
}

func (w *ioWriter) writePool(p pool) {
	fmt.Fprintf(w.w, "[POOL]      %v %v items: %v ready: %v  pending: %v\n", p.name, p.state, p.numItems, p.numReady, p.numPending)
}

func (w *ioWriter) writeContainer(c container) {
	fmt.Fprintf(w.w, "[CONTAINER] %v %v %v %v %v %v\n", c.pname, w.cid(c.id), c.state, c.actionName, c.actionAttempt, c.actionError)
}

func (w *ioWriter) stop() {
}

func (w *ioWriter) cid(cid string) string {
	return cid[0:12]
}
