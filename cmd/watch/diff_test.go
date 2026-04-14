package watch

import (
	"testing"

	"github.com/leowang801/botmurmur/internal/output"
)

// agent is a tiny constructor for tests — only the fields the diff cares about.
func agent(pid int, start, name string) output.Agent {
	return output.Agent{
		PID:        pid,
		Name:       name,
		StartTime:  start,
		Frameworks: []string{"claude-code"},
	}
}

func TestDiff_Empty(t *testing.T) {
	got := Diff(nil, nil)
	if len(got) != 0 {
		t.Errorf("expected no events, got %d", len(got))
	}
}

func TestDiff_AllAdded(t *testing.T) {
	curr := []output.Agent{
		agent(100, "2026-04-11T10:00:00Z", "claude"),
		agent(200, "2026-04-11T10:01:00Z", "cursor"),
	}
	got := Diff(nil, curr)
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	for i, e := range got {
		if e.Kind != EventAdded {
			t.Errorf("event %d: kind = %s, want added", i, e.Kind)
		}
	}
	if got[0].Agent.PID != 100 || got[1].Agent.PID != 200 {
		t.Errorf("ADDED order should match curr slice order, got %d then %d",
			got[0].Agent.PID, got[1].Agent.PID)
	}
}

func TestDiff_AllStopped(t *testing.T) {
	prev := []output.Agent{
		agent(100, "2026-04-11T10:00:00Z", "claude"),
		agent(200, "2026-04-11T10:01:00Z", "cursor"),
	}
	got := Diff(prev, nil)
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	for i, e := range got {
		if e.Kind != EventStopped {
			t.Errorf("event %d: kind = %s, want stopped", i, e.Kind)
		}
	}
}

func TestDiff_NoChange(t *testing.T) {
	snap := []output.Agent{
		agent(100, "2026-04-11T10:00:00Z", "claude"),
		agent(200, "2026-04-11T10:01:00Z", "cursor"),
	}
	// Same processes, same start times — diff must be empty.
	got := Diff(snap, snap)
	if len(got) != 0 {
		t.Errorf("expected no events for unchanged snapshot, got %d: %+v",
			len(got), got)
	}
}

// TestDiff_PIDReuse is the load-bearing test for the (PID, start_time) key.
// Without StartTime, this would look like "pid 100 still running" and we'd
// silently miss the agent transition. With StartTime, the recycled PID becomes
// a STOPPED + ADDED pair.
func TestDiff_PIDReuse(t *testing.T) {
	prev := []output.Agent{
		agent(100, "2026-04-11T10:00:00Z", "claude-code"),
	}
	curr := []output.Agent{
		// Same PID, different start time — OS recycled the slot for an
		// unrelated process.
		agent(100, "2026-04-11T10:05:00Z", "cursor"),
	}
	got := Diff(prev, curr)
	if len(got) != 2 {
		t.Fatalf("expected 2 events for PID reuse, got %d: %+v", len(got), got)
	}

	var sawAdded, sawStopped bool
	for _, e := range got {
		switch e.Kind {
		case EventAdded:
			sawAdded = true
			if e.Agent.StartTime != "2026-04-11T10:05:00Z" {
				t.Errorf("ADDED event has wrong start time: %s", e.Agent.StartTime)
			}
			if e.Agent.Name != "cursor" {
				t.Errorf("ADDED event has wrong name: %s", e.Agent.Name)
			}
		case EventStopped:
			sawStopped = true
			if e.Agent.StartTime != "2026-04-11T10:00:00Z" {
				t.Errorf("STOPPED event has wrong start time: %s", e.Agent.StartTime)
			}
			if e.Agent.Name != "claude-code" {
				t.Errorf("STOPPED event has wrong name: %s", e.Agent.Name)
			}
		}
	}
	if !sawAdded || !sawStopped {
		t.Errorf("PID reuse must produce both ADDED and STOPPED, got added=%v stopped=%v",
			sawAdded, sawStopped)
	}
}

func TestDiff_MixedChurn(t *testing.T) {
	prev := []output.Agent{
		agent(100, "2026-04-11T10:00:00Z", "claude"),  // stays
		agent(200, "2026-04-11T10:01:00Z", "cursor"),  // stops
		agent(300, "2026-04-11T10:02:00Z", "lcagent"), // stays
	}
	curr := []output.Agent{
		agent(100, "2026-04-11T10:00:00Z", "claude"),  // stays
		agent(300, "2026-04-11T10:02:00Z", "lcagent"), // stays
		agent(400, "2026-04-11T10:10:00Z", "newone"),  // added
	}
	got := Diff(prev, curr)
	if len(got) != 2 {
		t.Fatalf("expected 2 events (1 added, 1 stopped), got %d: %+v", len(got), got)
	}
	// ADDED comes first by contract.
	if got[0].Kind != EventAdded || got[0].Agent.PID != 400 {
		t.Errorf("event[0] should be ADDED pid=400, got %+v", got[0])
	}
	if got[1].Kind != EventStopped || got[1].Agent.PID != 200 {
		t.Errorf("event[1] should be STOPPED pid=200, got %+v", got[1])
	}
}
