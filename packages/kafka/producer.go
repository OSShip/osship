package kafka

import (
	"context"
	"encoding/json"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

type Event struct {
	EventID       string          `json:"event_id"`
	Type          string          `json:"type"`
	Timestamp     time.Time       `json:"timestamp"`
	Payload       json.RawMessage `json:"payload"`
	SchemaVersion string          `json:"schema_version"`
}

type Producer struct {
	writer *kafkago.Writer
}

func NewProducer(brokers, topic string) *Producer {
	return &Producer{
		writer: &kafkago.Writer{
			Addr:     kafkago.TCP(brokers),
			Topic:    topic,
			Balancer: &kafkago.LeastBytes{},
		},
	}
}

func (p *Producer) Publish(ctx context.Context, eventType string, payload interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	event := Event{
		EventID:       generateID(),
		Type:          eventType,
		Timestamp:     time.Now().UTC(),
		Payload:       payloadBytes,
		SchemaVersion: "1.0",
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.writer.WriteMessages(ctx, kafkago.Message{Value: data})
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomSuffix()
}

func randomSuffix() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[time.Now().UnixNano()%int64(len(chars))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}

func NewReader(brokers, topic, groupID string) *kafkago.Reader {
	return kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  []string{brokers},
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6,
	})
}
