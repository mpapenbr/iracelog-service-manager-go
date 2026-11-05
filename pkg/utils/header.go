package utils

import (
	"context"
	"net/http"

	"connectrpc.com/connect/v2"
)

// RequestHeaderFromServerContext returns the request headers of the server-side
// RPC carried in ctx as http.Header. Returns an empty header if ctx does not
// carry a server-side connect.CallInfo.
func RequestHeaderFromServerContext(ctx context.Context) http.Header {
	ret := http.Header{}
	info, ok := connect.CallInfoForServerContext(ctx)
	if !ok {
		return ret
	}
	for k, v := range info.RequestHeader().All() {
		ret[k] = append([]string(nil), v...)
	}
	return ret
}
