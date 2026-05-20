package codec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStatePersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "codec_states.json")

	r1 := NewRegistry(DefaultExecutorConfig())
	r1.LoadDefaults()

	s1 := r1.GetOrCreateState("0011223344556677")
	s1.SetVariable("counter", float64(42))
	s1.SetVariable("name", "alpha")

	s2 := r1.GetOrCreateState("aabbccddeeff0011")
	s2.SetVariable("energy_kwh", 1234.5)

	if err := r1.SaveStates(path); err != nil {
		t.Fatalf("SaveStates: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("file is empty")
	}

	r2 := NewRegistry(DefaultExecutorConfig())
	if err := r2.LoadStates(path); err != nil {
		t.Fatalf("LoadStates: %v", err)
	}

	got1 := r2.GetOrCreateState("0011223344556677")
	if v := got1.GetVariable("counter"); v != float64(42) {
		t.Errorf("counter: got %v (%T), want 42", v, v)
	}
	if v := got1.GetVariable("name"); v != "alpha" {
		t.Errorf("name: got %v, want \"alpha\"", v)
	}

	got2 := r2.GetOrCreateState("aabbccddeeff0011")
	if v := got2.GetVariable("energy_kwh"); v != 1234.5 {
		t.Errorf("energy_kwh: got %v, want 1234.5", v)
	}

	r2.RemoveState("0011223344556677")
	if _, exists := r2.states["0011223344556677"]; exists {
		t.Error("state still present after RemoveState")
	}
}

func TestLoadStatesMissingFileIsNoError(t *testing.T) {
	r := NewRegistry(DefaultExecutorConfig())
	if err := r.LoadStates(filepath.Join(t.TempDir(), "missing.json")); err != nil {
		t.Errorf("missing file should not error: %v", err)
	}
}
