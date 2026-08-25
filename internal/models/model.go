// Type of transaction - checked for compilance.
// rules for transaction - 

package models

import "time"

type Transaction struct {
	ID				string 	`json:"id"`
	UserID 			string 	`json:"user_id"`
	Amount			int64	`json:"amount"`
	Currency 		string 	`json:"currency"`
	Counterparty 	string 	`json:"counterparty"`
	CreatedAt 		time.Time `json:"created_at"`
}

type Alert struct {
	ID 				string 	`json:"id"`
	TransactionID 	string 	`json:"transac_id"`
	UserID			string	`json:"user_id"`
	RuleName		string 	`json:"rule_name"`
	Severity		string 	`json:"severity"`
	Reason			string	`json:"reason"`
	CreatedAt		time.Time	`json:"created_at"`
	Resolved		bool	`json:"resolved"`

}

type AuditEntry struct {
	ID            string    `json:"id"`
	TransactionID string    `json:"transaction_id"`
	RuleName      string    `json:"rule_name"`
	Outcome       string    `json:"outcome"` // "flagged" or "clear"
	Detail        string    `json:"detail"`
	CreatedAt     time.Time `json:"created_at"`
}