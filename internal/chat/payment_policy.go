package chat

import "fmt"

type PaymentEvent struct {
	EventID     string `json:"event_id"`
	PaymentID   string `json:"payment_id"`
	AccountID   string `json:"account_id"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
	RiskScore   int    `json:"risk_score"`
}

type Notification struct {
	Event       string `json:"event"`
	EventID     string `json:"event_id"`
	PaymentID   string `json:"payment_id"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
	Decision    string `json:"decision"`
	Reason      string `json:"reason"`
}

func DecidePayment(event PaymentEvent) (Notification, error) {
	if event.EventID == "" || event.PaymentID == "" || event.AccountID == "" {
		return Notification{}, fmt.Errorf("event_id, payment_id, and account_id are required")
	}
	if event.AmountMinor <= 0 || event.Currency == "" {
		return Notification{}, fmt.Errorf("amount_minor must be positive and currency is required")
	}
	if event.RiskScore < 0 || event.RiskScore > 100 {
		return Notification{}, fmt.Errorf("risk_score must be between 0 and 100")
	}

	n := Notification{
		Event:       "payment_posted",
		EventID:     event.EventID,
		PaymentID:   event.PaymentID,
		AmountMinor: event.AmountMinor,
		Currency:    event.Currency,
		Decision:    "notify",
		Reason:      "risk_below_review_threshold",
	}
	if event.RiskScore >= 70 {
		n.Event = "payment_review_required"
		n.Decision = "hold_for_review"
		n.Reason = "risk_at_or_above_review_threshold"
	}
	return n, nil
}
