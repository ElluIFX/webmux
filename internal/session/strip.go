package session

import (
	"strings"
	"unicode/utf8"
)

// Preview ignores split CSI/OSC sequences and retains a bounded printable line.
type Preview struct {
	state      byte
	line       []rune
	pending    [utf8.UTFMax]byte
	pendingLen int
	last       string
}

func (s *Preview) Feed(p []byte) string {
	for _, b := range p {
		switch s.state {
		case 0:
			if b == 27 {
				s.state = 1
			} else if b == '\n' || b == '\r' {
				s.pendingLen = 0
				s.save()
				s.line = s.line[:0]
			} else if b >= 32 || b == '\t' {
				s.consumeByte(b)
			}
		case 1:
			switch b {
			case '[':
				s.state = 2
			case ']', 'P', '^', '_', 'X':
				s.state = 3
			case '(', ')', '*', '+':
				s.state = 5
			default:
				s.state = 0
			}
		case 2:
			if b >= 0x40 && b <= 0x7e {
				s.state = 0
			}
		case 3:
			switch b {
			case 7:
				s.state = 0
			case 27:
				s.state = 4
			}
		case 4:
			if b == '\\' {
				s.state = 0
			} else {
				s.state = 3
			}
		case 5:
			s.state = 0
		}
	}
	s.save()
	return s.last
}
func (s *Preview) consumeByte(b byte) {
	if s.pendingLen == 0 && b < utf8.RuneSelf {
		s.appendRune(rune(b))
		return
	}
	s.pending[s.pendingLen] = b
	s.pendingLen++
	for s.pendingLen > 0 && utf8.FullRune(s.pending[:s.pendingLen]) {
		r, n := utf8.DecodeRune(s.pending[:s.pendingLen])
		s.appendRune(r)
		copy(s.pending[:], s.pending[n:s.pendingLen])
		s.pendingLen -= n
	}
}
func (s *Preview) appendRune(r rune) {
	if len(s.line) < 120 {
		s.line = append(s.line, r)
	}
}
func (s *Preview) save() {
	text := strings.TrimSpace(string(s.line))
	if text != "" {
		s.last = text
	}
}
