// traceCloser joins the async writer and the file into one "closable" object
package main

import (
	"io"
	"os"
)

type traceCloser struct {
	file        *os.File
	asyncWriter io.WriteCloser
}

// Close implements the io.WriteCloser interface
func (c *traceCloser) Close() error {
	// 1. Close the async writer first to "drain" the buffer into the file
	errAsync := c.asyncWriter.Close()

	// 2. Close the file handle so the OS saves it to disk
	errFile := c.file.Close()

	if errAsync != nil {
		return errAsync
	}
	return errFile
}
