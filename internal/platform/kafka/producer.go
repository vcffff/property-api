package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	kafkago "github.com/segmentio/kafka-go"
)

type Producer struct {
	writer *kafkago.Writer
}

func NewProducer(brokers []string) *Producer {
	writer := &kafkago.Writer{
		Addr:         kafkago.TCP(brokers...),
		Topic:        "property.events",
		Balancer:     &kafkago.Hash{},
		RequiredAcks: kafkago.RequireAll,
	}

	return &Producer{writer: writer}
}

func (p *Producer) PublishPropertyCreated(ctx context.Context, event PropertyCreatedEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	err = p.writer.WriteMessages(ctx, kafkago.Message{
		Key:   []byte(event.PropertyID),
		Value: data,
	})
	if err != nil {
		return fmt.Errorf("publish property event: %w", err)
	}

	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
