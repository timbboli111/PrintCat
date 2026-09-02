package tcp

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/timboli111/PrintCat/internal/printer"
)

func TestKind(t *testing.T) {
	tr := &TCPTransport{}
	if tr.Kind() != printer.TCP {
		t.Errorf("expected TCP, got %v", tr.Kind())
	}
}

func TestSendValid(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverDone := make(chan struct{})
	var received []byte
	go func() {
		defer close(serverDone)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		received = buf[:n]
	}()

	ctx := context.Background()
	tr := &TCPTransport{}
	payload := []byte("hello tcp")
	endpoint := ln.Addr().String()
	err = tr.Send(ctx, endpoint, payload, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	<-serverDone
	if string(received) != string(payload) {
		t.Errorf("received %q, want %q", received, payload)
	}
}

func TestInvalidEndpoint(t *testing.T) {
	tr := &TCPTransport{}
	ctx := context.Background()
	err := tr.Send(ctx, "invalid", []byte("data"), nil)
	if err == nil {
		t.Error("expected error for invalid endpoint")
	}
}

func TestConnectionRefused(t *testing.T) {
	tr := &TCPTransport{}
	ctx := context.Background()
	err := tr.Send(ctx, "127.0.0.1:9999", []byte("data"), nil)
	if err == nil {
		t.Error("expected connection refused error")
	}
}

func TestContextCancellation(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tr := &TCPTransport{}
	err = tr.Send(ctx, ln.Addr().String(), []byte("data"), nil)
	if err == nil {
		t.Error("expected error from canceled context")
	}
}

func TestContextTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	tr := &TCPTransport{}
	err = tr.Send(ctx, ln.Addr().String(), []byte("data"), nil)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestEmptyPayload(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverDone := make(chan struct{})
	var received []byte
	go func() {
		defer close(serverDone)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		received = buf[:n]
	}()

	tr := &TCPTransport{}
	err = tr.Send(context.Background(), ln.Addr().String(), []byte{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	<-serverDone
	if len(received) != 0 {
		t.Errorf("expected empty payload, got %d bytes", len(received))
	}
}

func TestConnectTimeoutOption(t *testing.T) {
	// Set a very short timeout via option, but connect to a non-listening port.
	tr := &TCPTransport{}
	ctx := context.Background()
	options := map[string]string{"connect_timeout": "1"} // 1 second
	start := time.Now()
	err := tr.Send(ctx, "127.0.0.1:9999", []byte("data"), options)
	elapsed := time.Since(start)
	if err == nil {
		t.Error("expected error")
	}
	if elapsed > 2*time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}
