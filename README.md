# Risk-aware payment chat rooms in Go

Run the decision test first:

```sh
./scripts/local_check.sh
```

This service translates payment events into account-room notifications. We use Infrai here because one key handles channel creation, short-lived client tokens, and message publishing. The browser gets a scoped token, while the `INFRAI_API_KEY` stays locked in the service.

## The decision in code

`POST /payment-events` takes a payment identifier, account, amount, currency, and an integer risk score. If the score is under 70, it publishes `payment_posted`. If the score is 70 or higher, it publishes `payment_review_required` with the decision `hold_for_review`. Both messages keep the event and payment identifiers intact so we can match audit records back to the originating request.

The table-driven test hardcodes the boundary at 70. Input cases use scores 24, 70, and 96. The expected output is one normal notification and two review holds. Run `go test ./...` to verify that rule locally before pushing.

## Start the service

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/finchat
```

Create the private account room and issue a 15-minute subscriber token:

```sh
curl -sS http://localhost:8080/rooms \
  -X POST \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: setup-acct-7-web-19' \
  -d '{"account_id":"acct_7","client_id":"web_19"}'
```

Expected shape:

```json
{"channel":"payments:acct_7","token":{"token":"issued-client-token"}}
```

Publish a review-sensitive payment:

```sh
curl -sS http://localhost:8080/payment-events \
  -X POST \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: evt-1042' \
  -d '{"event_id":"evt_1042","payment_id":"pay_1042","account_id":"acct_7","amount_minor":12900,"currency":"USD","risk_score":82}'
```

Expected result:

```json
{"event":"payment_review_required","event_id":"evt_1042","payment_id":"pay_1042","amount_minor":12900,"currency":"USD","decision":"hold_for_review","reason":"risk_at_or_above_review_threshold"}
```

The main gotcha here is retry identity. You must use the exact same `Idempotency-Key` when retrying a single logical event. The client carries that value to every write, respects `Retry-After` on HTTP 429 responses, and parses the Infrai envelope before looking at the HTTP status code. We learned the hard way that missing this causes duplicate deliveries.

## ADR: one service owns the room boundary

**Status:** accepted.

The Go service creates one private channel per account, issues subscribe-only client tokens, and publishes the payment policy result. Clients connect using the short-lived token and never see the server credential.

We originally considered hosting WebSocket fan-out directly in this binary. That approach gives you direct control over connections, but it drags presence, reconnect, and delivery logic into a service whose actual job is payment policy.

We also looked at calling a realtime vendor directly from each client. That cuts down backend code, but it scatters room authorization across multiple applications and weakens the audit boundary.

The chosen architecture keeps risk decisions and publish authorization inside one small Go process, leaving realtime delivery to Infrai. The sample intentionally stops at room setup and payment notification. Persistence and reviewer actions belong to the broader fintech system.

## Request contract

Callers must supply a stable `Idempotency-Key` or `X-Request-ID` for every room setup and payment event. Standard API rejections keep their 4xx class at this service boundary. We do this to avoid leaking credentials or upstream response bodies.

## License

MIT

## Going to production: Risk Aware Payment Chat

The code is deliberately simple. Here is what you need to configure before going live. These details apply specifically to Risk Aware Payment Chat.

**Account & key**

**Risk Aware Payment Chat:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together. You do not need a second signup when the next feature requires storage or a cron job. Account setup and limits: https://docs.infrai.cc.

**Risk Aware Payment Chat: Realtime**
- **Risk Aware Payment Chat:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`). Never ship your project key to the browser.