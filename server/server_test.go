package server

import "testing"

func TestNew(t *testing.T) {
	t.Parallel()
	s := New(Config{
		Tools:    []string{"gmail"},
		ToolTier: "core",
	})
	if s == nil {
		t.Fatal("New() returned nil")
	}
}

func TestServerConstants(t *testing.T) {
	t.Parallel()
	if ServerName != "google" {
		t.Fatalf("ServerName = %q, want google", ServerName)
	}
	if ServerVersion == "" {
		t.Fatal("ServerVersion is empty")
	}
}
