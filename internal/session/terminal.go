package session

import "time"

// terminalIO keeps platform terminal ownership below the shared session stream.
// *os.File implements it on Unix; Windows combines ConPTY's separate pipes.
type terminalIO interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
	SetWriteDeadline(time.Time) error
}
