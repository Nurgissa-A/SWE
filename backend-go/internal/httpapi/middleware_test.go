package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSAndRequestID(t *testing.T) {
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }), []string{"http://localhost:3000"})
	for _, tc := range []struct {
		origin, method, headers string
		want                    int
	}{{"http://localhost:3000", "PATCH", "authorization,content-type", 204}, {"https://evil.example", "PATCH", "authorization", 403}, {"http://localhost:3000", "DELETE", "authorization", 403}, {"http://localhost:3000", "PATCH", "x-untrusted", 403}} {
		r := httptest.NewRequest("OPTIONS", "/api/events", nil)
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Access-Control-Request-Method", tc.method)
		r.Header.Set("Access-Control-Request-Headers", tc.headers)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != tc.want || w.Header().Get("X-Request-ID") == "" {
			t.Fatal(w.Code, w.Header())
		}
		if tc.want == 204 && w.Header().Get("Access-Control-Allow-Origin") != tc.origin {
			t.Fatal("missing origin")
		}
	}
	r := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
}
func TestPanicIsContained(t *testing.T) {
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic("secret") }), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 500 {
		t.Fatal(w.Code)
	}
}
