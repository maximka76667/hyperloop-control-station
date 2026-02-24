package main

import (
	"io"

	"github.com/rs/zerolog"
)

type traceExitHandler struct {
	closer io.Closer
}

func (h traceExitHandler) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	if level == zerolog.FatalLevel || level == zerolog.PanicLevel {
		h.closer.Close() // This flushes everything before zerolog calls os.Exit(1)
	}
}
