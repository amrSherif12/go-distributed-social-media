package feed_service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/rabbitmq/amqp091-go"
)

type RabbitMQConsumer struct {
	connection *amqp091.Connection
	channel    *amqp091.Channel
	queue      string
}

func NewRabbitMQConsumer(amqpURL, queue string) (*RabbitMQConsumer, error) {
	if queue == "" {
		return nil, errors.New("RabbitMQ queue name is required")
	}
	connection, err := amqp091.Dial(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}
	channel, err := connection.Channel()
	if err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("open RabbitMQ consumer channel: %w", err)
	}
	if _, err := channel.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return nil, fmt.Errorf("declare post-created queue: %w", err)
	}
	if err := channel.Qos(10, 0, false); err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return nil, fmt.Errorf("configure post-created consumer: %w", err)
	}
	return &RabbitMQConsumer{connection: connection, channel: channel, queue: queue}, nil
}

func (c *RabbitMQConsumer) Run(ctx context.Context, handle func(context.Context, PostCreatedEvent) error) error {
	deliveries, err := c.channel.Consume(c.queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("start post-created consumer: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return errors.New("post-created delivery channel closed")
			}
			var event PostCreatedEvent
			if err := json.Unmarshal(delivery.Body, &event); err != nil {
				log.Printf("discarding malformed post-created event: %v", err)
				if nackErr := delivery.Nack(false, false); nackErr != nil {
					return fmt.Errorf("reject malformed post-created event: %w", nackErr)
				}
				continue
			}
			if err := handle(ctx, event); err != nil {
				log.Printf("retrying post-created event after processing failure: %v", err)
				if nackErr := delivery.Nack(false, true); nackErr != nil {
					return fmt.Errorf("requeue post-created event after handler failure: %w", nackErr)
				}
				continue
			}
			if err := delivery.Ack(false); err != nil {
				return fmt.Errorf("acknowledge post-created event: %w", err)
			}
		}
	}
}

func (c *RabbitMQConsumer) Close() error {
	channelErr := c.channel.Close()
	connectionErr := c.connection.Close()
	if channelErr != nil {
		return channelErr
	}
	return connectionErr
}
