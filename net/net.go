package net

import "os"

// XXX: prevent koding/logging race condition

func init() {
	if os.Getenv("KITE_LOG_LEVEL") == "" {
		os.Setenv("KITE_LOG_LEVEL", "DEBUG")
	}
}
