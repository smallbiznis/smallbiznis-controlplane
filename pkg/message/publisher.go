package message

import (
	"context"
	"encoding/json"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var PublisherMessage = fx.Module("confluent.publisher", fx.Provide(NewProducer))

type Publisher interface {
	Publish(ctx context.Context, topic string, key string, value any) error
	Close() error
}

type Publish struct {
	producer *kafka.Producer
}

func NewProducer(cfg *kafka.ConfigMap) (*Publish, error) {
	p, err := kafka.NewProducer(cfg)
	if err != nil {
		return nil, err
	}

	return &Publish{
		producer: p,
	}, nil
}

func (p *Publish) Publish(ctx context.Context, topic, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	deliveryChan := make(chan kafka.Event, 1)

	err = p.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(key),
		Value:          data,
	}, deliveryChan)
	if err != nil {
		return err
	}

	go func() {
		defer close(deliveryChan)
		e := <-deliveryChan
		m := e.(*kafka.Message)
		if m.TopicPartition.Error != nil {
			zap.L().Error("Kafka delivery failed", zap.Error(m.TopicPartition.Error))
		} else {
			zap.L().Debug("Kafka message delivered", zap.String("topic", *m.TopicPartition.Topic))
		}
	}()

	return nil
}

func (p *Publish) Close() error {
	p.producer.Flush(5000)
	p.producer.Close()
	return nil
}
