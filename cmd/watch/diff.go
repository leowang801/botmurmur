// Package watch implements the `botmurmur watch` long-running poll loop and
// the snapshot diff that turns successive scans into ADDED/STOPPED events.
//
// The diff identity is (PID, StartTime), not PID alone. PIDs are recycled by
// every modern OS — without StartTime, a recycled PID would silently look like
// "the same agent is still running" when in fact the original exited and an
// unrelated process took its slot. StartTime is immutable for the life of a
// process and pins the identity across the lifetime we care about.
package watch

import (
	"github.com/leowang801/botmurmur/internal/output"
)

// EventKind enumerates the diff event types emitted by Diff.
type EventKind string

const (
	// EventAdded means an agent appeared in curr that was not in prev.
	EventAdded EventKind = "added"
	// EventStopped means an agent was in prev but is missing from curr.
	EventStopped EventKind = "stopped"
)

// Event is a single change between two snapshots.
type Event struct {
	Kind  EventKind
	Agent output.Agent
}

// agentKey is the snapshot diff identity. Using (PID, StartTime) instead of
// PID alone defeats PID reuse: when the OS recycles a PID, StartTime changes
// and we correctly emit STOPPED for the old process and ADDED for the new one.
type agentKey struct {
	pid       int
	startTime string
}

func keyOf(a output.Agent) agentKey {
	return agentKey{pid: a.PID, startTime: a.StartTime}
}

// Diff returns the events needed to go from prev to curr. ADDED events come
// first (in curr order), then STOPPED events (in prev order). The order is
// stable so tests can compare slices without sorting.
func Diff(prev, curr []output.Agent) []Event {
	prevByKey := make(map[agentKey]struct{}, len(prev))
	for _, a := range prev {
		prevByKey[keyOf(a)] = struct{}{}
	}
	currByKey := make(map[agentKey]struct{}, len(curr))
	for _, a := range curr {
		currByKey[keyOf(a)] = struct{}{}
	}

	var events []Event
	for _, a := range curr {
		if _, ok := prevByKey[keyOf(a)]; !ok {
			events = append(events, Event{Kind: EventAdded, Agent: a})
		}
	}
	for _, a := range prev {
		if _, ok := currByKey[keyOf(a)]; !ok {
			events = append(events, Event{Kind: EventStopped, Agent: a})
		}
	}
	return events
}
