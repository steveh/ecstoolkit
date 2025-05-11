package shellsession

import (
	"context"
	"io"
)

type ioret struct {
	n   int
	err error
}

type ctxReader struct {
	r   io.Reader
	ctx context.Context //nolint:containedctx
}

// newContextReader creates a new context-aware reader.
func newContextReader(ctx context.Context, r io.Reader) *ctxReader {
	return &ctxReader{ctx: ctx, r: r}
}

// Read reads data from the underlying reader, respecting the context's cancellation.
func (r *ctxReader) Read(buf []byte) (int, error) {
	buf2 := make([]byte, len(buf))

	c := make(chan ioret, 1)

	go func() {
		n, err := r.r.Read(buf2)
		c <- ioret{n, err}
		close(c)
	}()

	select {
	case ret := <-c:
		copy(buf, buf2)

		return ret.n, ret.err
	case <-r.ctx.Done():
		return 0, r.ctx.Err() //nolint:wrapcheck
	}
}
