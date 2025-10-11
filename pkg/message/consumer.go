package message

import (
	"context"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var ConsumerMessage = fx.Module("confluent.consumer", fx.Provide(NewConsumer))

type ConsumerHandler interface {
	Topic() string
	Handle(ctx context.Context, key []byte, value []byte) error
}

type Consumer struct {
	consumer *kafka.Consumer
	handlers map[string]ConsumerHandler
	topic    string
}

func NewConsumer(cfg *kafka.ConfigMap, handlers []ConsumerHandler) (*Consumer, error) {
	c, err := kafka.NewConsumer(cfg)
	if err != nil {
		return nil, err
	}

	handlerMap := make(map[string]ConsumerHandler)
	topics := []string{}

	for _, h := range handlers {
		handlerMap[h.Topic()] = h
		topics = append(topics, h.Topic())
	}

	if err := c.SubscribeTopics(topics, nil); err != nil {
		return nil, err
	}

	return &Consumer{
		consumer: c,
		handlers: handlerMap,
	}, nil

}

func (c *Consumer) Start(ctx context.Context) {
	go func() {
		zap.L().Info("Kafka consumer started")

		for {
			select {
			case <-ctx.Done():
				zap.L().Info("Kafka consumer stopped")
				return

			default:
				msg, err := c.consumer.ReadMessage(-1)
				if err != nil {
					zap.L().Error("Kafka read error", zap.Error(err))
					continue
				}

				handler, ok := c.handlers[*msg.TopicPartition.Topic]
				if !ok {
					zap.L().Warn("No handler for topic", zap.String("topic", *msg.TopicPartition.Topic))
					continue
				}

				go func(m *kafka.Message) {
					if err := handler.Handle(ctx, m.Key, m.Value); err != nil {
						zap.L().Error("Handler error", zap.String("topic", *m.TopicPartition.Topic), zap.Error(err))
					}
				}(msg)
			}
		}
	}()
}

func (c *Consumer) Close() error {
	return c.consumer.Close()
}
