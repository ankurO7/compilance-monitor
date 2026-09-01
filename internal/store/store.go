package store

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/ankurO7/compilance-monitor/internal/models"
)

type Store interface {
	SaveTransaction(tx models.Transaction) error
	RecentTransactions(userID string, since time.Time) ([]models.Transaction, error)
	SaveAlert(a models.Alert) error
	ListAlerts(resolvedFilter *bool) ([]models.Alert, error)
	ResolveAlert(id string) error
	AppendAudit(e models.AuditEntry) error
	ListAudit(transactionID string) ([]models.AuditEntry, error)
}

type InMemoryStore struct {
	mu           sync.RWMutex
	transactions map[string][]models.Transaction
	alerts       map[string]models.Alert
	audit        []models.AuditEntry
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		transactions: make(map[string][]models.Transaction),
		alerts:       make(map[string]models.Alert),
	}
}

func (s *InMemoryStore) SaveTransaction(tx models.Transaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transactions[tx.UserID] = append(s.transactions[tx.UserID], tx)
	return nil
}

func (s *InMemoryStore) RecentTransactions(userID string, since time.Time) ([]models.Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	all := s.transactions[userID]
	out := make([]models.Transaction, 0, len(all))
	for _, tx := range all {
		if tx.CreatedAt.After(since) {
			out = append(out, tx)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *InMemoryStore) SaveAlert(a models.Alert) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a.ID == "" {
		return fmt.Errorf("alert ID required")
	}
	s.alerts[a.ID] = a
	return nil
}

func (s *InMemoryStore) ListAlerts(resolvedFilter *bool) ([]models.Alert, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Alert, 0, len(s.alerts))
	for _, a := range s.alerts {
		if resolvedFilter != nil && a.Resolved != *resolvedFilter {
			continue
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (s *InMemoryStore) ResolveAlert(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.alerts[id]
	if !ok {
		return fmt.Errorf("alert %s not found", id)
	}
	a.Resolved = true
	s.alerts[id] = a
	return nil
}

func (s *InMemoryStore) AppendAudit(e models.AuditEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.audit = append(s.audit, e)
	return nil
}

func (s *InMemoryStore) ListAudit(transactionID string) ([]models.AuditEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.AuditEntry, 0)
	for _, e := range s.audit {
		if e.TransactionID == transactionID {
			out = append(out, e)
		}
	}
	return out, nil
}