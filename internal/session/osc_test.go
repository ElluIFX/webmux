package session

import (
	"strings"
	"testing"
)

func TestOSCChunkBoundaries(t *testing.T) {
	for _, sequence := range []string{"\x1b]0;  标题  \a", "\x1b]2;  标题  \x1b\\"} {
		for split := 0; split <= len(sequence); split++ {
			var scanner OSCScanner
			var titles []string
			save := func(s string) { titles = append(titles, s) }
			scanner.Feed([]byte(sequence[:split]), save)
			scanner.Feed([]byte(sequence[split:]), save)
			if len(titles) != 1 || titles[0] != "标题" {
				t.Fatalf("split %d: %v", split, titles)
			}
		}
	}
}
func TestOSCBoundsAndRecovery(t *testing.T) {
	var scanner OSCScanner
	var titles []string
	scanner.Feed([]byte("\x1b]52;ignored\a\x1b]2;"+strings.Repeat("x", 513)+"\a\x1b]0;ok\a"), func(s string) { titles = append(titles, s) })
	if len(titles) != 1 || titles[0] != "ok" {
		t.Fatal(titles)
	}
}
