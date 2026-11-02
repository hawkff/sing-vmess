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
	conn := &VisionConn{Conn: outer, netConn: raw, directConn: raw}
	if err := conn.Close(); err != nil || outer.closed != 1 || raw.closed != 0 {
		t.Fatalf("before direct writes Close goes through the TLS layer: outer %d raw %d err %v", outer.closed, raw.closed, err)
	}
	conn = &VisionConn{Conn: outer, netConn: raw, directConn: uplink}
	conn.directWrite.Store(true)
	if err := conn.Close(); err != nil || outer.closed != 1 || raw.closed != 1 || uplink.closed != 1 {
		t.Fatalf("after direct writes Close closes the raw connections only: outer %d raw %d uplink %d err %v", outer.closed, raw.closed, uplink.closed, err)
	}
}
