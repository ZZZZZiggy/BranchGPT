package events

import (
	"context"
	"encoding/json"
	"github.com/redis/go-redis/v9"
	"go_chat_backend/models"
	"go_chat_backend/pkg/logging"
	"time"
)

const (
	DocumentEventChannel = "document:events"
)

type EventPublisher struct {
	redisClient *redis.Client
}

func NewEventPublisher(redisClient *redis.Client) *EventPublisher {
	return &EventPublisher{redisClient: redisClient}
}

func (p *EventPublisher) PublishDocumentEvent(event *models.DocumentEvent) error {
	event.Timestamp = time.Now()

	data, err := json.Marshal(event)
	if err != nil {
		logging.Logger.Error("fail PublishDocumentEvent", err)
		return err
	}
	ctx := context.Background()
	if err := p.redisClient.Publish(ctx, DocumentEventChannel, string(data)).Err(); err != nil {
		logging.Logger.Error("fail PublishDocumentEvent", err)
		return err
	}
	logging.Logger.Info("PublishDocumentEvent", "event", event)
	return nil
}

func (p *EventPublisher) SubscribeDocumentEvents(ctx context.Context) (<-chan *models.DocumentEvent, error) {
	pubsub := p.redisClient.Subscribe(ctx, DocumentEventChannel)
	if _, err := pubsub.Receive(ctx); err != nil {
		logging.Logger.Error("fail SubscribeDocumentEvents", err)
		return nil, err
	}
	ch := make(chan *models.DocumentEvent, 100)

	// goroutine to listen
	go func() {
		defer close(ch)
		defer func(pubsub *redis.PubSub) {
			err := pubsub.Close()
			if err != nil {
				logging.Logger.Error("fail SubscribeDocumentEvents", err)
			}
		}(pubsub)

		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-pubsub.Channel():
				var event models.DocumentEvent
				if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
					logging.Logger.Error("Failed to unmarshal event", "error", err)
					continue
				}

				select {
				case ch <- &event:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return ch, nil
}
