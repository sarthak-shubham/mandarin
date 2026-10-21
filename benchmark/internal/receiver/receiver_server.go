package receiver

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"
)

type ReceiverServer struct {
	Metrics     *ReceiverMetrics
	Delay       time.Duration
	FailureRate float64
	Port        int
}

func NewReceiverServer(port int, delay time.Duration, failureRate float64) *ReceiverServer {
	srv := &ReceiverServer{
		Metrics:     NewReceiverMetrics(),
		Delay:       delay,
		FailureRate: failureRate,
		Port:        port,
	}
	return srv
}

func (s *ReceiverServer) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /webhook/", s.handleWebhook)
	mux.HandleFunc("GET /metrics", s.handleMetrics)
	mux.HandleFunc("POST /reset", s.handleReset)

	return mux
}

func (s *ReceiverServer) handleWebhook(w http.ResponseWriter, r *http.Request) {
	_, err := io.Copy(io.Discard, r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if s.Delay > 0 {
		time.Sleep(s.Delay)
	}

	if s.FailureRate > 0 && rand.Float64() < s.FailureRate {
		s.Metrics.RecordDelivery(false)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	s.Metrics.RecordDelivery(true)
	w.WriteHeader(http.StatusOK)
}

func (s *ReceiverServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	snap := s.Metrics.Snapshot()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err := json.NewEncoder(w).Encode(snap)
	if err != nil {
		http.Error(w, "encode metrics error", http.StatusInternalServerError)
		return
	}
}

func (s *ReceiverServer) handleReset(w http.ResponseWriter, r *http.Request) {
	s.Metrics.Reset()
	w.WriteHeader(http.StatusOK)
}

func (s *ReceiverServer) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: s.Handler(),
	}

	err := server.ListenAndServe()
	return err
}
