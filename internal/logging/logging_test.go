package logging

import (
	"strings"
	"testing"
)

func TestHighlight(t *testing.T) {
	plain := `[Guild 1] SOTW selection
  weights (total 9.750000):
    [ 7] Crafting  <- picked
  roll: 4.2 -> "Crafting" (week 23)`

	got := highlight(plain, false)
	if got != plain {
		t.Fatal("highlight disabled should return input unchanged")
	}

	got = highlight(plain, true)
	if got == plain {
		t.Fatal("highlight enabled should add ANSI color codes")
	}
	if !strings.Contains(got, "\033[") {
		t.Fatalf("expected ANSI escape in highlighted output:\n%s", got)
	}
}
