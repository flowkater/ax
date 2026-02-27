package codex

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ScaffoldAdapter provides deterministic in-process AppServerAdapter behavior.
type ScaffoldAdapter struct {
	mu         sync.Mutex
	threadSeq  int
	turnSeq    int
	threads    map[string]*Thread
	turnByID   map[string]*Turn
	lastTurnBy map[string]string
}

// NewScaffoldAdapter creates a deterministic adapter for scaffold mode.
func NewScaffoldAdapter() *ScaffoldAdapter {
	return &ScaffoldAdapter{
		threads:    map[string]*Thread{},
		turnByID:   map[string]*Turn{},
		lastTurnBy: map[string]string{},
	}
}

func (s *ScaffoldAdapter) CreateThread(_ context.Context, title string) (*Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.threadSeq++
	id := fmt.Sprintf("scf-th-%03d", s.threadSeq)
	thread := &Thread{
		ID:        id,
		Title:     title,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	s.threads[id] = thread
	return cloneThread(thread), nil
}

func (s *ScaffoldAdapter) RunTurn(_ context.Context, threadID, prompt string) (*Turn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.threads[threadID]; !ok {
		s.threads[threadID] = &Thread{
			ID:        threadID,
			Title:     "rollback-source",
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		}
	}
	s.turnSeq++
	id := fmt.Sprintf("turn-%03d", s.turnSeq)
	turn := &Turn{
		ID:        id,
		ThreadID:  threadID,
		Role:      "assistant",
		Content:   "scaffold:" + prompt,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	s.turnByID[id] = turn
	s.lastTurnBy[threadID] = id
	return cloneTurn(turn), nil
}

func (s *ScaffoldAdapter) GetThread(_ context.Context, threadID string) (*Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	thread, ok := s.threads[threadID]
	if !ok {
		thread = &Thread{
			ID:        threadID,
			Title:     "restored",
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		s.threads[threadID] = thread
	}
	return cloneThread(thread), nil
}

func (s *ScaffoldAdapter) ResumeSession(ctx context.Context, threadID string) (*Thread, error) {
	return s.GetThread(ctx, threadID)
}

func (s *ScaffoldAdapter) ForkSession(_ context.Context, threadID string) (*Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	orig, ok := s.threads[threadID]
	if !ok {
		orig = &Thread{
			ID:        threadID,
			Title:     "fork-source",
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		s.threads[threadID] = orig
	}
	s.threadSeq++
	id := fmt.Sprintf("scf-th-%03d", s.threadSeq)
	thread := &Thread{
		ID:        id,
		Title:     "fork:" + orig.Title,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	s.threads[id] = thread
	return cloneThread(thread), nil
}

func (s *ScaffoldAdapter) RollbackTurns(_ context.Context, threadID, toTurnID string) (*Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.threads[threadID]; !ok {
		return nil, fmt.Errorf("AX_ENGINE_THREAD_NOT_FOUND: %s", threadID)
	}
	if toTurnID != "" {
		if _, ok := s.turnByID[toTurnID]; !ok {
			return nil, fmt.Errorf("AX_ENGINE_TURN_NOT_FOUND: %s", toTurnID)
		}
		s.lastTurnBy[threadID] = toTurnID
	}
	return cloneThread(s.threads[threadID]), nil
}

func (s *ScaffoldAdapter) SteerTurn(_ context.Context, threadID, turnID, instruction string) (*Turn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.threads[threadID]; !ok {
		return nil, fmt.Errorf("AX_ENGINE_THREAD_NOT_FOUND: %s", threadID)
	}
	if turnID != "" {
		if _, ok := s.turnByID[turnID]; !ok {
			return nil, fmt.Errorf("AX_ENGINE_TURN_NOT_FOUND: %s", turnID)
		}
	}
	s.turnSeq++
	id := fmt.Sprintf("turn-%03d", s.turnSeq)
	turn := &Turn{
		ID:        id,
		ThreadID:  threadID,
		Role:      "assistant",
		Content:   "steered:" + instruction,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	s.turnByID[id] = turn
	s.lastTurnBy[threadID] = id
	return cloneTurn(turn), nil
}

func (s *ScaffoldAdapter) InterruptTurn(_ context.Context, threadID, turnID string) (*InterruptResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.threads[threadID]; !ok {
		return nil, fmt.Errorf("AX_ENGINE_THREAD_NOT_FOUND: %s", threadID)
	}
	if turnID != "" {
		if _, ok := s.turnByID[turnID]; !ok {
			return nil, fmt.Errorf("AX_ENGINE_TURN_NOT_FOUND: %s", turnID)
		}
	}
	return &InterruptResult{Interrupted: true, ThreadID: threadID, TurnID: turnID}, nil
}

func (s *ScaffoldAdapter) StreamTurn(ctx context.Context, threadID, prompt string) (<-chan StreamEvent, error) {
	turn, err := s.RunTurn(ctx, threadID, prompt)
	if err != nil {
		return nil, err
	}
	out := make(chan StreamEvent, 2)
	out <- StreamEvent{
		Type:     StreamEventDelta,
		ThreadID: threadID,
		TurnID:   turn.ID,
		Delta:    "scaffold",
	}
	out <- StreamEvent{
		Type:      StreamEventCompleted,
		ThreadID:  threadID,
		TurnID:    turn.ID,
		Completed: true,
	}
	close(out)
	return out, nil
}

func cloneThread(in *Thread) *Thread {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneTurn(in *Turn) *Turn {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}
