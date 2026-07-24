package api

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *Server) metrics(ctx context.Context) string {
	var b strings.Builder
	b.WriteString("velox_apigateway_up{service=\"apigateway\"} 1\n")

	if s.store != nil {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		up := s.store.Ping(pingCtx) == nil
		cancel()
		fmt.Fprintf(&b, "velox_apigateway_database_up{service=\"apigateway\"} %d\n", boolMetric(up))
	}
	if s.cacheClient != nil {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		up := s.cacheClient.Ping(pingCtx).Err() == nil
		cancel()
		fmt.Fprintf(&b, "velox_apigateway_cache_up{service=\"apigateway\"} %d\n", boolMetric(up))
	}
	return b.String()
}

func boolMetric(v bool) int {
	if v {
		return 1
	}
	return 0
}
