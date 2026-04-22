package server

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// getCounterValue returns the current value of a CounterVec for the given labels.
func getCounterValue(t *testing.T, counter *prometheus.CounterVec, labels ...string) float64 {
	t.Helper()
	m := &dto.Metric{}
	if err := counter.WithLabelValues(labels...).Write(m); err != nil {
		t.Fatalf("failed to read counter: %v", err)
	}
	return m.GetCounter().GetValue()
}

// getHistogramCount returns the sample count of a HistogramVec for the given labels.
func getHistogramCount(t *testing.T, hist *prometheus.HistogramVec, labels ...string) uint64 {
	t.Helper()
	observer := hist.WithLabelValues(labels...).(prometheus.Histogram)
	m := &dto.Metric{}
	if err := observer.Write(m); err != nil {
		t.Fatalf("failed to read histogram: %v", err)
	}
	return m.GetHistogram().GetSampleCount()
}

func TestMetrics_RequestCount(t *testing.T) {
	before := getCounterValue(t, httpRequestsTotal, "GET", "200")

	handler := Metrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	after := getCounterValue(t, httpRequestsTotal, "GET", "200")
	if diff := after - before; diff != 1 {
		t.Errorf("expected counter to increment by 1, got %v", diff)
	}
}

func TestMetrics_RequestDuration(t *testing.T) {
	before := getHistogramCount(t, httpRequestDuration, "GET")

	handler := Metrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	after := getHistogramCount(t, httpRequestDuration, "GET")
	if diff := after - before; diff != 1 {
		t.Errorf("expected histogram sample count to increment by 1, got %v", diff)
	}
}

func TestMetrics_StatusCodeCapture(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statusStr := strconv.Itoa(tt.statusCode)
			before := getCounterValue(t, httpRequestsTotal, "GET", statusStr)

			handler := Metrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			after := getCounterValue(t, httpRequestsTotal, "GET", statusStr)
			if diff := after - before; diff != 1 {
				t.Errorf("expected counter for status %d to increment by 1, got %v", tt.statusCode, diff)
			}
		})
	}
}

func TestMetrics_DefaultStatusCode(t *testing.T) {
	// When WriteHeader is not called explicitly, the default should be 200.
	before := getCounterValue(t, httpRequestsTotal, "GET", "200")

	handler := Metrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do not call WriteHeader; Go defaults to 200 on first Write.
		_, _ = w.Write([]byte("hello"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/default", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	after := getCounterValue(t, httpRequestsTotal, "GET", "200")
	if diff := after - before; diff != 1 {
		t.Errorf("expected counter to increment by 1 for default 200, got %v", diff)
	}
}
