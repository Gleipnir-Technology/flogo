package main

type flogoState struct {
	builderStatus    builderStatus
	lastBuildOutput  string
	lastBuildSuccess bool
	lastRunStdout    []byte
	lastRunStderr    []byte
	runnerStatus     runnerStatus
}
type builderStatus int

const (
	builderStatusCompiling builderStatus = iota
	builderStatusFailed
	builderStatusOK
)

type runnerStatus int

const (
	runnerStatusRunning runnerStatus = iota
	runnerStatusStopOK
	runnerStatusStopErr
	runnerStatusWaiting
)

func newFlogoState() flogoState {
	return flogoState{
		lastBuildOutput:  "",
		lastBuildSuccess: false,
		lastRunStdout:    []byte{},
		lastRunStderr:    []byte{},
		runnerStatus:     runnerStatusWaiting,
	}
}
