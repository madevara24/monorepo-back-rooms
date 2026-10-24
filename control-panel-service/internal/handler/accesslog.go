package handler

import (
	"log"
	"net/http"
	"os"
	"time"
)

// AccessLog writes one line per API request: method, matched route or
// path, status, and duration. The query string, body, headers, cookies,
// session token, IP, and email are deliberately absent. /healthz is
// excluded because the deploy poll would otherwise dominate the log.
func AccessLog(logger *log.Logger, next http.Handler) http.Handler {
	if logger == nil {
		logger = log.New(os.Stdout, "", log.LstdFlags)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)

		route := r.Pattern
		if route == "" {
			route = r.Method + " " + r.URL.Path
		}
		logger.Printf("%s %d %s", route, sw.status, time.Since(start).Round(time.Millisecond))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}
