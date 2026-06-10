// Package mq 封装 RabbitMQ，把 PDF 解析等耗时任务从请求链路解耦。
// controller 投递任务到队列即返回，worker 消费后执行并回写结果。
package mq

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"

	"GopherPaper/internal/config"
)

// Client 持有连接与 channel，并声明业务队列。
type Client struct {
	conn *amqp.Connection
	ch   *amqp.Channel
	cfg  config.MQConfig
}

// New 建立连接并声明解析队列，幂等。
func New(cfg config.MQConfig) (*Client, error) {
	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("mq: 连接 RabbitMQ 失败: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("mq: 打开 channel 失败: %w", err)
	}
	// durable 队列，重启不丢。
	for _, q := range []string{cfg.ParseQueue, cfg.ReportQueue} {
		if q == "" {
			continue
		}
		if _, err := ch.QueueDeclare(q, true, false, false, false, nil); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return nil, fmt.Errorf("mq: 声明队列 %s 失败: %w", q, err)
		}
	}
	return &Client{conn: conn, ch: ch, cfg: cfg}, nil
}

// Publish 向队列投递一条持久化消息。
func (c *Client) Publish(ctx context.Context, queue string, body []byte) error {
	return c.ch.PublishWithContext(ctx, "", queue, false, false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		})
}

// Consume 返回队列的投递通道供 worker 消费，需手动 ack。
func (c *Client) Consume(queue string) (<-chan amqp.Delivery, error) {
	return c.ch.Consume(queue, "", false, false, false, false, nil)
}

func (c *Client) Close() error {
	if c.ch != nil {
		_ = c.ch.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
