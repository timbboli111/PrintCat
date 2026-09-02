package tcp

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/timboli111/PrintCat/internal/printer"
)

type TCPTransport struct{}

func (t *TCPTransport) Kind() printer.TransportKind {
	return printer.TCP
}

func (t *TCPTransport) Send(ctx context.Context, endpoint string, payload []byte, options map[string]string) error {
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint: %w", err)
	}

	timeout := 5 * time.Second
	if timeoutStr, ok := options["connect_timeout"]; ok {
		if sec, err := strconv.Atoi(timeoutStr); err == nil && sec > 0 {
			timeout = time.Duration(sec) * time.Second
		}
	}

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		conn.SetWriteDeadline(deadline)
	}

	sent := 0
	for sent < len(payload) {
		n, err := conn.Write(payload[sent:])
		if err != nil {
			return fmt.Errorf("write failed: %w", err)
		}
		sent += n
	}
	return nil
}
