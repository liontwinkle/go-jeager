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

package spanstore

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/uber-go/zap"
	"github.com/uber/jaeger-lib/metrics"

	"github.com/uber/jaeger/model"
	"github.com/uber/jaeger/pkg/cassandra"
	"github.com/uber/jaeger/pkg/cassandra/mocks"
	"github.com/uber/jaeger/pkg/testutils"
	"github.com/uber/jaeger/plugin/storage/cassandra/spanstore/dbmodel"
	"github.com/uber/jaeger/storage/spanstore"
)

type spanReaderTest struct {
	session   *mocks.Session
	logger    zap.Logger
	logBuffer *bytes.Buffer
	reader    *SpanReader
}

func withSpanReader(fn func(r *spanReaderTest)) {
	session := &mocks.Session{}
	logger, logBuffer := testutils.NewLogger(false)
	metricsFactory := metrics.NewLocalFactory(0)
	r := &spanReaderTest{
		session:   session,
		logger:    logger,
		logBuffer: logBuffer,
		reader:    NewSpanReader(session, metricsFactory, logger),
	}
	fn(r)
}

func TestNewSpanReader(t *testing.T) {
	withSpanReader(func(r *spanReaderTest) {
		var reader spanstore.Reader = r.reader // check API conformance
		assert.NotNil(t, reader)
	})
}

func TestSpanReaderGetServices(t *testing.T) {
	withSpanReader(func(r *spanReaderTest) {
		r.reader.serviceNamesReader = func() ([]string, error) { return []string{"service-a"}, nil }
		s, err := r.reader.GetServices()
		assert.NoError(t, err)
		assert.Equal(t, []string{"service-a"}, s)
	})
}

func TestSpanReaderGetOperations(t *testing.T) {
	withSpanReader(func(r *spanReaderTest) {
		r.reader.operationNamesReader = func(string) ([]string, error) { return []string{"operation-a"}, nil }
		s, err := r.reader.GetOperations("service-x")
		assert.NoError(t, err)
		assert.Equal(t, []string{"operation-a"}, s)
	})
}

func TestSpanReaderGetTrace(t *testing.T) {
	badScan := func() interface{} {
		return matchOnceWithSideEffect(func(args []interface{}) {
			for _, arg := range args {
				if v, ok := arg.(*[]dbmodel.KeyValue); ok {
					*v = []dbmodel.KeyValue{
						{
							ValueType: "bad",
						},
					}
				}
			}
		})
	}

	testCases := []struct {
		scanner     interface{}
		closeErr    error
		expectedErr string
	}{
		{scanner: matchOnce()},
		{scanner: badScan(), expectedErr: "not a valid ValueType string bad"},
		{
			scanner:     matchOnce(),
			closeErr:    errors.New("error on close()"),
			expectedErr: "Error reading traces from storage: error on close()",
		},
	}
	for _, tc := range testCases {
		testCase := tc // capture loop var
		t.Run("expected err="+testCase.expectedErr, func(t *testing.T) {
			withSpanReader(func(r *spanReaderTest) {
				iter := &mocks.Iterator{}
				iter.On("Scan", testCase.scanner).Return(true)
				iter.On("Scan", matchEverything()).Return(false)
				iter.On("Close").Return(testCase.closeErr)

				query := &mocks.Query{}
				query.On("Consistency", cassandra.One).Return(query)
				query.On("Iter").Return(iter)

				r.session.On("Query", mock.AnythingOfType("string"), matchEverything()).Return(query)

				trace, err := r.reader.GetTrace(model.TraceID{})
				if testCase.expectedErr == "" {
					assert.NoError(t, err)
					assert.NotNil(t, trace)
				} else {
					assert.EqualError(t, err, testCase.expectedErr)
					assert.Nil(t, trace)
				}
			})
		})
	}
}

func TestSpanReaderGetTrace_TraceNotFound(t *testing.T) {
	withSpanReader(func(r *spanReaderTest) {
		iter := &mocks.Iterator{}
		iter.On("Scan", matchEverything()).Return(false)
		iter.On("Close").Return(nil)

		query := &mocks.Query{}
		query.On("Consistency", cassandra.One).Return(query)
		query.On("Iter").Return(iter)

		r.session.On("Query", mock.AnythingOfType("string"), matchEverything()).Return(query)

		trace, err := r.reader.GetTrace(model.TraceID{})
		assert.Nil(t, trace)
		assert.EqualError(t, err, "trace not found")
	})
}

func TestSpanReaderFindTracesBadRequest(t *testing.T) {
	withSpanReader(func(r *spanReaderTest) {
		_, err := r.reader.FindTraces(nil)
		assert.Error(t, err)
	})
}

func TestSpanReaderFindTraces(t *testing.T) {
	testCases := []struct {
		caption        string
		numTraces      int
		queryTags      bool
		mainQueryError error
		tagsQueryError error
		loadQueryError error
		expectedCount  int
		expectedError  string
		expectedLogs   []string
	}{
		{
			caption:       "main query",
			expectedCount: 2,
		},
		{
			caption:       "tag query",
			expectedCount: 2,
			queryTags:     true,
		},
		{
			caption:       "with limit",
			numTraces:     1,
			expectedCount: 1,
		},
		{
			caption:        "main query error",
			mainQueryError: errors.New("main query error"),
			expectedError:  "main query error",
			expectedLogs: []string{
				"[E] Failed to exec query query=SELECT trace_id, span_id FROM traces",
			},
		},
		{
			caption:        "tags query error",
			queryTags:      true,
			tagsQueryError: errors.New("tags query error"),
			expectedError:  "tags query error",
			expectedLogs: []string{
				"[E] Failed to exec query query=SELECT trace_id, span_id FROM tag_index",
			},
		},
		{
			caption:        "load trace error",
			loadQueryError: errors.New("load query error"),
			expectedCount:  0,
			expectedLogs: []string{
				"[E] Failure to read trace trace_id=1 error=Error reading traces from storage: load query error",
				"[E] Failure to read trace trace_id=2 error=Error reading traces from storage: load query error",
			},
		},
	}
	for _, tc := range testCases {
		testCase := tc // capture loop var
		t.Run(testCase.caption, func(t *testing.T) {
			withSpanReader(func(r *spanReaderTest) {
				// scanMatcher can match Iter.Scan() parameters and set trace ID fields
				scanMatcher := func(name string) interface{} {
					traceIDs := []dbmodel.TraceID{
						dbmodel.TraceIDFromDomain(model.TraceID{Low: 1}),
						dbmodel.TraceIDFromDomain(model.TraceID{Low: 2}),
					}
					scanFunc := func(args []interface{}) bool {
						if len(traceIDs) == 0 {
							return false
						}
						for _, arg := range args {
							if ptr, ok := arg.(*dbmodel.TraceID); ok {
								*ptr = traceIDs[0]
								break
							}
						}
						traceIDs = traceIDs[1:]
						return true
					}
					return mock.MatchedBy(scanFunc)
				}

				mockQuery := func(queryErr error) *mocks.Query {
					iter := &mocks.Iterator{}
					iter.On("Scan", scanMatcher("queryIter")).Return(true)
					iter.On("Scan", matchEverything()).Return(false)
					iter.On("Close").Return(queryErr)

					query := &mocks.Query{}
					query.On("Bind", matchEverything()).Return(query)
					query.On("Consistency", cassandra.One).Return(query)
					query.On("Iter").Return(iter)

					return query
				}

				mainQuery := mockQuery(testCase.mainQueryError)
				tagsQuery := mockQuery(testCase.tagsQueryError)

				makeLoadQuery := func() *mocks.Query {
					loadQueryIter := &mocks.Iterator{}
					loadQueryIter.On("Scan", scanMatcher("loadIter")).Return(true)
					loadQueryIter.On("Scan", matchEverything()).Return(false)
					loadQueryIter.On("Close").Return(testCase.loadQueryError)

					loadQuery := &mocks.Query{}
					loadQuery.On("Consistency", cassandra.One).Return(loadQuery)
					loadQuery.On("Iter").Return(loadQueryIter)
					return loadQuery
				}

				r.session.On("Query", stringMatcher("SELECT trace_id, span_id FROM traces"), matchEverything()).Return(mainQuery)
				r.session.On("Query", stringMatcher("SELECT trace_id, span_id FROM tag_index"), matchEverything()).Return(tagsQuery)
				r.session.On("Query", stringMatcher("SELECT trace_id, span_id, "), matchOnce()).Return(makeLoadQuery())
				r.session.On("Query", stringMatcher("SELECT trace_id, span_id, "), matchEverything()).Return(makeLoadQuery())

				queryParams := &spanstore.TraceQueryParameters{ServiceName: "service-a", NumTraces: 100}
				if testCase.numTraces != 0 {
					queryParams.NumTraces = testCase.numTraces
				}
				if testCase.queryTags {
					queryParams.Tags = make(map[string]string)
					queryParams.Tags["x"] = "y"
				}
				res, err := r.reader.FindTraces(queryParams)
				if testCase.expectedError == "" {
					assert.NoError(t, err)
					assert.Len(t, res, testCase.expectedCount, "expecting certain number of traces")
				} else {
					assert.EqualError(t, err, testCase.expectedError)
				}
				for _, expectedLog := range testCase.expectedLogs {
					assert.True(t, strings.Contains(r.logBuffer.String(), expectedLog), "Log must contain %s, but was %s", expectedLog, r.logBuffer.String())
				}
				if len(testCase.expectedLogs) == 0 {
					assert.Equal(t, "", r.logBuffer.String())
				}
			})

		})
	}
}
