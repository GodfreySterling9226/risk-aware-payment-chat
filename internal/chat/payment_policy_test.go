package chat

import "testing"

func TestDecidePayment(t *testing.T) {
	tests := []struct {
		name         string
		riskScore    int
		wantEvent    string
		wantDecision string
	}{
		{name: "routine payment is posted", riskScore: 24, wantEvent: "payment_posted", wantDecision: "notify"},
		{name: "threshold payment waits for review", riskScore: 70, wantEvent: "payment_review_required", wantDecision: "hold_for_review"},
		{name: "high risk payment waits for review", riskScore: 96, wantEvent: "payment_review_required", wantDecision: "hold_for_review"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notification, err := DecidePayment(PaymentEvent{
				EventID: "evt_1042", PaymentID: "pay_1042", AccountID: "acct_7",
				AmountMinor: 12900, Currency: "USD", RiskScore: tt.riskScore,
			})
			if err != nil {
				t.Fatal(err)
			}
			if notification.Event != tt.wantEvent || notification.Decision != tt.wantDecision {
				t.Fatalf("got event=%q decision=%q", notification.Event, notification.Decision)
			}
			if notification.EventID != "evt_1042" || notification.PaymentID != "pay_1042" {
				t.Fatalf("audit identifiers were not preserved: %+v", notification)
			}
		})
	}
}
