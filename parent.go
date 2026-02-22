package main

import (
	"bytes"
	"os/exec"
)

type EventOutput interface {
}
type Parent struct {
	Child *exec.Cmd
	//OnOutput <- EventOutput
	Stdout bytes.Buffer
	Stderr bytes.Buffer
}

func (r *Parent) onStdout(b []byte) {
	r.Stdout.Write(b)
	r.Stdout.Write([]byte("\n"))
	/*
		r.OnEvent <- EventRunner{
			Buffer: r.Stdout.Bytes(),
			Type:   EventRunnerStdout,
		}
	*/
}
func (r *Parent) onStderr(b []byte) {
	r.Stderr.Write(b)
	r.Stderr.Write([]byte("\n"))
	/*
		r.OnEvent <- EventRunner{
			Buffer: r.Stderr.Bytes(),
			Type:   EventRunnerStderr,
		}
	*/
}
