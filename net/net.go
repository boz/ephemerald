package net

import "os"

const (
	rpcCheckoutName = "checkout"
	rpcReturnName   = "return"
)

// XXX: prevent koding/logging race condition

func init() {
	if os.Getenv("KITE_LOG_LEVEL") == "" {
		os.Setenv("KITE_LOG_LEVEL", "DEBUG")
	}
}
