package scanner

import (
	"context"
	"io"
)

// Scanner detects malware in a stream of bytes.
//
// Implementations must be safe for concurrent use.
type Scanner interface {
	// Ping verifies that the scanning backend is reachable and healthy.
	Ping(ctx context.Context) error

	// Scan reads r to completion and reports whether the content is clean.
	// A nil error with clean == false means malware was detected.
	Scan(ctx context.Context, r io.Reader) (clean bool, err error)
}
