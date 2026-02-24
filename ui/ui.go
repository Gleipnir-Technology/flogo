package ui

import (
	"context"
	"net/url"

	"github.com/Gleipnir-Technology/flogo/state"
)

type EventType int

const (
	EventNone EventType = iota
	EventExit
	EventResize
	EventUpdate // forcibly update clients

)

type Event struct {
	Type EventType
}
type UI interface {
	Close()
	Events() <-chan Event
	Run(context.Context, chan<- Event, <-chan *state.Flogo) error
}

func NewTUI(target string, upstream url.URL) (UI, error) {
	return newUITcell(target, upstream)
}

func NewFlat(target string, upstream url.URL) (UI, error) {
	return newUIFlat()
}
