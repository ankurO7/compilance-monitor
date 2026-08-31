package worker

import (
	"log"
	"time"

	"github.com/ankurO7/compilance-monitor/internal/models"
	"github.com/ankurO7/compilance-monitor/internal/rules"
	"github.com/ankurO7/compilance-monitor/internal/store"
	
)

type Pool struct {
	queue chan models.Transaction
	numWorkers int
	st			store.Store
	rules		[]rules.Rule
	idFn		func() string
}

func NewPool(numWorkers int, queueSize int, st store.Store, ruleSet []rules.Rule, idFn func() string) *Pool {
	return &Pool{
		queue: make(chan models.Transaction, queueSize),
		numWorkers: numWorkers,
		st: 		st,
		rules:		ruleSet,
		idFn: 		idFn,
	}
}

// Submit enqueues a transaction. Returns false if the queue is full
// (backpressure signal to the HTTP handler).

func (p *Pool) Submit(tx models.Transaction) bool {
	select {
	case p.queue <- tx :
		return true
	default:
		return false
	}
}

// Start launches the worker goroutines. Non-blocking - call once at startup.

func (p *Pool) Start(stopCh <-chan struct{}) {
	for i := 0; i < p.numWorkers; i++ {
		go p.runWorker(stopCh)
	}
}

func (p *Pool) runWorker(stopCh <-chan struct{}){
	for {
		select {
		case <-stopCh:
			return
		case tx := <-p.queue:
			p.evaluate(tx)
		}
	}
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

func (p *Pool) QueueDepth() int {
	return len(p.queue)
}