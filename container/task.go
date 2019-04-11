package container

import "github.com/boz/ephemerald/lifecycle"

func runAction(a lifecycle.Action) <-chan error {
	ch := make(chan error, 1)
	return ch
}
