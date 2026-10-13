package session

import "strings"

// OSCScanner is incremental and bounded, including across split ST terminators.
type OSCScanner struct {
	state    byte
	buf      []byte
	overflow bool
}

func (s *OSCScanner) Feed(p []byte, title func(string)) {
	for _, b := range p {
		switch s.state {
		case 0:
			if b == 27 {
				s.state = 1
			}
		case 1:
			if b == ']' {
				s.state = 2
				s.buf = s.buf[:0]
				s.overflow = false
			} else if b != 27 {
				s.state = 0
			}
		case 2:
			if b == 7 {
				s.finish(title)
			} else if b == 27 {
				s.state = 3
			} else {
				s.append(b)
			}
		case 3:
			if b == '\\' {
				s.finish(title)
			} else {
				s.append(27)
				s.append(b)
				s.state = 2
			}
		}
	}
}
func (s *OSCScanner) append(b byte) {
	if len(s.buf) < 512 {
		s.buf = append(s.buf, b)
	} else {
		s.overflow = true
	}
}
func (s *OSCScanner) finish(title func(string)) {
	if !s.overflow {
		text := string(s.buf)
		if strings.HasPrefix(text, "0;") || strings.HasPrefix(text, "2;") {
			title(strings.TrimSpace(text[2:]))
		}
	}
	s.state = 0
}
