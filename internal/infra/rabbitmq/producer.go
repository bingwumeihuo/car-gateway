package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"vehicle-gateway/internal/config"
)

type RabbitMQProducer struct {
	conn       *amqp.Connection
	ch         *amqp.Channel
	cfg        config.RabbitMQConfig
	logger     *zap.Logger
	mu         sync.Mutex
	isClosed   bool
	reconnectC chan struct{}
}

func NewRabbitMQProducer(cfg config.RabbitMQConfig, logger *zap.Logger) (*RabbitMQProducer, error) {
	p := &RabbitMQProducer{
		cfg:        cfg,
		logger:     logger,
		reconnectC: make(chan struct{}, 1),
	}

	// Try to connect initially in background to avoid blocking
	// If it fails, Produce will try again
	go func() {
		p.logger.Info("Attempting initial RabbitMQ connection", zap.String("url", cfg.URL))
		if err := p.connect(); err != nil {
			p.logger.Warn("Initial RabbitMQ connection failed (will retry on produce)", zap.Error(err))
			p.signalReconnect()
		} else {
			p.logger.Info("Successfully connected to RabbitMQ")
		}
	}()

	// Start reconnection loop immediately to handle background retries
	go p.handleReconnect()

	return p, nil
}

func (p *RabbitMQProducer) connect() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Build connection URL with virtual host if specified
	connURL := p.cfg.URL
	if p.cfg.VirtualHost != "" {
		vhost := p.cfg.VirtualHost
		// If vhost starts with /, escape it to %2f for URL usage
		// e.g. "/dev" -> "%2fdev"
		if strings.HasPrefix(vhost, "/") {
			vhost = "%2f" + vhost[1:]
		}

		// Check if URL already contains a virtual host
		if !strings.Contains(connURL, "/") || strings.HasSuffix(connURL, "/") {
			if strings.HasSuffix(connURL, "/") {
				connURL += vhost
			} else {
				connURL += "/" + vhost
			}
		} else {
			// If URL has path, replace it with the virtual host
			parts := strings.Split(connURL, "/")
			connURL = strings.Join(parts[:3], "/") + "/" + vhost
		}
	}

	// Mask password for logging
	maskedURL := connURL
	if u, err := amqp.ParseURI(connURL); err == nil {
		u.Password = "******"
		maskedURL = u.String()
	}
	p.logger.Debug("Connecting to RabbitMQ", zap.String("url", maskedURL))
	conn, err := amqp.Dial(connURL)
	if err != nil {
		p.logger.Error("Failed to connect to RabbitMQ", zap.Error(err))
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	p.logger.Debug("Opening RabbitMQ channel")
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		p.logger.Error("Failed to open RabbitMQ channel", zap.Error(err))
		return fmt.Errorf("failed to open a channel: %w", err)
	}

	// Declare exchange (idempotent)
	p.logger.Debug("Declaring RabbitMQ exchange", zap.String("exchange", p.cfg.Exchange))
	err = ch.ExchangeDeclare(
		p.cfg.Exchange, // name
		"topic",        // type
		true,           // durable
		false,          // auto-deleted
		false,          // internal
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		p.logger.Error("Failed to declare RabbitMQ exchange", zap.Error(err))
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare Queue if configured
	if p.cfg.QueueName != "" {
		p.logger.Debug("Declaring RabbitMQ queue", zap.String("queue", p.cfg.QueueName))
		_, err = ch.QueueDeclare(
			p.cfg.QueueName, // name
			true,            // durable
			false,           // delete when unused
			false,           // exclusive
			false,           // no-wait
			nil,             // arguments
		)
		if err != nil {
			ch.Close()
			conn.Close()
			p.logger.Error("Failed to declare RabbitMQ queue", zap.Error(err))
			return fmt.Errorf("failed to declare queue: %w", err)
		}

		// Bind Queue to Exchange
		p.logger.Debug("Binding RabbitMQ queue to exchange",
			zap.String("queue", p.cfg.QueueName),
			zap.String("exchange", p.cfg.Exchange),
			zap.String("routing_key", p.cfg.RoutingKey))

		err = ch.QueueBind(
			p.cfg.QueueName,  // queue name
			p.cfg.RoutingKey, // routing key
			p.cfg.Exchange,   // exchange
			false,
			nil,
		)
		if err != nil {
			ch.Close()
			conn.Close()
			p.logger.Error("Failed to bind RabbitMQ queue", zap.Error(err))
			return fmt.Errorf("failed to bind queue: %w", err)
		}
	}

	p.conn = conn
	p.ch = ch
	p.isClosed = false

	// Monitor connection close
	go func() {
		<-conn.NotifyClose(make(chan *amqp.Error))
		p.signalReconnect()
	}()

	p.logger.Info("Connected to RabbitMQ", zap.String("url", p.cfg.URL))
	return nil
}

func (p *RabbitMQProducer) signalReconnect() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.isClosed {
		select {
		case p.reconnectC <- struct{}{}:
		default:
		}
	}
}

func (p *RabbitMQProducer) handleReconnect() {
	for range p.reconnectC {
		p.logger.Warn("RabbitMQ connection lost, attempting to reconnect...")
		for {
			if err := p.connect(); err != nil {
				p.logger.Error("Failed to reconnect to RabbitMQ", zap.Error(err))
				time.Sleep(5 * time.Second)
				continue
			}
			p.logger.Info("Reconnected to RabbitMQ")
			break
		}
	}
}

// Produce sends data to the exchange
func (p *RabbitMQProducer) Produce(ctx context.Context, topic string, key string, data interface{}) error {
	p.mu.Lock()
	if p.isClosed {
		p.mu.Unlock()
		return fmt.Errorf("connection is closed")
	}

	// Check connection
	if p.ch == nil || p.ch.IsClosed() {
		p.mu.Unlock()
		// Trigger reconnect if not already trying
		p.signalReconnect()
		p.logger.Warn("RabbitMQ not connected, triggering reconnect")
		return fmt.Errorf("RabbitMQ not connected")
	}

	ch := p.ch
	p.mu.Unlock()

	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	routingKey := p.cfg.RoutingKey
	if key != "" {
		routingKey = key
	}

	err = ch.PublishWithContext(ctx,
		p.cfg.Exchange, // exchange
		routingKey,     // routing key
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
			Timestamp:   time.Now(),
		})
	fmt.Println("Published message to RabbitMQ", zap.String("exchange", p.cfg.Exchange), zap.String("routing_key", routingKey))

	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

func (p *RabbitMQProducer) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.isClosed = true
	if p.ch != nil {
		p.ch.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
}
