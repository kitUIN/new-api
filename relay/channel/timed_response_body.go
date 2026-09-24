package channel

import (
	"io"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// timedResponseBody measures body receipt separately from response headers.
// SSE handlers can stop at a protocol terminator before the HTTP body reaches EOF.
type timedResponseBody struct {
	io.ReadCloser
	info *relaycommon.RelayInfo
}

func (body *timedResponseBody) Read(p []byte) (int, error) {
	n, err := body.ReadCloser.Read(p)
	if n > 0 {
		body.info.MarkUpstreamFirstByte()
	}
	if err != nil {
		body.info.MarkUpstreamRequestEnd()
	}
	return n, err
}

func (body *timedResponseBody) Close() error {
	body.info.MarkUpstreamRequestEnd()
	return body.ReadCloser.Close()
}
