# Compliance Monitor

A backend service, written in Go, that checks financial transactions
for suspicious patterns in real time.

**This is a simulation not a certified compliance product, and it doesn't use real sanctions or
watchlist data.**

**Status: runs locally, tested and working. Not yet deployed. Data is
stored in memory only, so it resets when the server restarts.**

## What problem this is based on

Banks and financial companies are required to watch transactions for
money laundering and fraud, and report anything suspicious. This
project is a simplified version of that kind of system — it doesn't
handle real money or connect to any real bank, but it checks synthetic
transactions using the same kinds of patterns real systems look for.

## What it does

You send it a transaction (a user ID, an amount, and who they paid).
It checks that transaction against three rules:

- **Structuring** — flags someone making several transactions just
  under a reporting limit, which is a known way people try to avoid
  scrutiny on one big transfer.
- **Velocity anomaly** — flags a transaction that's way bigger than a
  user's normal spending, using basic statistics (standard deviation)
  to spot the outlier.
- **Watchlist** — flags a transaction if the person being paid is on a
  blocklist.

Every check gets logged, whether it flags something or not, so you can
always see why a transaction was or wasn't flagged.

## How it's built

- Transactions come in through an HTTP API and get put in a queue
  instead of being checked immediately, so the API responds fast.
- A set of background workers pulls transactions off the queue and
  runs the checks. Multiple transactions get processed at the same
  time, but transactions from the *same* user are always processed in
  the order they came in (more on why below).
- Data is currently stored in memory (a Go map, till now).

## Run it locally

```bash
go build -o server ./cmd/server
./server
# runs on :8080, or set the PORT environment variable
```

## Try it

```bash
curl localhost:8080/health

curl -X POST localhost:8080/demo/seed

curl localhost:8080/alerts

curl -X POST localhost:8080/transactions \
  -H "Content-Type: application/json" \
  -d '{"user_id":"alice","amount":250.00,"currency":"USD","counterparty":"corner-store"}'

curl -X POST localhost:8080/alerts/<alert-id>/resolve

curl localhost:8080/audit/<transaction-id>
```

`/demo/seed` generates fake transactions that are designed to trigger
each of the three rules, so you can see it working without needing
real data.

## Example output

```json
[
  {
    "rule_name": "watchlist",
    "severity": "high",
    "reason": "counterparty \"blocked-entity-1\" matches watchlist"
  },
  {
    "rule_name": "structuring",
    "severity": "high",
    "reason": "4 transactions between 9000.00 and 10000.00 within 72h0m0s (possible structuring)"
  },
  {
    "rule_name": "velocity_anomaly",
    "severity": "medium",
    "reason": "amount 15000.00 is 1182.6 standard deviations above the 8-transaction average of 100.00"
  }
]
```

Also tested by hand: a normal transaction triggers no alerts, and a
transaction sent to a known watchlisted name triggers exactly one.

## API

| Method | Path                      | Description                              |
|--------|---------------------------|-------------------------------------------|
| POST   | `/transactions`           | Submit a transaction for checking         |
| GET    | `/alerts?resolved=false`  | List alerts, optionally filtered          |
| POST   | `/alerts/{id}/resolve`    | Mark an alert resolved                    |
| GET    | `/audit/{transactionID}`  | Full history of checks for a transaction  |
| GET    | `/health`                 | Health check + current queue size         |
| POST   | `/demo/seed`              | Generate fake data that triggers each rule|


## Roadmap (to be done)

- Add a real database (Postgres) instead of in-memory storage. The
  storage code is written behind an interface so this should be a
  contained change.
- Maybe a simple dashboard to view alerts without using curl.
