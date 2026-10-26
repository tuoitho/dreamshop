package main

import (
	"context"
	"errors"
	"testing"
)

func TestSeedIfEnabled(t *testing.T) {
	called := false
	err := seedIfEnabled(context.Background(), false, func(context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("seedIfEnabled(disabled) error = %v", err)
	}
	if called {
		t.Fatal("expected seed not to run when disabled")
	}

	wantErr := errors.New("seed failure")
	err = seedIfEnabled(context.Background(), true, func(context.Context) error {
		called = true
		return wantErr
	})
	if !called {
		t.Fatal("expected seed to run when enabled")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("seedIfEnabled(enabled) error = %v, want %v", err, wantErr)
	}
}
