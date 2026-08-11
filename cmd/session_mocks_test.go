package cmd

type mockSessionConnector struct {
	connectedTo string
	err         error
	order       *[]string
}

func (m *mockSessionConnector) Connect(name string) error {
	m.connectedTo = name
	if m.order != nil {
		*m.order = append(*m.order, "connect")
	}
	return m.err
}

type mockSessionValidator struct {
	sessions map[string]bool
}

func (m *mockSessionValidator) HasSession(name string) bool {
	return m.sessions[name]
}

type ackWrite struct {
	batch string
	token string
}

type mockAckWriter struct {
	calls []ackWrite
	err   error
	order *[]string
}

func (m *mockAckWriter) Write(batch, token string) error {
	m.calls = append(m.calls, ackWrite{batch: batch, token: token})
	if m.order != nil {
		*m.order = append(*m.order, "write")
	}
	return m.err
}
