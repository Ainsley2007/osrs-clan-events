package discord

import (
	"testing"
)

func TestQueueMetricAutocompleteChoices_PrefixFilter(t *testing.T) {
	names := []string{"Vorkath", "Zulrah", "Nightmare"}
	all := queueMetricAutocompleteChoices(names, "")
	if len(all) != 3 {
		t.Fatalf("expected 3 choices, got %d", len(all))
	}

	filtered := queueMetricAutocompleteChoices(names, "vor")
	if len(filtered) != 1 || filtered[0].Name != "Vorkath" {
		t.Fatalf("expected Vorkath only, got %+v", filtered)
	}
}

func TestSelectionSourceLabel(t *testing.T) {
	if selectionSourceLabel(true) != "— queued" {
		t.Fatal("expected queued label")
	}
	if selectionSourceLabel(false) != "— random" {
		t.Fatal("expected random label")
	}
}
