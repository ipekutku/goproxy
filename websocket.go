package goproxy

import (
	"io"
	"net"
	"net/http"
	"strings"
)

func headerContains(header http.Header, name string, value string) bool {
	for _, v := range header[name] {
		for _, s := range strings.Split(v, ",") {
			if strings.EqualFold(value, strings.TrimSpace(s)) {
				return true
			}
		}
	}
	return false
}

func isWebSocketRequest(r *http.Request) bool {
	return headerContains(r.Header, "Connection", "upgrade") &&
		(headerContains(r.Header, "Upgrade", "websocket") ||
			headerContains(r.Header, "Upgrade", "SPDY/3.1"))
}

func (proxy *ProxyHttpServer) hijackConnection(ctx *ProxyCtx, w http.ResponseWriter) (net.Conn, error) {
	// Connect to Client
	hj, ok := w.(http.Hijacker)
	if !ok {
		panic("httpserver does not support hijacking")
	}
	clientConn, _, err := hj.Hijack()
	if err != nil {
		ctx.Warnf("Hijack error: %v", err)
		return nil, err
	}
	return clientConn, nil
}

func (proxy *ProxyHttpServer) proxyWebsocket(ctx *ProxyCtx, remoteConn io.ReadWriter, proxyClient io.ReadWriter) {
	// 2 is the number of goroutines, this code is implemented according to
	// https://stackoverflow.com/questions/52031332/wait-for-one-goroutine-to-finish
	waitChan := make(chan struct{}, 2)
	go func() {
		_ = copyOrWarn(ctx, remoteConn, proxyClient)
		waitChan <- struct{}{}
	}()

	go func() {
		_ = copyOrWarn(ctx, proxyClient, remoteConn)
		waitChan <- struct{}{}
	}()

	// Wait until one end closes the connection
	<-waitChan
}
