package vless

import (
	"net"
	"testing"
)

type closeRecorder struct {
	net.Conn
	closed int
}

func (c *closeRecorder) Close() error {
	c.closed++
	return nil
}

func TestVisionCloseAfterDirectWriteSkipsTLSLayer(t *testing.T) {
	outer, raw, uplink := &closeRecorder{}, &closeRecorder{}, &closeRecorder{}
	conn := &VisionConn{Conn: outer, netConn: raw, rawConn: raw, directConn: raw}
	if err := conn.Close(); err != nil || outer.closed != 1 || raw.closed != 0 {
		t.Fatalf("before direct writes Close goes through the TLS layer: outer %d raw %d err %v", outer.closed, raw.closed, err)
	}
	conn = &VisionConn{Conn: outer, netConn: raw, rawConn: raw, directConn: raw}
	conn.directWrite.Store(true)
	if err := conn.Close(); err != nil || outer.closed != 1 || raw.closed != 1 {
		t.Fatalf("after direct writes Close closes the shared raw connection once: outer %d raw %d err %v", outer.closed, raw.closed, err)
	}
	conn = &VisionConn{Conn: outer, netConn: raw, rawConn: raw, directConn: uplink}
	conn.directWrite.Store(true)
	if err := conn.Close(); err != nil || outer.closed != 1 || raw.closed != 2 || uplink.closed != 1 {
		t.Fatalf("after direct writes Close closes both raw connections only: outer %d raw %d uplink %d err %v", outer.closed, raw.closed, uplink.closed, err)
	}
}

func TestVisionCloseClosesHandedOverConnections(t *testing.T) {
	outer, raw, early, late := &closeRecorder{}, &closeRecorder{}, &closeRecorder{}, &closeRecorder{}
	conn := &VisionConn{Conn: outer, netConn: raw, rawConn: raw, directConn: raw}
	conn.SetDirectConn(early)
	if err := conn.Close(); err != nil || outer.closed != 1 || early.closed != 1 || raw.closed != 0 {
		t.Fatalf("a handed-over connection closes with the TLS layer: outer %d early %d raw %d err %v", outer.closed, early.closed, raw.closed, err)
	}
	conn.SetDirectConn(late)
	if late.closed != 1 {
		t.Fatalf("a connection handed over after Close is closed at once: %d", late.closed)
	}
}

func TestVisionCloseRacesSetDirectConn(t *testing.T) {
	for i := 0; i < 200; i++ {
		conn := &VisionConn{Conn: &closeRecorder{}, netConn: &closeRecorder{}, directConn: &closeRecorder{}}
		conn.directWrite.Store(true)
		handed := &closeRecorder{}
		done := make(chan struct{})
		go func() {
			conn.SetDirectConn(handed)
			close(done)
		}()
		_ = conn.Close()
		<-done
		// Handed over before Close, it closed with the raw connections; after, at once.
		if handed.closed != 1 {
			t.Fatalf("the handed-over connection closed %d times", handed.closed)
		}
	}
}
