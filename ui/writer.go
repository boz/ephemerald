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
	ioPool(p).Print(w.w)
}

func (w *ioWriter) writeContainer(c container) {
	ioContainer(c).Print(w.w)
}

func (w *ioWriter) stop() {
}

const (
	ioPoolPrefix      = "[POOL]      "
	ioContainerPrefix = "[CONTAINER] "
)

type ioPool pool

func (p ioPool) Print(w io.Writer) {
	fmt.Fprint(w, ioPoolPrefix)
	fmt.Fprintf(w, "%v %v items: %v ready: %v  pending: %v\n", p.name, p.state, p.numItems, p.numReady, p.numPending)
}

type ioContainer container

func (c ioContainer) Print(w io.Writer) {
	fmt.Fprint(w, ioContainerPrefix)
	fmt.Fprintf(w, "%v %v %v", c.pname, c.id[0:12], c.state)

	if c.actionName == "" {
		goto done
	}

	fmt.Fprintf(w, " %v [%v/%v]", c.actionName, c.actionAttempt, c.actionAttempts)

	if c.actionError == nil {
		goto done
	}

	fmt.Fprintf(w, " %v", c.actionError)

done:
	fmt.Fprintln(w)
}
