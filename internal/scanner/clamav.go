package scanner

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
)

const (
	clamAVChunkSize        = 32 * 1024
	clamAVMaxResponseBytes = 4096
)

// ClamAV is a Scanner backed by a clamd daemon reachable over TCP.
type ClamAV struct {
	address string
}

var _ Scanner = (*ClamAV)(nil)

func NewClamAV(address string) *ClamAV {
	return &ClamAV{
		address: address,
	}
}

func (c *ClamAV) Ping(ctx context.Context) error {
	conn, err := c.dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("zPING\x00")); err != nil {
		return fmt.Errorf("sending PING command: %w", err)
	}

	response, err := readResponse(conn)
	if err != nil {
		return err
	}

	if response != "PONG" {
		return fmt.Errorf("unexpected ClamAV response: %q", response)
	}

	return nil
}

func (c *ClamAV) Scan(ctx context.Context, r io.Reader) (bool, error) {
	conn, err := c.dial(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("zINSTREAM\x00")); err != nil {
		return false, fmt.Errorf("sending INSTREAM command: %w", err)
	}

	buf := make([]byte, clamAVChunkSize)

	for {
		n, err := r.Read(buf)

		if n > 0 {
			if err := writeChunk(conn, buf[:n]); err != nil {
				return false, err
			}
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			return false, fmt.Errorf("reading file: %w", err)
		}
	}

	// A zero-length chunk tells clamd the stream has ended.
	if err := writeChunk(conn, nil); err != nil {
		return false, err
	}

	response, err := readResponse(conn)
	if err != nil {
		return false, err
	}

	switch {
	case strings.HasPrefix(response, "stream: OK"):
		return true, nil
	case strings.HasSuffix(response, "FOUND"):
		return false, nil
	default:
		return false, fmt.Errorf("unexpected ClamAV response: %s", response)
	}
}

func (c *ClamAV) dial(ctx context.Context) (net.Conn, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", c.address)
	if err != nil {
		return nil, fmt.Errorf("connecting to ClamAV: %w", err)
	}

	// Make blocking reads and writes honour the caller's deadline.
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			conn.Close()
			return nil, fmt.Errorf("setting ClamAV deadline: %w", err)
		}
	}

	return conn, nil
}

// readResponse reads clamd's full reply. clamd closes the connection after
// answering a z-prefixed command, so reading until EOF yields the whole
// message (a single Read may return only part of it).
func readResponse(r io.Reader) (string, error) {
	raw, err := io.ReadAll(io.LimitReader(r, clamAVMaxResponseBytes))
	if err != nil {
		return "", fmt.Errorf("reading ClamAV response: %w", err)
	}

	return strings.TrimRight(string(raw), "\x00\r\n "), nil
}

// writeChunk frames data for the INSTREAM protocol: a 4-byte big-endian
// length followed by the payload. A nil/empty payload is the terminator.
func writeChunk(w io.Writer, data []byte) error {
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(data)))

	if _, err := w.Write(header[:]); err != nil {
		return fmt.Errorf("writing chunk size: %w", err)
	}

	if len(data) == 0 {
		return nil
	}

	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("writing chunk: %w", err)
	}

	return nil
}
