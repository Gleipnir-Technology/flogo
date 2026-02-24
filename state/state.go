package state

type Flogo struct {
	Builder *Builder
	Runner  *Runner
}
type Process struct {
	ExitCode *int
	Output   []byte
	Stderr   []byte
	Stdout   []byte
}
type StatusBuilder int

const (
	StatusBuilderCompiling StatusBuilder = iota
	StatusBuilderFailed
	StatusBuilderOK
)

type Builder struct {
	BuildPrevious *Process
	BuildCurrent  *Process
	Status        StatusBuilder
}
type Runner struct {
	RunPrevious *Process
	RunCurrent  *Process
	Status      StatusRunner
}
type StatusRunner int

const (
	StatusRunnerRunning StatusRunner = iota
	StatusRunnerStopOK
	StatusRunnerStopErr
	StatusRunnerWaiting
)

func StatusStringBuilder(s StatusBuilder) string {
	switch s {
	case StatusBuilderCompiling:
		return "compiling"
	case StatusBuilderFailed:
		return "failed"
	case StatusBuilderOK:
		return "ok"
	}
	return "unknown"
}

func StatusStringRunner(s StatusRunner) string {
	switch s {
	case StatusRunnerRunning:
		return "running"
	case StatusRunnerStopOK:
		return "ok"
	case StatusRunnerStopErr:
		return "error"
	case StatusRunnerWaiting:
		return "waiting"
	}
	return "unknown"
}
