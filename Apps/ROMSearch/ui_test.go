package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMixedFeaturedListIsReadyBeforeNetwork(t *testing.T) {
	root := t.TempDir()
	romRoot := filepath.Join(root, "Roms")
	for _, folder := range []string{"PSP", "GBA"} {
		if e := os.MkdirAll(filepath.Join(romRoot, folder), 0755); e != nil {
			t.Fatal(e)
		}
		emu := filepath.Join(root, "Emus", folder)
		if e := os.MkdirAll(emu, 0755); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(emu, "config.json"), []byte(`{"rompath":"../../Roms/`+folder+`"}`), 0644); e != nil {
			t.Fatal(e)
		}
	}
	t.Setenv("ROM_SEARCH_ROOT", romRoot)
	a := newApp()
	if a.busy || a.loading || len(a.games) < 8 {
		t.Fatalf("featured list not ready: busy=%v games=%d", a.busy, len(a.games))
	}
	seen := map[string]bool{}
	for _, g := range a.games {
		seen[g.System] = true
	}
	if !seen["PSP"] || !seen["GBA"] {
		t.Fatalf("featured list does not mix installed systems: %v", seen)
	}
	a.press("source")
	if !a.loading || len(a.games) != 0 {
		t.Fatal("source switch should show loading without games from the previous source")
	}
	a.press("source")
	a.press("source")
	if a.loading || a.busy || len(a.games) < 8 {
		t.Fatal("returning to the cached featured list blocked the UI")
	}
}

func TestBrickProSelectOpensKeyboardAndStartExits(t *testing.T) {
	for _, mapping := range []struct {
		kind, selectButton, startButton uint32
	}{
		{0x651, 4, 6},
		{0x603, 6, 7},
	} {
		if got := buttonAction(mapping.kind, uint8(mapping.selectButton)); got != "edit" {
			t.Fatalf("SELECT mapped to %q", got)
		}
		if got := buttonAction(mapping.kind, uint8(mapping.startButton)); got != "quit" {
			t.Fatalf("START mapped to %q", got)
		}
	}
	a := &App{running: true, mode: "results"}
	a.press("edit")
	if a.mode != "keyboard" {
		t.Fatal("SELECT did not open keyboard")
	}
	a.press("quit")
	if a.running {
		t.Fatal("START did not end the app")
	}
}
