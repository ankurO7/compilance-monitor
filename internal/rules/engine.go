package rules

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ankurO7/compilance-monitor/internal/models"
)

type Result struct {
	Flagged  bool
	RuleName string
	Severity string
	Reason   string
}

type Rule interface {
	Name() string
	Evaluate(tx models.Transaction, history []models.Transaction) Result
}

// Structuring: flags multiple transactions just under a reporting
// threshold within a short window (dodging mandatory reporting). 
type StructuringRule struct {
	Threshold  float64
	NearBand   float64
	Window     time.Duration
	MinMatches int
}

func NewStructuringRule() StructuringRule {
	return StructuringRule{Threshold: 10000, NearBand: 0.9, Window: 72 * time.Hour, MinMatches: 3}
}

func (r StructuringRule) Name() string { return "structuring" }

func (r StructuringRule) Evaluate(tx models.Transaction, history []models.Transaction) Result {
	lowerBound := r.Threshold * r.NearBand
	cutoff := tx.CreatedAt.Add(-r.Window)

	count := 0
	if float64(tx.Amount) >= lowerBound && float64(tx.Amount) < r.Threshold {
		count = 1
	}
	for _, h := range history {
		if h.CreatedAt.Before(cutoff) {
			continue
		}
		if float64(h.Amount) >= lowerBound && float64(h.Amount) < r.Threshold {
			count++
		}
	}

	if count >= r.MinMatches {
		return Result{
			Flagged:  true,
			RuleName: r.Name(),
			Severity: "high",
			Reason: fmt.Sprintf("%d transactions between %.2f and %.2f within %s (possible structuring)",
				count, lowerBound, r.Threshold, r.Window),
		}
	}
	return Result{RuleName: r.Name()}
}

// Velocity anomaly: flags a transaction far outside the user's
// historical average, using a z-score.
type VelocityRule struct {
	Window     time.Duration
	ZThreshold float64
	MinHistory int
}

func NewVelocityRule() VelocityRule {
	return VelocityRule{Window: 30 * 24 * time.Hour, ZThreshold: 3.0, MinHistory: 5}
}

func (r VelocityRule) Name() string { return "velocity_anomaly" }

func (r VelocityRule) Evaluate(tx models.Transaction, history []models.Transaction) Result {
	cutoff := tx.CreatedAt.Add(-r.Window)
	var amounts []float64
	for _, h := range history {
		if h.CreatedAt.After(cutoff) {
			amounts = append(amounts, float64(h.Amount))
		}
	}
	if len(amounts) < r.MinHistory {
		return Result{RuleName: r.Name()}
	}

	mean := 0.0
	for _, a := range amounts {
		mean += a
	}
	mean /= float64(len(amounts))

	variance := 0.0
	for _, a := range amounts {
		variance += (a - mean) * (a - mean)
	}
	variance /= float64(len(amounts))
	stddev := math.Sqrt(variance)

	if stddev == 0 {
		return Result{RuleName: r.Name()}
	}

	z := (float64(tx.Amount) - mean) / stddev
	if z >= r.ZThreshold {
		return Result{
			Flagged:  true,
			RuleName: r.Name(),
			Severity: "medium",
			Reason: fmt.Sprintf("amount %.2f is %.1f standard deviations above the %d-transaction average of %.2f",
				float64(tx.Amount), z, len(amounts), mean),
		}
	}
	return Result{RuleName: r.Name()}
}

// Watchlist: flags a transaction whose counterparty matches a blocklist
// Placeholder names only, not real sanctions data.
type WatchlistRule struct {
	Blocked map[string]bool
}

func NewWatchlistRule(names []string) WatchlistRule {
	blocked := make(map[string]bool, len(names))
	for _, n := range names {
		blocked[strings.ToLower(strings.TrimSpace(n))] = true
	}
	return WatchlistRule{Blocked: blocked}
}

func (r WatchlistRule) Name() string { return "watchlist" }

func (r WatchlistRule) Evaluate(tx models.Transaction, _ []models.Transaction) Result {
	key := strings.ToLower(strings.TrimSpace(tx.Counterparty))
	if r.Blocked[key] {
		return Result{
			Flagged:  true,
			RuleName: r.Name(),
			Severity: "high",
			Reason:   fmt.Sprintf("counterparty %q matches watchlist", tx.Counterparty),
		}
	}
	return Result{RuleName: r.Name()}
}
