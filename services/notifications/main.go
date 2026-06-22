package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/osship/osship/packages/kafka"
	"github.com/osship/osship/packages/observability"
	kafkago "github.com/segmentio/kafka-go"
)

func main() {
	port := env("PORT", "8086")
	brokers := env("KAFKA_BROKERS", "kafka:9092")
	resendKey := env("RESEND_API_KEY", "")
	fromEmail := env("RESEND_FROM_EMAIL", "noreply@osship.local")

	ctx := context.Background()
	topics := []string{"listing.events", "enrollment.events", "payment.events", "session.events", "mentor.events"}

	for _, topic := range topics {
		go consumeTopic(ctx, brokers, topic, resendKey, fromEmail)
	}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(observability.PrometheusMiddleware("notifications"))
	r.Get("/health", observability.HealthHandler("notifications"))
	r.Get("/metrics", observability.MetricsHandler().ServeHTTP)

	log.Printf("notifications listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func consumeTopic(ctx context.Context, brokers, topic, resendKey, fromEmail string) {
	reader := kafka.NewReader(brokers, topic, "notifications-group")
	defer reader.Close()
	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			log.Printf("kafka read error [%s]: %v", topic, err)
			continue
		}
		var event kafka.Event
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			continue
		}
		subject, body := templateForEvent(event.Type, event.Payload)
		if subject != "" {
			if err := sendEmail(resendKey, fromEmail, subject, body); err != nil {
				log.Printf("email error: %v", err)
			} else {
				log.Printf("sent notification: %s", event.Type)
			}
		}
	}
}

func templateForEvent(eventType string, payload json.RawMessage) (string, string) {
	switch eventType {
	case "enrollment.confirmed":
		return "Enrollment Confirmed", "Your mentorship enrollment has been confirmed. Welcome to OSShip!"
	case "payout.recorded":
		return "Payout Recorded", "A payout has been recorded in your mentorship ledger."
	case "session.scheduled":
		return "Session Scheduled", "A new live mentorship session has been scheduled. Check your dashboard."
	case "session.reminder_due":
		return "Session Reminder", "Your mentorship session starts soon. Join via your dashboard."
	case "mentor.approved":
		return "Mentor Application Approved", "Congratulations! You can now publish mentorship listings."
	case "listing.created":
		return "Listing Created", "Your mentorship listing is now live on OSShip."
	default:
		return "", ""
	}
}

func sendEmail(apiKey, from, subject, body string) error {
	if apiKey == "" {
		log.Printf("[dev] email: %s - %s", subject, body)
		return nil
	}
	payload := map[string]interface{}{
		"from":    from,
		"to":      []string{"notifications@osship.local"},
		"subject": subject,
		"html":    fmt.Sprintf("<p>%s</p>", body),
	}
	data, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend error: %s", string(b))
	}
	return nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var _ = strings.TrimSpace
var _ kafkago.Message
