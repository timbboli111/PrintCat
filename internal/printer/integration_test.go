package printer_test

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/printer"
	"github.com/timboli111/PrintCat/internal/printer/cpcl"
	"github.com/timboli111/PrintCat/internal/printer/epl"
	"github.com/timboli111/PrintCat/internal/printer/starprnt"
	"github.com/timboli111/PrintCat/internal/printer/transport/tcp"
	"github.com/timboli111/PrintCat/internal/printer/tspl"
	"github.com/timboli111/PrintCat/internal/printer/zpl"
	"github.com/timboli111/PrintCat/internal/render/basic"
)

func TestIntegrationTSPLOverTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	var received []byte
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		n, _ := conn.Read(buf)
		received = append(received, buf[:n]...)
	}()

	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	elem := document.NewTextElement("t1", "Hello", document.Rect{
		Position: document.Point{X: 10000, Y: 10000},
		Size:     document.Size{Width: 40000, Height: 10000},
	}, 12000)
	doc.Elements = append(doc.Elements, elem)

	p := printer.Printer{
		ID:   "test",
		Name: "Test Printer",
		Connection: printer.Connection{
			Protocol:  printer.TSPL,
			Transport: printer.TCP,
			Endpoint:  ln.Addr().String(),
			Options:   nil,
		},
		Profile: printer.PrinterProfile{
			DPI:                 203,
			SupportedProtocols:  []printer.Protocol{printer.TSPL},
			SupportedTransports: []printer.TransportKind{printer.TCP},
		},
	}

	service := printer.NewService()
	if err := service.RegisterBackend(&tspl.Encoder{}); err != nil {
		t.Fatal(err)
	}
	if err := service.RegisterTransport(&tcp.TCPTransport{}); err != nil {
		t.Fatal(err)
	}

	renderer := &basic.Renderer{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := service.Print(ctx, p, doc, renderer); err != nil {
		t.Fatalf("Print error: %v", err)
	}

	<-done

	if len(received) == 0 {
		t.Error("no data received")
	}
	payload := string(received)
	expectedCmds := []string{"SIZE", "GAP", "CLS", "TEXT", "PRINT"}
	for _, cmd := range expectedCmds {
		if !strings.Contains(payload, cmd) {
			t.Errorf("missing command %q in payload: %s", cmd, payload)
		}
	}
}

func TestIntegrationZPLOverTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	var received []byte
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		n, _ := conn.Read(buf)
		received = append(received, buf[:n]...)
	}()

	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	elem := document.NewTextElement("t1", "Hello", document.Rect{
		Position: document.Point{X: 10000, Y: 10000},
		Size:     document.Size{Width: 40000, Height: 10000},
	}, 12000)
	doc.Elements = append(doc.Elements, elem)

	p := printer.Printer{
		ID:   "test",
		Name: "Test Printer",
		Connection: printer.Connection{
			Protocol:  printer.ZPL,
			Transport: printer.TCP,
			Endpoint:  ln.Addr().String(),
			Options:   nil,
		},
		Profile: printer.PrinterProfile{
			DPI:                 203,
			SupportedProtocols:  []printer.Protocol{printer.ZPL},
			SupportedTransports: []printer.TransportKind{printer.TCP},
		},
	}

	service := printer.NewService()
	if err := service.RegisterBackend(&zpl.Encoder{}); err != nil {
		t.Fatal(err)
	}
	if err := service.RegisterTransport(&tcp.TCPTransport{}); err != nil {
		t.Fatal(err)
	}

	renderer := &basic.Renderer{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := service.Print(ctx, p, doc, renderer); err != nil {
		t.Fatalf("Print error: %v", err)
	}

	<-done

	if len(received) == 0 {
		t.Error("no data received")
	}
	payload := string(received)
	expectedCmds := []string{"^XA", "^PW", "^LL", "^FO", "^FD", "^FS", "^XZ"}
	for _, cmd := range expectedCmds {
		if !strings.Contains(payload, cmd) {
			t.Errorf("missing command %q in payload: %s", cmd, payload)
		}
	}
}

func TestIntegrationCPCLOverTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	var received []byte
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		n, _ := conn.Read(buf)
		received = append(received, buf[:n]...)
	}()

	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	elem := document.NewTextElement("t1", "Hello", document.Rect{
		Position: document.Point{X: 10000, Y: 10000},
		Size:     document.Size{Width: 40000, Height: 10000},
	}, 12000)
	doc.Elements = append(doc.Elements, elem)

	p := printer.Printer{
		ID:   "test",
		Name: "Test Printer",
		Connection: printer.Connection{
			Protocol:  printer.CPCL,
			Transport: printer.TCP,
			Endpoint:  ln.Addr().String(),
			Options:   nil,
		},
		Profile: printer.PrinterProfile{
			DPI:                 203,
			SupportedProtocols:  []printer.Protocol{printer.CPCL},
			SupportedTransports: []printer.TransportKind{printer.TCP},
		},
	}

	service := printer.NewService()
	if err := service.RegisterBackend(&cpcl.Encoder{}); err != nil {
		t.Fatal(err)
	}
	if err := service.RegisterTransport(&tcp.TCPTransport{}); err != nil {
		t.Fatal(err)
	}

	renderer := &basic.Renderer{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := service.Print(ctx, p, doc, renderer); err != nil {
		t.Fatalf("Print error: %v", err)
	}

	<-done

	if len(received) == 0 {
		t.Error("no data received")
	}
	payload := string(received)
	expectedCmds := []string{"! 0", "203", "TEXT", "Hello", "PRINT"}
	for _, cmd := range expectedCmds {
		if !strings.Contains(payload, cmd) {
			t.Errorf("missing command %q in payload: %s", cmd, payload)
		}
	}
}

func TestIntegrationEPLOverTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	var received []byte
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		n, _ := conn.Read(buf)
		received = append(received, buf[:n]...)
	}()

	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	elem := document.NewTextElement("t1", "Hello", document.Rect{
		Position: document.Point{X: 10000, Y: 10000},
		Size:     document.Size{Width: 40000, Height: 10000},
	}, 12000)
	doc.Elements = append(doc.Elements, elem)

	p := printer.Printer{
		ID:   "test",
		Name: "Test Printer",
		Connection: printer.Connection{
			Protocol:  printer.EPL,
			Transport: printer.TCP,
			Endpoint:  ln.Addr().String(),
			Options:   nil,
		},
		Profile: printer.PrinterProfile{
			DPI:                 203,
			SupportedProtocols:  []printer.Protocol{printer.EPL},
			SupportedTransports: []printer.TransportKind{printer.TCP},
		},
	}

	service := printer.NewService()
	if err := service.RegisterBackend(&epl.Encoder{}); err != nil {
		t.Fatal(err)
	}
	if err := service.RegisterTransport(&tcp.TCPTransport{}); err != nil {
		t.Fatal(err)
	}

	renderer := &basic.Renderer{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := service.Print(ctx, p, doc, renderer); err != nil {
		t.Fatalf("Print error: %v", err)
	}

	<-done

	if len(received) == 0 {
		t.Error("no data received")
	}
	payload := string(received)
	expectedCmds := []string{"N\n", "q", "Q", "A", "Hello", "P"}
	for _, cmd := range expectedCmds {
		if !strings.Contains(payload, cmd) {
			t.Errorf("missing command %q in payload: %s", cmd, payload)
		}
	}
}

func TestIntegrationStarPRNTOverTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	var received []byte
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		n, _ := conn.Read(buf)
		received = append(received, buf[:n]...)
	}()

	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	elem := document.NewTextElement("t1", "Hello", document.Rect{
		Position: document.Point{X: 10000, Y: 10000},
		Size:     document.Size{Width: 40000, Height: 10000},
	}, 12000)
	doc.Elements = append(doc.Elements, elem)

	p := printer.Printer{
		ID:   "test",
		Name: "Test Printer",
		Connection: printer.Connection{
			Protocol:  printer.StarPRNT,
			Transport: printer.TCP,
			Endpoint:  ln.Addr().String(),
			Options:   nil,
		},
		Profile: printer.PrinterProfile{
			DPI:                 203,
			SupportedProtocols:  []printer.Protocol{printer.StarPRNT},
			SupportedTransports: []printer.TransportKind{printer.TCP},
		},
	}

	service := printer.NewService()
	if err := service.RegisterBackend(&starprnt.Encoder{}); err != nil {
		t.Fatal(err)
	}
	if err := service.RegisterTransport(&tcp.TCPTransport{}); err != nil {
		t.Fatal(err)
	}

	renderer := &basic.Renderer{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := service.Print(ctx, p, doc, renderer); err != nil {
		t.Fatalf("Print error: %v", err)
	}

	<-done

	if len(received) == 0 {
		t.Error("no data received")
	}

	// Verifikasi payload StarPRNT
	expected := []struct {
		name string
		seq  []byte
	}{
		{"ESC @", []byte{0x1B, 0x40}},
		{"ESC C", []byte{0x1B, 0x43, 0x00}},
		{"ESC GS A", []byte{0x1B, 0x1D, 0x41}},
		{"ESC RS F", []byte{0x1B, 0x1E, 0x46}},
		{"text Hello", []byte("Hello")},
		{"FF", []byte{0x0C}},
	}
	for _, e := range expected {
		if !bytes.Contains(received, e.seq) {
			t.Errorf("missing %s in payload: %x", e.name, received)
		}
	}
}
