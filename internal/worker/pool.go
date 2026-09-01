package worker

import (
	"log"
	"time"

	"github.com/ankurO7/compilance-monitor/internal/models"
	"github.com/ankurO7/compilance-monitor/internal/rules"
	"github.com/ankurO7/compilance-monitor/internal/store"
	
)

type Pool struct {
	queues     []chan models.Transaction
	numWorkers int
	st         store.Store
	rules      []rules.Rule
	idFn       func() string
}

func NewPool(numWorkers int, queueSize int, st store.Store, ruleSet []rules.Rule, idFn func() string) *Pool {
	queues := make([]chan models.Transaction, numWorkers)
	for i := range queues {
		queues[i] = make(chan models.Transaction, queueSize)
	}
	return &Pool{queues: queues, numWorkers: numWorkers, st: st, rules: ruleSet, idFn: idFn}
}

func hashUser(userID string) int {
	h := 0
	for _, c := range userID {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return h
}

func (p *Pool) Submit(tx models.Transaction) bool {
	idx := hashUser(tx.UserID) % p.numWorkers
	select {
	case p.queues[idx] <- tx:
		return true
	default:
		return false
	}
}

func (p *Pool) Start(stopCh <-chan struct{}) {
	for i := 0; i < p.numWorkers; i++ {
		go p.runWorker(i, stopCh)
	}
}

func (p *Pool) runWorker(idx int, stopCh <-chan struct{}) {
	for {
		select {
		case <-stopCh:
			return
		case tx := <-p.queues[idx]:
			p.evaluate(tx)
		}
	}
}

func (p *Pool) QueueDepth() int {
	total := 0
	for _, q := range p.queues {
		total += len(q)
	}
	return total
}

func (p *Pool) evaluate(tx models.Transaction) {
	history, err := p.st.RecentTransactions(tx.UserID, tx.CreatedAt.Add(-90*24*time.Hour))
	if err != nil {
		log.Printf("worker: failed to load history for %s: %v", tx.UserID, err)
		return
	}

	// save the transaction before evaluating rules, so any 
	// other transaction for this user processed right after (maybe)
	// by a different worker) sees it in history. Matters for rules like
	// structuring that depend on multiple transactions.

	if err := p.st.SaveTransaction(tx); err != nil {
		log.Printf("worker: failed to save transaction: %v", err)
		return
	}

	for _, rule := range p.rules {
		res := rule.Evaluate(tx, history)

		outcome := "clear"
		detail := "no issue found"
		if res.Flagged {
			outcome = "flagged"
			detail = res.Reason

			alert := models.Alert{
				ID:		p.idFn(),
				TransactionID: tx.ID,
				UserID: tx.UserID,
				RuleName: res.RuleName,
				Severity: res.Severity,
				Reason: res.Reason,
				CreatedAt: time.Now().UTC(),
			}
			if err := p.st.SaveAlert(alert); err != nil {
				log.Printf("worker: failed to save alert: %v", err)
			}
		}

		entry := models.AuditEntry{
			ID:		p.idFn(),
			TransactionID: tx.ID,
			RuleName: rule.Name(),
			Outcome: outcome,
			Detail: detail,
			CreatedAt: time.Now().UTC(),
		}

		if err := p.st.AppendAudit(entry); err != nil{
			log.Printf("worker: failed to append audit entry: %v", err)
		}
	}

}