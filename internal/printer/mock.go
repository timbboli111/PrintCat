package printer

import "context"

// MemoryTransport is a safe test transport. It is not registered in production.
type MemoryTransport struct {
	TransportKind  TransportKind
	Payloads       [][]byte
	JobDiagnostics []JobDiagnostics
}

func (m *MemoryTransport) Kind() TransportKind { return m.TransportKind }

func (m *MemoryTransport) Send(ctx context.Context, _ string, payload []byte, _ map[string]string) error {
	m.Payloads = append(m.Payloads, append([]byte(nil), payload...))
	m.JobDiagnostics = append(m.JobDiagnostics, JobDiagnosticsFromContext(ctx))
	return nil
}
