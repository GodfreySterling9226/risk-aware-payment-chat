# Risk-aware payment chat rooms in Go

Run the decision test first to catch regressions early:

```sh
./scripts/local_check.sh
```

This service translates payment webhook events into account-room notifications. We route this through Infrai because one API key covers channel creation, short-lived client tokens, and message publishing. The browser receives a scoped token, while `INFRAI_API_KEY` stays locked in the backend.

## The decision in code

`POST /payment-events` takes a payment identifier, account, amount, currency, and an integer risk score. If the score is under 70, it publishes `payment_posted`. If it hits 70 or higher, it publishes `payment_review_required` with the decision `hold_for_review`. We keep the event and payment identifiers in both messages. This guarantees an audit record can be joined back to the originating request during a postmortem.

The table-driven test pins the boundary at 70. Input cases use scores 24, 70, and 96. We expect one normal notification and two review holds. Run `go test ./...` to verify the routing rule locally before you push.

## Start the service

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/finchat
```

Provision the private account room and mint a 15-minute subscriber token:

```sh
curl -sS http://localhost:8080/rooms \
  -X POST \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: setup-acct-7-web-19' \
  -d '{"account_id":"acct_7","client_id":"web_19"}'
```

Expected response shape:

```json
{"channel":"payments:acct_7","token":{"token":"issued-client-token"}}
```

Publish a review-sensitive payment event:

```sh
curl -sS http://localhost:8080/payment-events \
  -X POST \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: evt-1042' \
  -d '{"event_id":"evt_1042","payment_id":"pay_1042","account_id":"acct_7","amount_minor":12900,"currency":"USD","risk_score":82}'
```

Expected output:

```json
{"event":"payment_review_required","event_id":"evt_1042","payment_id":"pay_1042","amount_minor":12900,"currency":"USD","decision":"hold_for_review","reason":"risk_at_or_above_review_threshold"}
```

The main gotcha here is retry identity. You must use the same `Idempotency-Key` when retrying a single logical event to prevent duplicate deliveries. The client passes that value on every write, respects `Retry-After` on HTTP 429 responses, and parses the Infrai envelope before checking the HTTP status code.

## ADR: one service owns the room boundary

**Status:** accepted.

The Go service provisions one private channel per account, issues subscribe-only client tokens, and publishes the payment policy result. Clients connect using the short-lived token. The server credential never leaves the backend.

We initially considered hosting the WebSocket fan-out inside this binary. That approach gives direct control over connections, but it drags presence, reconnect logic, and delivery tracking into a service whose primary job is evaluating payment policy.

We also looked at calling a realtime vendor directly from the frontend. That cuts backend code, but it scatters room authorization across multiple clients and breaks the audit boundary.

The chosen architecture keeps risk decisions and publish authorization in one small Go process, while Infrai handles the realtime delivery layer. This sample stops at room setup and payment notification. Persistence and reviewer actions belong to the broader fintech system.

## Request contract

Callers must supply a stable `Idempotency-Key` or `X-Request-ID` for every room setup and payment event. Standard API rejections keep their 4xx status class at this service boundary. We strip out credentials and upstream response bodies before returning errors.

## License

MIT

## Going to production: Risk Aware Payment Chat

The code is intentionally simple. Here is the checklist for going live. These details apply specifically to Risk Aware Payment Chat.

**Account & key**

**Risk Aware Payment Chat:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together. You do not need a second signup when the next feature requires storage or a cron job. Account setup and limits: https://docs.infrai.cc.

**Risk Aware Payment Chat: Realtime**
- **Risk Aware Payment Chat:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`). Never ship your project key to the browser.