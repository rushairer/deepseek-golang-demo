package agent

import "testing"

func TestParseFinal(t *testing.T) {
	result, err := parseFinal(`{"analysis":"ok","suggestions":["next"],"confidence":0.8}`)
	if err != nil {
		t.Fatal(err)
	}
	if result.Analysis != "ok" || result.Confidence != 0.8 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestParseFinalRejectsOutOfRangeConfidence(t *testing.T) {
	if _, err := parseFinal(`{"analysis":"ok","suggestions":[],"confidence":1.2}`); err == nil {
		t.Fatal("expected confidence validation error")
	}
}
