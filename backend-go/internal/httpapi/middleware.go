package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/biletflow/backend-go/internal/auth"
)

type response struct {
	http.ResponseWriter
	status int
}

func (w *response) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
		w.ResponseWriter.WriteHeader(status)
	}
}
func (w *response) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func Middleware(next http.Handler, origins []string) http.Handler {
	allowed := map[string]bool{}
	for _, o := range origins {
		allowed[o] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		var random [12]byte
		if _, err := rand.Read(random[:]); err != nil {
			auth.WriteError(w, 500, "request initialization failed")
			return
		}
		requestID := hex.EncodeToString(random[:])
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Add("Vary", "Origin")
		if origin := r.Header.Get("Origin"); origin != "" && allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		}
		if r.Method == http.MethodOptions {
			if !allowed[r.Header.Get("Origin")] {
				auth.WriteError(w, 403, "origin not allowed")
				return
			}
			method := r.Header.Get("Access-Control-Request-Method")
			if method != "GET" && method != "POST" && method != "PATCH" {
				auth.WriteError(w, 403, "method not allowed")
				return
			}
			for _, h := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
				h = strings.TrimSpace(strings.ToLower(h))
				if h != "" && h != "authorization" && h != "content-type" {
					auth.WriteError(w, 403, "header not allowed")
					return
				}
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.WriteHeader(204)
			return
		}
		rw := &response{ResponseWriter: w}
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("request panic", "requestId", requestID)
				if rw.status == 0 {
					auth.WriteError(rw, 500, "internal error")
				}
			}
			slog.Info("http request", "requestId", requestID, "method", r.Method, "route", r.Pattern, "status", rw.status, "durationMs", time.Since(start).Milliseconds())
		}()
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		r.Body = http.MaxBytesReader(rw, r.Body, 1<<20)
		r = r.WithContext(ctx)
		next.ServeHTTP(rw, r)
	})
}
