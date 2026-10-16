package post_service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

type RabbitMQPublisher struct {
	connection *amqp091.Connection
	channel    *amqp091.Channel
	queue      string
	confirms   <-chan amqp091.Confirmation
	mu         sync.Mutex
	closed     bool
}

func NewRabbitMQPublisher(amqpURL, queue string) (*RabbitMQPublisher, error) {
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
		return nil, fmt.Errorf("open RabbitMQ channel: %w", err)
	}
	if _, err := channel.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return nil, fmt.Errorf("declare RabbitMQ queue: %w", err)
	}
	if err := channel.Confirm(false); err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return nil, fmt.Errorf("enable RabbitMQ publisher confirms: %w", err)
	}

	return &RabbitMQPublisher{
		connection: connection,
		channel:    channel,
		queue:      queue,
		confirms:   channel.NotifyPublish(make(chan amqp091.Confirmation, 1)),
	}, nil
}

func (p *RabbitMQPublisher) PublishPostCreated(ctx context.Context, event PostCreatedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode post-created event: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return errors.New("RabbitMQ publisher is closed")
	}
	if err := p.channel.PublishWithContext(ctx, "", p.queue, false, false, amqp091.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp091.Persistent,
		Timestamp:    time.Unix(event.Timestamp, 0).UTC(),
		Body:         body,
	}); err != nil {
		return fmt.Errorf("publish post-created event: %w", err)
	}

	select {
	case confirmation, ok := <-p.confirms:
		if !ok {
			return errors.New("RabbitMQ publisher confirmation channel closed")
		}
		if !confirmation.Ack {
			return errors.New("RabbitMQ rejected post-created event")
		}
		return nil
	case <-ctx.Done():
		p.closed = true
		_ = p.channel.Close()
		_ = p.connection.Close()
		return ctx.Err()
	}
}

func (p *RabbitMQPublisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	channelErr := p.channel.Close()
	connectionErr := p.connection.Close()
	if channelErr != nil {
		return channelErr
	}
	return connectionErr
}
