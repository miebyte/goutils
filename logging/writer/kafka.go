// File:		kafka.go
// Created by:	Hoven
// Created on:	2025-04-04
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package writer

import (
	"context"
	"io"
	"os"

	"github.com/segmentio/kafka-go"
)

type KafkaLogWriter struct {
	ctx         context.Context
	osWriter    io.Writer
	kafkaWriter *kafka.Writer
	messageChan chan []byte
	done        chan struct{}
	toConsole   bool
}

type OptionFunc func(*KafkaLogWriter)

func WithEnableToConsole() OptionFunc {
	return func(kw *KafkaLogWriter) {
		kw.toConsole = true
	}
}

func WithContext(ctx context.Context) OptionFunc {
	return func(kw *KafkaLogWriter) {
		kw.ctx = ctx
	}
}

func NewKafkaLogWriter(kafkaWriter *kafka.Writer, opts ...OptionFunc) *KafkaLogWriter {
	h := &KafkaLogWriter{
		ctx:         context.TODO(),
		osWriter:    os.Stdout,
		kafkaWriter: kafkaWriter,
		messageChan: make(chan []byte, 1000),
		done:        make(chan struct{}),
	}

	for _, opt := range opts {
		opt(h)
	}

	go h.processMessages()

	return h
}

func (w *KafkaLogWriter) processMessages() {
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.done:
			return
		case msg, ok := <-w.messageChan:
			if !ok {
				return
			}
			_ = w.kafkaWriter.WriteMessages(w.ctx, kafka.Message{Value: msg})

			if w.toConsole && w.osWriter != nil {
				w.osWriter.Write(msg)
			}
		}
	}
}

func (w *KafkaLogWriter) Write(p []byte) (n int, err error) {
	buf := make([]byte, len(p))
	copy(buf, p)
	w.messageChan <- buf
	return len(p), nil
}

func (w *KafkaLogWriter) Close() error {
	close(w.done)
	close(w.messageChan)
	return w.kafkaWriter.Close()
}
