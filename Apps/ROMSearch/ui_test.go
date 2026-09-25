package main

import "testing"

func TestPickerDefersAndCancelsCatalog(t *testing.T) {
	a := &App{
		source:   2,
		systems:  systems[:2],
		page:     1,
		mode:     "home",
		messages: make(chan result, 4),
		cache:    make(map[string]catalogPage),
	}
	a.press("source")
	a.press("systemNext")
	if a.busy || a.loading || a.mode != "home" {
		t.Fatal("changing source or emulator started a request")
	}
	a.press("accept")
	if !a.busy || !a.loading {
		t.Fatal("A did not start browsing")
	}
	a.press("back")
	if a.busy || a.loading || a.mode != "home" {
		t.Fatal("B did not cancel browsing")
	}
	a.cache["RomsGames|PSP|1|"] = catalogPage{games: []Game{{Title: "cached"}}}
	a.source, a.system = 2, 0
	a.press("accept")
	if a.busy || len(a.games) != 1 || a.games[0].Title != "cached" {
		t.Fatal("visited page was fetched again")
	}
}
