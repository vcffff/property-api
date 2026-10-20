package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	kafkago "github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader *kafkago.Reader
}

func NewConsumer(brokers []string) *Consumer {
	return &Consumer{
		reader: kafkago.NewReader(kafkago.ReaderConfig{
			Brokers:  brokers,
			GroupID:  "property-consumer-group",
			Topic:    "property.events",
			MinBytes: 1,
			MaxBytes: 10e6,
		}),
	}

}
func (c *Consumer) Start(ctx context.Context) error {
	for {
		m, err := c.reader.FetchMessage(ctx)

		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("fetch message: %w", err)
		}

		var event PropertyCreatedEvent
		if err := json.Unmarshal(m.Value, &event); err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			continue
		}

		log.Printf("Received property.created event: id=%s property_id=%s", event.EventID, event.PropertyID)
		if err := c.reader.CommitMessages(ctx, m); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("commit message: %w", err)
		}
		log.Printf("Committed message: offset=%d", m.Offset)

	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
