package session

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPreviewStreaming(t *testing.T) {
	var p Preview
	for _, row := range []struct{ chunk, want string }{
		{"first\r\nsec", "first\nsec"},
		{"ond", "first\nsecond"},
		{"\r\nthird", "first\nsecond\nthird"},
		{"\r\nfour", "second\nthird\nfour"},
		{"th", "second\nthird\nfourth"},
		{"\r\n\r\n \t\r\n", "second\nthird\nfourth"},
		{"", "second\nthird\nfourth"},
		{"fifth\r", "third\nfourth\nfifth"},
		{"\x1b[2Ksixth", "third\nfourth\nsixth"},
		{"\r\nsixth\r\n", "fourth\nsixth\nsixth"},
	} {
		if got := p.Feed([]byte(row.chunk)); got != row.want {
			t.Fatalf("chunk %q: preview = %q, want %q", row.chunk, got, row.want)
		}
	}
}

func TestPreviewChunkBoundaries(t *testing.T) {
	for _, row := range []struct{ name, input, want string }{
		{"rolling", "one\r\ntwo\r\nthree\r\nfour\r\nfive\r\n", "three\nfour\nfive"},
		{"repeated", "same\r\nsame\r\nsame\r\nsame\r\n", "same\nsame\nsame"},
		{"progress", "ready\r\n0%\r50%\r100%\r\nDone", "ready\n100%\nDone"},
		{"carriage return preserves suffix", "long progress\rOK", "OKng progress"},
		{"erase suffix", "long progress\rOK\x1b[K", "OK"},
		{"linefeed preserves column", "abc\nx", "abc\n   x"},
		{"tab and backspace", "ab\bX\tY", "aX      Y"},
		{"ansi unicode", "\x1b[32m中文预览\x1b[0m\r\n\x1b]2;hidden title\a继续?\r\n\x1b]0;hidden\x1b\\🙂好", "中文预览\n继续?\n🙂好"},
		{"combining across writes", "e\u0301x", "e\u0301x"},
		{"cursor redraw", "context\r\nold activity\r\nold status\x1b[1F\x1b[2Knew activity\x1b[1E\x1b[2Knew status", "context\nnew activity\nnew status"},
		{"saved cursor", "first\r\nsecond\x1b7\x1b[1;1Hnew\x1b[K\x1b8!", "new\nsecond!"},
	} {
		t.Run(row.name, func(t *testing.T) {
			input := []byte(row.input)
			for split := 0; split <= len(input); split++ {
				var p Preview
				p.Feed(input[:split])
				if got := p.Feed(input[split:]); got != row.want {
					t.Fatalf("split %d: preview = %q, want %q", split, got, row.want)
				}
			}
			var p Preview
			for _, b := range input {
				p.Feed([]byte{b})
			}
			if got := p.Feed(nil); got != row.want {
				t.Fatalf("byte chunks: preview = %q, want %q", got, row.want)
			}
		})
	}
}

func TestPreviewLineBounds(t *testing.T) {
	first := strings.Repeat("界", 120)
	second := strings.Repeat("🙂", 120)
	third := strings.Repeat("z", 120)
	input := first + "discarded\r\n" + second + "discarded\r\n" + third + "discarded"
	want := first + "\n" + second + "\n" + third
	for _, chunkSize := range []int{1, 7, len(input)} {
		var p Preview
		p.Resize(300, 24)
		for start := 0; start < len(input); start += chunkSize {
			p.Feed([]byte(input[start:min(start+chunkSize, len(input))]))
		}
		got := p.Feed(nil)
		if got != want {
			t.Fatalf("chunk size %d: preview = %q, want %q", chunkSize, got, want)
		}
		for _, line := range strings.Split(got, "\n") {
			if !utf8.ValidString(line) || utf8.RuneCountInString(line) != 120 {
				t.Fatalf("invalid bounded line: %q", line)
			}
		}
		if got := p.Feed([]byte("\r\x1b[2Kshort\r\nnext")); got != second+"\nshort\nnext" {
			t.Fatalf("rewrite after truncation = %q", got)
		}
	}
}

func TestPreviewSnapshotsRemainIndependent(t *testing.T) {
	var s Session
	s.info.Preview = s.preview.Feed([]byte("one\r\ntwo\r\nthree"))
	previous := s.Info()
	s.info.Preview = s.preview.Feed([]byte(" more\r\nfour\r\nfive"))
	current := s.Info()
	if previous.Preview != "one\ntwo\nthree" {
		t.Fatalf("previous snapshot changed: %q", previous.Preview)
	}
	if current.Preview != "three more\nfour\nfive" {
		t.Fatalf("current snapshot = %q", current.Preview)
	}
	if sameDisplay(previous, current) {
		t.Fatal("preview change was invisible")
	}
	if !sameDisplay(current, s.Info()) {
		t.Fatal("unchanged preview did not compare equal")
	}
}

func TestPreviewCursorRedraw(t *testing.T) {
	var p Preview
	p.Feed([]byte("context\r\nold activity\r\nold status"))
	got := p.Feed([]byte("\x1b[1F\x1b[2Knew activity\x1b[1E\x1b[2Knew status"))
	if want := "context\nnew activity\nnew status"; got != want {
		t.Fatalf("redrawn screen = %q, want %q", got, want)
	}
	if got := p.Feed([]byte("\x1b[2J\x1b[H")); got != "" || p.last != "" {
		t.Fatalf("cleared screen retained stale output: %q, last %q", got, p.last)
	}
}

func TestPreviewAlternateScreen(t *testing.T) {
	var p Preview
	p.Feed([]byte("shell prompt"))
	if got := p.Feed([]byte("\x1b[?1049h\x1b[Heditor\r\nstatus")); got != "editor\nstatus" {
		t.Fatalf("alternate screen = %q", got)
	}
	if got := p.Feed([]byte("\x1b[?1049l")); got != "shell prompt" {
		t.Fatalf("restored primary screen = %q", got)
	}
}

func TestPreviewWrapScrollAndResize(t *testing.T) {
	var p Preview
	p.Resize(4, 3)
	if got := p.Feed([]byte("界界abcdXYZ")); got != "界界\nabcd\nXYZ" {
		t.Fatalf("wide character wrapping = %q", got)
	}
	if got := p.Feed([]byte("\r\nend")); got != "abcd\nXYZ\nend" {
		t.Fatalf("scrolled screen = %q", got)
	}
	p.Resize(8, 4)
	if got := p.Feed([]byte("\x1b[2J\x1b[H12345678Z")); got != "12345678\nZ" {
		t.Fatalf("resized screen = %q", got)
	}
}

func TestPreviewQueriesDoNotBlockOrAppearInOutput(t *testing.T) {
	var p Preview
	// Query responses are intentionally left without a callback. Feed and
	// Resize must finish without sending input or requiring a reader goroutine.
	p.Feed([]byte("before\x1b[6n\x1b[c\x1b]10;?\a\x1b[?2048h"))
	p.Resize(100, 30)
	if got := p.Feed([]byte("\r\nafter")); got != "before\nafter" {
		t.Fatalf("device queries changed visible output: %q", got)
	}
}
