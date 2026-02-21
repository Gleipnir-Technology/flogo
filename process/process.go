package process

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

type Process struct {
	// Contains the exit code after the program exits
	ExitCode *int
	// Contains all of stdeer that has been emitted by this process
	Stderr bytes.Buffer
	// Contains all of stdout that has been emitted by this process
	Stdout bytes.Buffer

	args     []string
	cmd      *exec.Cmd
	onExit   chan *os.ProcessState
	onStart  chan struct{}
	onStderr chan []byte
	onStdout chan []byte
	target   string
}

func New(target string, args ...string) *Process {
	return &Process{
		ExitCode: nil,
		Stdout:   bytes.Buffer{},
		Stderr:   bytes.Buffer{},
		args:     args,
		cmd:      nil,
		onExit:   make(chan *os.ProcessState, 1),
		onStart:  make(chan struct{}, 1),
		onStderr: make(chan []byte, 10),
		onStdout: make(chan []byte, 10),
		target:   target,
	}
}

// Restart the process.
// If the process is running, signals the process to exit, waits for exit,
// then calls "Start"
// Provides an error if the process is already running
func (p *Process) Restart(ctx context.Context) error {
	if p.cmd != nil {
		p.SignalInterrupt()
	}
	return nil
}
func (p *Process) Signal(s syscall.Signal) error {
	if p.cmd == nil {
		return fmt.Errorf("cmd is nil")
	}
	return p.cmd.Process.Signal(s)
}
func (p *Process) SignalInterrupt() {
	p.Signal(syscall.SIGINT)
}
func (p *Process) Start(ctx context.Context) error {
	p.Stdout.Reset()
	p.Stderr.Reset()
	if _, err := os.Stat(p.target); os.IsNotExist(err) {
		return fmt.Errorf("Target program '%s' does not exist", p.target)
	}
	// Create the command
	p.cmd = exec.Command(p.target)

	// Get a pipe for stdout
	stdout, err := p.cmd.StdoutPipe()
	if err != nil {
		p.cmd = nil
		return errors.New("Failed to get stdout pipe")
	}

	// get stderr too
	stderr, err := p.cmd.StderrPipe()
	if err != nil {
		p.cmd = nil
		return errors.New("Failed to get stderr pipe")
	}

	// Start the command (non-blocking)
	if err := p.cmd.Start(); err != nil {
		p.cmd = nil
		return fmt.Errorf("Failed to start '%s': %w", p.target, err)
	}

	// Read stdout line by line
	scanner := bufio.NewScanner(stdout)
	go func() {
		for scanner.Scan() {
			b := scanner.Bytes()
			//fmt.Printf("stdout: %s\n", string(b))
			p.onStdout <- b
		}
	}()

	// Read stderr line by line
	stderrScanner := bufio.NewScanner(stderr)
	go func() {
		for stderrScanner.Scan() {
			b := stderrScanner.Bytes()
			//fmt.Printf("stderr: %s\n", string(b))
			p.onStderr <- b
		}
	}()

	// Wait for the command to finish
	go func() {
		s, err := p.cmd.Process.Wait()
		if err != nil {
			fmt.Printf("Got error on cmd.Wait(): %v\n", err)
		}
		p.onExit <- s
		p.cmd = nil
	}()
	p.onStart <- struct{}{}
	return nil
}
func (p *Process) OnStart() <-chan struct{} {
	return p.onStart
}
func (p *Process) OnExit() <-chan *os.ProcessState {
	return p.onExit
}
func (p *Process) OnStderr() <-chan []byte {
	return p.onStderr
}
func (p *Process) OnStdout() <-chan []byte {
	return p.onStdout
}
