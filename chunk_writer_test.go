package vmess

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/sagernet/sing/common/buf"
)

type testChunkWriter interface {
	io.Writer
	WriteBuffer(buffer *buf.Buffer) error
}

// A zero-length chunk marks the end of the stream, so empty writes must emit nothing.
func TestChunkWritersSkipEmptyWrites(t *testing.T) {
	key, nonce := make([]byte, 16), make([]byte, 12)
	for _, testCase := range []struct {
		name      string
		newWriter func(io.Writer) testChunkWriter
		newReader func(io.Reader) io.Reader
	}{
		{
			name:      "stream",
			newWriter: func(w io.Writer) testChunkWriter { return NewStreamChunkWriter(w, nil, nil) },
			newReader: func(r io.Reader) io.Reader { return NewStreamChunkReader(r, nil, nil) },
		},
		{
			name:      "aead",
			newWriter: func(w io.Writer) testChunkWriter { return NewAes128GcmChunkWriter(w, key, nonce, nil) },
			newReader: func(r io.Reader) io.Reader { return NewAes128GcmChunkReader(r, key, nonce, nil) },
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var stream bytes.Buffer
			writer := testCase.newWriter(&stream)
			if _, err := writer.Write(nil); err != nil {
				t.Fatal(err)
			}
			if err := writer.WriteBuffer(buf.New()); err != nil {
				t.Fatal(err)
			}
			if stream.Len() != 0 {
				t.Fatalf("empty writes emitted %d bytes", stream.Len())
			}
			payload := []byte("payload")
			if _, err := writer.Write(payload); err != nil {
				t.Fatal(err)
			}
			received := make([]byte, 64)
			n, err := testCase.newReader(&stream).Read(received)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(received[:n], payload) {
				t.Fatalf("read %q, want %q", received[:n], payload)
			}
		})
	}
}

type failingTestWriter struct{}

func (failingTestWriter) Write([]byte) (int, error) { return 0, errors.New("upstream write failed") }

func TestAEADChunkWriterUnlocksAfterWriteError(t *testing.T) {
	writer := NewAes128GcmChunkWriter(failingTestWriter{}, make([]byte, 16), make([]byte, 12), nil)
	if _, err := writer.Write([]byte("first")); err == nil {
		t.Fatal("the first write unexpectedly succeeded")
	}
	done := make(chan error, 1)
	go func() {
		_, err := writer.Write([]byte("second"))
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("the second write unexpectedly succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the second write blocked after the first one failed")
	}
}
