package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/example/fintech-chat-rooms/internal/chat"
	"github.com/example/fintech-chat-rooms/internal/infrai"
)

type server struct {
	realtime *infrai.Client
}

type roomRequest struct {
	AccountID string `json:"account_id"`
	ClientID  string `json:"client_id"`
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	s := &server{realtime: infrai.New(key)}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /rooms", s.createRoom)
	mux.HandleFunc("POST /payment-events", s.publishPayment)
	log.Println("finchat listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func (s *server) createRoom(w http.ResponseWriter, r *http.Request) {
	var input roomRequest
	if err := decodeJSON(r, &input); err != nil || input.AccountID == "" || input.ClientID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account_id and client_id are required"})
		return
	}
	channel := "payments:" + input.AccountID
	requestID := requestID(r)
	if requestID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Idempotency-Key or X-Request-ID is required"})
		return
	}
	if _, err := s.realtime.CreateChannel(r.Context(), infrai.CreateChannelInput{
		Channel: channel, Type: "private", Vendor: "tencent_im",
	}, requestID+":channel"); err != nil {
		writeUpstreamError(w, err)
		return
	}
	token, err := s.realtime.IssueToken(r.Context(), infrai.IssueTokenInput{
		ClientID: input.ClientID, Channels: []string{channel},
		Capabilities: []string{"subscribe"}, TTLSeconds: 900,
	}, requestID+":token")
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"channel": channel, "token": json.RawMessage(token)})
}

func (s *server) publishPayment(w http.ResponseWriter, r *http.Request) {
	var event chat.PaymentEvent
	if err := decodeJSON(r, &event); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	notification, err := chat.DecidePayment(event)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	id := requestID(r)
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Idempotency-Key or X-Request-ID is required"})
		return
	}
	channel := "payments:" + event.AccountID
	_, err = s.realtime.Publish(r.Context(), infrai.PublishInput{
		Channel: channel, Event: notification.Event, Data: notification, AccountID: event.AccountID,
	}, id+":publish")
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, notification)
}

func requestID(r *http.Request) string {
	id := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if id == "" {
		id = strings.TrimSpace(r.Header.Get("X-Request-ID"))
	}
	return id
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func writeUpstreamError(w http.ResponseWriter, err error) {
	var apiErr *infrai.APIError
	if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
		writeJSON(w, apiErr.HTTPStatus, map[string]string{"error": apiErr.Message, "code": apiErr.Code})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "realtime request could not be completed"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}
