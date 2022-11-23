//go:build unix
package xic

import (
	"os/signal"
	"syscall"
)

func install_additional_signals(engine *_Engine) {
	signal.Notify(engine.sigChan, syscall.SIGTERM)
}

