// Copyright (c) 2017 Uber Technologies, Inc.
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

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/opentracing/opentracing-go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	jaegerClientConfig "github.com/uber/jaeger-client-go/config"
	jaegerClientZapLog "github.com/uber/jaeger-client-go/log/zap"
	"github.com/uber/jaeger-lib/metrics"
	"go.uber.org/zap"

	"github.com/jaegertracing/jaeger/cmd/env"
	"github.com/jaegertracing/jaeger/cmd/flags"
	"github.com/jaegertracing/jaeger/cmd/query/app"
	"github.com/jaegertracing/jaeger/cmd/query/app/querysvc"
	"github.com/jaegertracing/jaeger/pkg/config"
	"github.com/jaegertracing/jaeger/pkg/version"
	"github.com/jaegertracing/jaeger/plugin/storage"
	"github.com/jaegertracing/jaeger/ports"
	istorage "github.com/jaegertracing/jaeger/storage"
	storageMetrics "github.com/jaegertracing/jaeger/storage/spanstore/metrics"
)

func main() {
	svc := flags.NewService(ports.QueryAdminHTTP)

	storageFactory, err := storage.NewFactory(storage.FactoryConfigFromEnvAndCLI(os.Args, os.Stderr))
	if err != nil {
		log.Fatalf("Cannot initialize storage factory: %v", err)
	}

	v := viper.New()
	var command = &cobra.Command{
		Use:   "jaeger-query",
		Short: "Jaeger query service provides a Web UI and an API for accessing trace data.",
		Long:  `Jaeger query service provides a Web UI and an API for accessing trace data.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := svc.Start(v); err != nil {
				return err
			}
			logger := svc.Logger // shortcut
			baseFactory := svc.MetricsFactory.Namespace(metrics.NSOptions{Name: "jaeger"})
			metricsFactory := baseFactory.Namespace(metrics.NSOptions{Name: "query"})

			tracer, closer, err := jaegerClientConfig.Configuration{
				Sampler: &jaegerClientConfig.SamplerConfig{
					Type:  "probabilistic",
					Param: 1.0,
				},
				RPCMetrics: true,
			}.New(
				"jaeger-query",
				jaegerClientConfig.Metrics(svc.MetricsFactory),
				jaegerClientConfig.Logger(jaegerClientZapLog.NewLogger(logger)),
			)
			if err != nil {
				logger.Fatal("Failed to initialize tracer", zap.Error(err))
			}
			defer closer.Close()
			opentracing.SetGlobalTracer(tracer)

			storageFactory.InitFromViper(v)
			if err := storageFactory.Initialize(baseFactory, logger); err != nil {
				logger.Fatal("Failed to init storage factory", zap.Error(err))
			}
			spanReader, err := storageFactory.CreateSpanReader()
			if err != nil {
				logger.Fatal("Failed to create span reader", zap.Error(err))
			}
			spanReader = storageMetrics.NewReadMetricsDecorator(spanReader, metricsFactory)
			dependencyReader, err := storageFactory.CreateDependencyReader()
			if err != nil {
				logger.Fatal("Failed to create dependency reader", zap.Error(err))
			}
			queryServiceOptions := archiveOptions(storageFactory, logger)
			queryService := querysvc.NewQueryService(
				spanReader,
				dependencyReader,
				queryServiceOptions)

			queryOpts := new(app.QueryOptions).InitFromViper(v)
			server := app.NewServer(svc, queryService, queryOpts, tracer)

			if err := server.Start(); err != nil {
				logger.Fatal("Could not start servers", zap.Error(err))
			}

			svc.RunAndThen(func() {
				server.Close()
			})
			return nil
		},
	}

	command.AddCommand(version.Command())
	command.AddCommand(env.Command())

	config.AddFlags(
		v,
		command,
		svc.AddFlags,
		storageFactory.AddFlags,
		app.AddFlags,
	)

	if error := command.Execute(); error != nil {
		fmt.Println(error.Error())
		os.Exit(1)
	}
}

func archiveOptions(storageFactory istorage.Factory, logger *zap.Logger) querysvc.QueryServiceOptions {
	archiveFactory, ok := storageFactory.(istorage.ArchiveFactory)
	if !ok {
		logger.Info("Archive storage not supported by the factory")
		return querysvc.QueryServiceOptions{}
	}
	reader, err := archiveFactory.CreateArchiveSpanReader()
	if err == istorage.ErrArchiveStorageNotConfigured || err == istorage.ErrArchiveStorageNotSupported {
		logger.Info("Archive storage not created", zap.String("reason", err.Error()))
		return querysvc.QueryServiceOptions{}
	}
	if err != nil {
		logger.Error("Cannot init archive storage reader", zap.Error(err))
		return querysvc.QueryServiceOptions{}
	}
	writer, err := archiveFactory.CreateArchiveSpanWriter()
	if err == istorage.ErrArchiveStorageNotConfigured || err == istorage.ErrArchiveStorageNotSupported {
		logger.Info("Archive storage not created", zap.String("reason", err.Error()))
		return querysvc.QueryServiceOptions{}
	}
	if err != nil {
		logger.Error("Cannot init archive storage writer", zap.Error(err))
		return querysvc.QueryServiceOptions{}
	}
	return querysvc.QueryServiceOptions{
		ArchiveSpanReader: reader,
		ArchiveSpanWriter: writer,
	}
}
