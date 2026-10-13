package session

import (
	"strings"
	"unicode/utf8"
)

// Preview ignores split CSI/OSC sequences and retains a bounded printable line.
type Preview struct {
	state   byte
	line    []rune
	pending []byte
	last    string
}

func (s *Preview) Feed(p []byte) string {
	var clean []byte
	for _, b := range p {
		switch s.state {
		case 0:
			if b == 27 {
				s.state = 1
			} else if b == '\n' || b == '\r' {
				s.consume(clean)
				clean = nil
				s.save()
				s.line = s.line[:0]
			} else if b >= 32 || b == '\t' || b >= 128 {
				clean = append(clean, b)
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
			if b == 7 {
				s.state = 0
			} else if b == 27 {
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
	s.consume(clean)
	s.save()
	return s.last
}
func (s *Preview) consume(p []byte) {
	s.pending = append(s.pending, p...)
	for len(s.pending) > 0 {
		if !utf8.FullRune(s.pending) {
			break
		}
		r, n := utf8.DecodeRune(s.pending)
		s.line = append(s.line, r)
		s.pending = s.pending[n:]
		if len(s.line) > 240 {
			s.line = s.line[len(s.line)-240:]
		}
	}
}
func (s *Preview) save() {
	text := strings.TrimSpace(string(s.line))
	if text != "" {
		r := []rune(text)
		s.last = string(r[:min(120, len(r))])
	}
}
