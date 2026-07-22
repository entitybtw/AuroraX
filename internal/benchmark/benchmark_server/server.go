package benchserver

import (
	"context"
	"net"
	"net/http"

	"aurora/internal/benchmark"
)

type BenchServer struct {
	handler http.Handler
	httpSrv *http.Server
}

func NewBenchServer(mockCfg benchmark.MockProviderConfig) *BenchServer {
	mock := benchmark.NewMockProvider(mockCfg)
	handler := benchmark.NewBenchHTTPHandler(mock)
	return &BenchServer{handler: handler}
}

func (s *BenchServer) StartListener(ctx context.Context, addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}

func (s *BenchServer) Serve(l net.Listener) error {
	s.httpSrv = &http.Server{Handler: s.handler}
	s.httpSrv.Protocols = new(http.Protocols)
	s.httpSrv.Protocols.SetHTTP1(true)
	s.httpSrv.Protocols.SetUnencryptedHTTP2(true)
	return s.httpSrv.Serve(l)
}

func (s *BenchServer) Close() {
	if s.httpSrv != nil {
		_ = s.httpSrv.Close()
	}
}
