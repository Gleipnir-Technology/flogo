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
	"time"
)

type Process struct {
	// Contains the exit code after the program exits
	ExitCode *int
	OnOutput *SubscriptionManager[[]byte]
	OnStderr *SubscriptionManager[[]byte]
	OnStdout *SubscriptionManager[[]byte]
	OnExit   *SubscriptionManager[*os.ProcessState]
	OnStart  *SubscriptionManager[struct{}]
	// Contains the interleaved total output emitted by this process
	Output bytes.Buffer
	// Contains all of stdeer that has been emitted by this process
	Stderr bytes.Buffer
	// Contains all of stdout that has been emitted by this process
	Stdout bytes.Buffer

	args       []string
	chanExit   chan *os.ProcessState
	chanStart  chan struct{}
	chanStderr chan []byte
	chanStdout chan []byte
	cmd        *exec.Cmd
	dir        string
	target     string
}

func New(target string, args ...string) *Process {
	return &Process{
		ExitCode:   nil,
		OnExit:     NewSubscriptionManager[*os.ProcessState](),
		OnOutput:   NewSubscriptionManager[[]byte](),
		OnStart:    NewSubscriptionManager[struct{}](),
		OnStderr:   NewSubscriptionManager[[]byte](),
		OnStdout:   NewSubscriptionManager[[]byte](),
		Stdout:     bytes.Buffer{},
		Stderr:     bytes.Buffer{},
		args:       args,
		chanExit:   make(chan *os.ProcessState, 1),
		chanStart:  make(chan struct{}, 1),
		chanStderr: make(chan []byte, 10),
		chanStdout: make(chan []byte, 10),
		cmd:        nil,
		target:     target,
	}
}

// Restart the process.
// If the process is running, signals the process to exit, waits for exit,
// then calls "Start"
// Provides an error if the process is already running
func (p *Process) Restart(ctx context.Context) error {
	p.Stop()
	return p.Start(ctx)
}
func (p *Process) SetDir(d string) {
	p.dir = d
	if p.cmd != nil {
		p.cmd.Dir = d
	}
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
	p.Output.Reset()
	p.Stdout.Reset()
	p.Stderr.Reset()
	// Create the command
	p.cmd = exec.Command(p.target, p.args...)

	if p.dir != "" {
		p.cmd.Dir = p.dir
	}
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

	// Read stdout line by line
	scanner := bufio.NewScanner(stdout)
	go func() {
		for scanner.Scan() {
			b := scanner.Bytes()
			p.onStream(p.OnStdout, &p.Stdout, p.chanStdout, b)
		}
	}()

	// Read stderr line by line
	stderrScanner := bufio.NewScanner(stderr)
	go func() {
		for stderrScanner.Scan() {
			b := stderrScanner.Bytes()
			p.onStream(p.OnStderr, &p.Stderr, p.chanStderr, b)
		}
	}()

	// Start the command (non-blocking)
	if err := p.cmd.Start(); err != nil {
		p.cmd = nil
		return fmt.Errorf("Failed to start '%s': %w", p.target, err)
	}

	// Wait for the command to finish
	go func() {
		s, err := p.cmd.Process.Wait()
		if err != nil {
			fmt.Printf("Got error on cmd.Wait(): %v\n", err)
		}
		select {
		case p.chanExit <- s:
		default:
		}
		p.OnExit.Publish(s)
		p.cmd = nil
	}()
	select {
	case p.chanStart <- struct{}{}:
	default:
	}
	p.OnStart.Publish(struct{}{})
	return nil
}

// Signal the process to stop. Wait for it to complete, or for 3 seconds to pass, then
// actively kill. This function does not return until the child is dead
func (p *Process) Stop() {
	if p.cmd != nil {
		p.SignalInterrupt()
	}
	select {
	case <-p.chanExit:
	case <-time.After(time.Second * 3):
		if p.cmd != nil {
			p.Signal(syscall.SIGKILL)
		}
	}
}
func (p *Process) onStream(mgr *SubscriptionManager[[]byte], buf *bytes.Buffer, c chan<- []byte, b []byte) {
	buf.Write(b)
	buf.Write([]byte("\n"))
	p.Output.Write(b)
	p.Output.Write([]byte("\n"))
	select {
	case c <- b:
	default:
	}
	mgr.Publish(b)
	p.OnOutput.Publish(b)
}
