package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// Gzip transparently decompresses gzip-encoded request bodies and compresses
// responses when the client accepts gzip and the payload is textual.
func Gzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			zr, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "invalid gzip body", http.StatusBadRequest)
				return
			}
			defer zr.Close()
			r.Body = io.NopCloser(zr)
			r.Header.Del("Content-Length")
			r.ContentLength = -1
		}

		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		gw := &gzipResponseWriter{ResponseWriter: w}
		defer gw.Close()
		next.ServeHTTP(gw, r)
	})
}

// gzipResponseWriter compresses the response body only when the content type is
// compressible. The underlying gzip.Writer is created lazily so uncompressed
// responses (including empty 204 bodies) pass through untouched.
type gzipResponseWriter struct {
	http.ResponseWriter
	zw          *gzip.Writer
	wroteHeader bool
	compress    bool
}

func (g *gzipResponseWriter) WriteHeader(status int) {
	if !g.wroteHeader {
		g.wroteHeader = true
		g.compress = compressibleContentType(g.Header().Get("Content-Type"))
		if g.compress {
			g.Header().Set("Content-Encoding", "gzip")
			g.Header().Del("Content-Length")
			g.zw = gzip.NewWriter(g.ResponseWriter)
		}
	}
	g.ResponseWriter.WriteHeader(status)
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if !g.wroteHeader {
		g.WriteHeader(http.StatusOK)
	}
	if g.compress {
		return g.zw.Write(b)
	}
	return g.ResponseWriter.Write(b)
}

// Close flushes and closes the gzip writer if one was created.
func (g *gzipResponseWriter) Close() error {
	if g.zw != nil {
		return g.zw.Close()
	}
	return nil
}

func compressibleContentType(ct string) bool {
	return strings.Contains(ct, "application/json") || strings.Contains(ct, "text/plain")
}
