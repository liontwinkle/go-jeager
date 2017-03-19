// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package servers

import (
	"testing"
	"time"

	athrift "github.com/apache/thrift/lib/go/thrift"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber/jaeger-lib/metrics"
	mTestutils "github.com/uber/jaeger-lib/metrics/testutils"

	"github.com/uber/jaeger/thrift-gen/agent"
	"github.com/uber/jaeger/thrift-gen/zipkincore"

	"github.com/uber/jaeger/cmd/agent/app/customtransports"
	"github.com/uber/jaeger/cmd/agent/app/servers/thriftudp"
	"github.com/uber/jaeger/cmd/agent/app/testutils"
)

func TestTBufferedServer(t *testing.T) {
	t.Run("processed", func(t *testing.T) {
		testTBufferedServer(t, 10, false)
	})
	t.Run("dropped", func(t *testing.T) {
		testTBufferedServer(t, 1, true)
	})
}

func testTBufferedServer(t *testing.T, queueSize int, testDroppedPackets bool) {
	metricsFactory := metrics.NewLocalFactory(0)

	transport, err := thriftudp.NewTUDPServerTransport("127.0.0.1:0")
	require.NoError(t, err)

	maxPacketSize := 65000
	server, err := NewTBufferedServer(transport, queueSize, maxPacketSize, metricsFactory)
	require.NoError(t, err)
	go server.Serve()
	defer server.Stop()
	time.Sleep(10 * time.Millisecond) // wait for server to start serving

	hostPort := transport.Addr().String()
	client, clientCloser, err := testutils.NewZipkinThriftUDPClient(hostPort)
	require.NoError(t, err)
	defer clientCloser.Close()

	span := zipkincore.NewSpan()
	span.Name = "span1"

	err = client.EmitZipkinBatch([]*zipkincore.Span{span})
	require.NoError(t, err)

	if testDroppedPackets {
		// because queueSize == 1 for this test, and we're not reading from data chan,
		// the second packet we send will be dropped by the server
		err = client.EmitZipkinBatch([]*zipkincore.Span{span})
		require.NoError(t, err)

		for i := 0; i < 50; i++ {
			c, _ := metricsFactory.Snapshot()
			if c["thrift.udp.server.packets.dropped"] == 1 {
				return
			}
			time.Sleep(time.Millisecond)
		}
		c, _ := metricsFactory.Snapshot()
		assert.FailNow(t, "Dropped packets counter not incremented", "Counters: %+v", c)
	}

	inMemReporter := testutils.NewInMemoryReporter()
	select {
	case readBuf := <-server.DataChan():
		assert.NotEqual(t, 0, len(readBuf.GetBytes()))
		protoFact := athrift.NewTCompactProtocolFactory()
		trans := &customtransport.TBufferedReadTransport{}
		protocol := protoFact.GetProtocol(trans)
		protocol.Transport().Write(readBuf.GetBytes())
		server.DataRecd(readBuf)
		handler := agent.NewAgentProcessor(inMemReporter)
		handler.Process(protocol, protocol)
	case <-time.After(time.Second * 1):
		t.Fatalf("Server should have received span submission")
	}

	require.Equal(t, 1, len(inMemReporter.ZipkinSpans()))
	assert.Equal(t, "span1", inMemReporter.ZipkinSpans()[0].Name)

	// server must emit metrics
	mTestutils.AssertCounterMetrics(t, metricsFactory,
		mTestutils.ExpectedMetric{Name: "thrift.udp.server.packets.processed", Value: 1},
		mTestutils.ExpectedMetric{Name: "thrift.udp.server.packets.dropped", Value: 0},
	)
	mTestutils.AssertGaugeMetrics(t, metricsFactory,
		mTestutils.ExpectedMetric{Name: "thrift.udp.server.packet_size", Value: 38},
		mTestutils.ExpectedMetric{Name: "thrift.udp.server.queue_size", Value: 0},
	)
}
