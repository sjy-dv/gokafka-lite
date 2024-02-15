package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
)

func main() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	sig := <-signals
	logrus.WithField("action", "shutdown").
		Info(fmt.Sprintf("Received signal: %s. Shutting down...\n", sig))
}
