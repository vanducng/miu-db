package mcpserver

import (
	"fmt"
	"io"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type UnsupportedTransportError struct {
	Transport string
}

func (e *UnsupportedTransportError) Error() string {
	return fmt.Sprintf("unsupported MCP transport %q", e.Transport)
}

func transportFor(name string, stdin io.Reader, stdout io.Writer) (mcp.Transport, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", TransportStdio:
		return &mcp.IOTransport{
			Reader: nopReadCloser{Reader: stdin},
			Writer: nopWriteCloser{Writer: stdout},
		}, nil
	case "http", "streamable-http":
		return nil, &UnsupportedTransportError{Transport: name}
	default:
		return nil, &UnsupportedTransportError{Transport: name}
	}
}

type nopReadCloser struct {
	io.Reader
}

func (nopReadCloser) Close() error { return nil }

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }
