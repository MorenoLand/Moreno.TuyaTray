package lanlink

import (
	"io"
	"net"
	"sync"
	"time"
)

type Stream struct {
	c    *Conn
	mu   sync.Mutex
	conn net.Conn
	seq  uint32
}

func NewStream(c *Conn) *Stream { return &Stream{c: c, seq: 100} }

func (s *Stream) open() error {
	if err := s.c.check(); err != nil {
		return err
	}
	conn, err := s.c.dial()
	if err != nil {
		return err
	}
	_ = conn.SetDeadline(zeroTime)
	s.conn = conn
	go func() { _, _ = io.Copy(io.Discard, conn) }()
	return nil
}

func (s *Stream) Send(dps Dps) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for try := 0; try < 2; try++ {
		if s.conn == nil {
			if err := s.open(); err != nil {
				return err
			}
		}
		p, err := s.c.sealed(cmdControl, s.c.body(map[string]any{"dps": dps}))
		if err != nil {
			return err
		}
		s.seq++
		_ = s.conn.SetWriteDeadline(time.Now().Add(600 * time.Millisecond))
		if _, err = s.conn.Write(buildFrame(s.seq, cmdControl, p)); err == nil {
			return nil
		}
		_ = s.conn.Close()
		s.conn = nil
	}
	return io.ErrClosedPipe
}

func (s *Stream) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
}
