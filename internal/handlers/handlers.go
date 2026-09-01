package handlers

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/ankurO7/compilance-monitor/internal/models"
	"github.com/ankurO7/compilance-monitor/internal/store"
	"github.com/ankurO7/compilance-monitor/internal/worker"
)

type API struct {
	Store store.Store
	Pool  *worker.Pool
	IDFn  func() string
}

func (a *API) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /transactions", a.ingestTransaction)
	mux.HandleFunc("GET /alerts", a.listAlerts)
	mux.HandleFunc("POST /alerts/{id}/resolve", a.resolveAlert)
	mux.HandleFunc("GET /audit/{transactionID}", a.listAudit)
	mux.HandleFunc("GET /health", a.health)
	mux.HandleFunc("POST /demo/seed", a.seedDemoData)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

type ingestRequest struct {
	UserID       string  `json:"user_id"`
	Amount       float64 `json:"amount"`
	Currency     string  `json:"currency"`
	Counterparty string  `json:"counterparty"`
}

func (a *API) ingestTransaction(w http.ResponseWriter, r *http.Request) {
	var req ingestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.UserID == "" || req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "user_id and a positive amount are required")
		return
	}
	if req.Currency == "" {
		req.Currency = "USD"
	}

	tx := models.Transaction{
		ID:           a.IDFn(),
		UserID:       req.UserID,
		Amount:       int64(req.Amount),
		Currency:     req.Currency,
		Counterparty: req.Counterparty,
		CreatedAt:    time.Now().UTC(),
	}

	if !a.Pool.Submit(tx) {
		writeError(w, http.StatusServiceUnavailable, "evaluation queue full, try again shortly")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"transaction_id": tx.ID,
		"status":         "queued_for_evaluation",
	})
}

func (a *API) listAlerts(w http.ResponseWriter, r *http.Request) {
	var resolvedFilter *bool
	if v := r.URL.Query().Get("resolved"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "resolved must be true or false")
			return
		}
		resolvedFilter = &b
	}

	alerts, err := a.Store.ListAlerts(resolvedFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, alerts)
}

func (a *API) resolveAlert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.Store.ResolveAlert(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}

func (a *API) listAudit(w http.ResponseWriter, r *http.Request) {
	txID := r.PathValue("transactionID")
	entries, err := a.Store.ListAudit(txID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "ok",
		"queue_depth": a.Pool.QueueDepth(),
	})
}

// seedDemoData generates synthetic transactions that deliberately trigger
// each rule, so the system is demoable without real data.
func (a *API) seedDemoData(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	submitted := 0

	submit := func(userID string, amount float64, counterparty string, at time.Time) {
		tx := models.Transaction{
			ID:           a.IDFn(),
			UserID:       userID,
			Amount:       int64(amount),
			Currency:     "USD",
			Counterparty: counterparty,
			CreatedAt:    at,
		}
		if a.Pool.Submit(tx) {
			submitted++
		}
	}

	// Structuring: four transfers just under the $10,000 threshold.
	for i := 0; i < 4; i++ {
		submit("demo-structurer", 9500+rand.Float64()*300, "acme-holdings", now.Add(-time.Duration(i)*time.Hour))
	}

	// Velocity anomaly: stable history, then one huge outlier.
	for i := 0; i < 8; i++ {
		submit("demo-spender", 80+rand.Float64()*40, "grocery-co", now.Add(-time.Duration(24-i)*time.Hour))
	}
	submit("demo-spender", 15000, "electronics-mart", now)

	// Watchlist hit.
	submit("demo-watchlist-user", 500, "blocked-entity-1", now)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "seeded",
		"submitted": submitted,
		"note":      "give the worker pool a second, then GET /alerts",
	})
}