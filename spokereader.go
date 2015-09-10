package spin

import (
	"io"
)

type spokeReader struct {
	spoke Spoke
	msg   []byte
	ok    bool
}

func newSpokeReader(spoke Spoke) *spokeReader {
	return &spokeReader{
		spoke: spoke,
		ok:    true,
	}
}

func (r *spokeReader) Read(p []byte) (int, error) {
	if len(r.msg) > 0 {
		var n int
		if len(r.msg) > len(p) {
			n = len(p)
			copy(p, r.msg[:n])
			r.msg = r.msg[n:]
		} else {
			n = len(r.msg)
			copy(p[:n], r.msg)
			r.msg = nil
		}
		return n, nil
	}
	if !r.ok {
		return 0, io.EOF
	}
	r.msg, r.ok = r.spoke.Receive()
	return r.Read(p)
}
