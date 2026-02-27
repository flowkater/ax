package codex

import (
	"context"
	"testing"
)

func TestScaffoldAdapterDeterministicThreadAndTurn(t *testing.T) {
	adapter := NewScaffoldAdapter()
	ctx := context.Background()

	thread, err := adapter.CreateThread(ctx, "plan-1")
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	if thread.ID != "scf-th-001" {
		t.Fatalf("unexpected thread id: %s", thread.ID)
	}

	turn, err := adapter.RunTurn(ctx, thread.ID, "step 1")
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if turn.ID != "turn-001" {
		t.Fatalf("unexpected turn id: %s", turn.ID)
	}
	if turn.ThreadID != thread.ID {
		t.Fatalf("unexpected turn thread: %s", turn.ThreadID)
	}
}

func TestScaffoldAdapterLifecycleMethods(t *testing.T) {
	adapter := NewScaffoldAdapter()
	ctx := context.Background()

	thread, err := adapter.CreateThread(ctx, "plan")
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	if _, err := adapter.ResumeSession(ctx, thread.ID); err != nil {
		t.Fatalf("ResumeSession: %v", err)
	}
	turn, err := adapter.RunTurn(ctx, thread.ID, "base")
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if _, err := adapter.SteerTurn(ctx, thread.ID, turn.ID, "tighten"); err != nil {
		t.Fatalf("SteerTurn: %v", err)
	}
	if _, err := adapter.InterruptTurn(ctx, thread.ID, turn.ID); err != nil {
		t.Fatalf("InterruptTurn: %v", err)
	}
	if _, err := adapter.ForkSession(ctx, thread.ID); err != nil {
		t.Fatalf("ForkSession: %v", err)
	}
	if _, err := adapter.RollbackTurns(ctx, thread.ID, turn.ID); err != nil {
		t.Fatalf("RollbackTurns: %v", err)
	}
}

func TestScaffoldAdapterInterruptTurnUnknownTurnReturnsError(t *testing.T) {
	adapter := NewScaffoldAdapter()
	ctx := context.Background()
	thread, err := adapter.CreateThread(ctx, "interrupt")
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	if _, err := adapter.InterruptTurn(ctx, thread.ID, "turn-missing"); err == nil {
		t.Fatal("expected unknown turn interrupt to fail")
	}
}

func TestScaffoldAdapterStreamTurn(t *testing.T) {
	adapter := NewScaffoldAdapter()
	ctx := context.Background()
	thread, err := adapter.CreateThread(ctx, "stream")
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	events, err := adapter.StreamTurn(ctx, thread.ID, "stream prompt")
	if err != nil {
		t.Fatalf("StreamTurn: %v", err)
	}
	var got []StreamEvent
	for evt := range events {
		got = append(got, evt)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	if got[0].Type != StreamEventDelta || got[1].Type != StreamEventCompleted {
		t.Fatalf("unexpected event types: %#v", got)
	}
}
