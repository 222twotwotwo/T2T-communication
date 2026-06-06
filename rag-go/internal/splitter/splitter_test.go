package splitter

import "testing"

func TestOverlapText(t *testing.T) {
	got := OverlapText("abcdefghij", 4, 1)
	want := []string{"abcd", "defg", "ghij"}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("chunk %d=%q want %q", i, got[i], want[i])
		}
	}
}
