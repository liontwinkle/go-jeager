// Copyright (c) 2018 The Jaeger Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package consumer

import (
	"io"

	"github.com/uber/jaeger-lib/metrics"
	"go.uber.org/zap"

	"github.com/jaegertracing/jaeger/cmd/ingester/app/consumer/offset"
	"github.com/jaegertracing/jaeger/cmd/ingester/app/processor"
	"github.com/jaegertracing/jaeger/cmd/ingester/app/processor/decorator"
)

type processorFactory struct {
	topic          string
	consumer       SaramaConsumer
	metricsFactory metrics.Factory
	logger         *zap.Logger
	baseProcessor  processor.SpanProcessor
	parallelism    int
}

func (c *processorFactory) new(partition int32, minOffset int64) processor.SpanProcessor {
	c.logger.Info("Creating new processors", zap.Int32("partition", partition))

	markOffset := func(offset int64) {
		c.consumer.MarkPartitionOffset(c.topic, partition, offset, "")
	}

	om := offset.NewManager(minOffset, markOffset, partition, c.metricsFactory)

	retryProcessor := decorator.NewRetryingProcessor(c.metricsFactory, c.baseProcessor)
	cp := NewCommittingProcessor(retryProcessor, om)
	spanProcessor := processor.NewDecoratedProcessor(c.metricsFactory, cp)
	pp := processor.NewParallelProcessor(spanProcessor, c.parallelism, c.logger)

	return newStartedProcessor(pp, om)
}

type service interface {
	Start()
	io.Closer
}

type startProcessor interface {
	Start()
	processor.SpanProcessor
}

type startedProcessor struct {
	services  []service
	processor startProcessor
}

func newStartedProcessor(parallelProcessor startProcessor, services ...service) processor.SpanProcessor {
	s := &startedProcessor{
		services:  services,
		processor: parallelProcessor,
	}

	for _, service := range services {
		service.Start()
	}

	s.processor.Start()
	return s
}

func (c *startedProcessor) Process(message processor.Message) error {
	return c.processor.Process(message)
}

func (c *startedProcessor) Close() error {
	c.processor.Close()

	for _, service := range c.services {
		service.Close()
	}
	return nil
}
