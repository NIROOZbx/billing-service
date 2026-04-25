package producer

import (
	"context"
	"time"

	"github.com/NIROOZbx/billing-service/config"
	"github.com/bytedance/sonic"
	"github.com/segmentio/kafka-go"
)

type producer struct {
	writer *kafka.Writer
}

type Producer interface{
	 Publish(ctx context.Context, topic string, event any) error
	 Close() error
}

func NewKafkaProducer(cfg config.KafkaConfig) *producer {
	return &producer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(cfg.BrokerAddress),
			BatchSize:    cfg.BatchSize,
			BatchTimeout: time.Duration(cfg.BatchTimeoutMS) * time.Millisecond,
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireAll,
		},
	}
}

func (p *producer) Publish(ctx context.Context, topic string, event any) error {

	bytes,err:=sonic.Marshal(event)

	if err!=nil{
		return err
	}
	return p.writer.WriteMessages(ctx, kafka.Message{Topic: topic,Value: bytes})
}

func (p *producer) Close() error {
	return p.writer.Close()
}