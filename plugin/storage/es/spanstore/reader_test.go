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
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/olivere/elastic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/uber/jaeger/model"
	"github.com/uber/jaeger/pkg/es/mocks"
	"github.com/uber/jaeger/pkg/testutils"
	"github.com/uber/jaeger/storage/spanstore"
)

var exampleESSpan = []byte(
	`{
	   "traceID": "1",
	   "parentSpanID": "2",
	   "spanID": "3",
	   "flags": 0,
	   "operationName": "op",
	   "references": [],
	   "startTime": 812965625,
	   "duration": 3290114992,
	   "tags": [
	      {
		 "key": "tag",
		 "value": "1965806585",
		 "type": "int64"
	      }
	   ],
	   "logs": [
	      {
		 "timestamp": 812966073,
		 "fields": [
		    {
		       "key": "logtag",
		       "value": "helloworld",
		       "type": "string"
		    }
		 ]
	      }
	   ],
	   "process": {
	      "serviceName": "serv",
	      "tags": [
		 {
		    "key": "processtag",
		    "value": "false",
		    "type": "bool"
		 }
	      ]
	   }
	}`)

type spanReaderTest struct {
	client    *mocks.Client
	logger    *zap.Logger
	logBuffer *testutils.Buffer
	reader    *SpanReader
}

func withSpanReader(fn func(r *spanReaderTest)) {
	client := &mocks.Client{}
	logger, logBuffer := testutils.NewLogger()
	r := &spanReaderTest{
		client:    client,
		logger:    logger,
		logBuffer: logBuffer,
		reader:    NewSpanReader(client, logger),
	}
	fn(r)
}

func TestNewSpanReader(t *testing.T) {
	withSpanReader(func(r *spanReaderTest) {
		var reader spanstore.Reader = r.reader // check API conformance
		assert.NotNil(t, reader)
	})
}

func TestSpanReader_GetTrace(t *testing.T) {
	withSpanReader(func(r *spanReaderTest) {
		mockExistsService(r)

		hits := make([]*elastic.SearchHit, 1)
		hits[0] = &elastic.SearchHit{
			Source: (*json.RawMessage)(&exampleESSpan),
		}
		searchHits := &elastic.SearchHits{Hits: hits}

		mockSearchService("", r).On("Do", mock.AnythingOfType("*context.emptyCtx")).
			Return(&elastic.SearchResult{
				Hits: searchHits,
			}, nil)

		trace, err := r.reader.GetTrace(model.TraceID{Low: 1})
		require.NoError(t, err)
		require.NotNil(t, trace)

		// TODO: This is not a deep equal; does not check every element.
		require.Len(t, trace.Spans, 1)
		testSpan := trace.Spans[0]
		assert.Equal(t, uint64(1), testSpan.TraceID.Low)
		assert.Equal(t, model.SpanID(2), testSpan.ParentSpanID)
		assert.Equal(t, model.SpanID(3), testSpan.SpanID)
		assert.Equal(t, model.Flags(0), testSpan.Flags)
		assert.Equal(t, "op", testSpan.OperationName)
		assert.Equal(t, "serv", testSpan.Process.ServiceName)
		require.Len(t, testSpan.Process.Tags, 1)
		assert.Equal(t, "processtag", testSpan.Process.Tags[0].Key)
		assert.Equal(t, false, testSpan.Process.Tags[0].Value())
		require.Len(t, testSpan.Tags, 1)
		assert.Equal(t, "tag", testSpan.Tags[0].Key)
		assert.Equal(t, int64(1965806585), testSpan.Tags[0].Value())
		require.Len(t, testSpan.Logs, 1)
		require.Len(t, testSpan.Logs[0].Fields, 1)
		assert.Equal(t, "logtag", testSpan.Logs[0].Fields[0].Key)
		assert.Equal(t, "helloworld", testSpan.Logs[0].Fields[0].Value())
	})
}

func TestSpanReader_GetTraceQueryError(t *testing.T) {
	withSpanReader(func(r *spanReaderTest) {
		mockExistsService(r)
		mockSearchService("", r).On("Do", mock.AnythingOfType("*context.emptyCtx")).
			Return(nil, errors.New("query error occurred"))
		trace, err := r.reader.GetTrace(model.TraceID{Low: 1})
		require.EqualError(t, err, "Query execution failed: query error occurred")
		require.Nil(t, trace)
	})
}

func TestSpanReader_GetTraceNoSpansError(t *testing.T) {
	withSpanReader(func(r *spanReaderTest) {
		mockExistsService(r)

		hits := make([]*elastic.SearchHit, 0)
		searchHits := &elastic.SearchHits{Hits: hits}

		mockSearchService("", r).On("Do", mock.AnythingOfType("*context.emptyCtx")).
			Return(&elastic.SearchResult{
				Hits: searchHits,
			}, nil)

		trace, err := r.reader.GetTrace(model.TraceID{Low: 1})
		require.EqualError(t, err, "trace not found")
		require.Nil(t, trace)
	})
}

func TestSpanReader_GetTraceInvalidSpanError(t *testing.T) {
	withSpanReader(func(r *spanReaderTest) {
		mockExistsService(r)

		data := []byte(`{"TraceID": "123"asdf fadsg}`)
		hits := make([]*elastic.SearchHit, 1)
		hits[0] = &elastic.SearchHit{
			Source: (*json.RawMessage)(&data),
		}
		searchHits := &elastic.SearchHits{Hits: hits}

		mockSearchService("", r).On("Do", mock.AnythingOfType("*context.emptyCtx")).
			Return(&elastic.SearchResult{
				Hits: searchHits,
			}, nil)

		trace, err := r.reader.GetTrace(model.TraceID{Low: 1})
		require.Error(t, err, "invalid span")
		require.Nil(t, trace)
	})
}

func TestSpanReader_GetTraceSpanConversionError(t *testing.T) {
	withSpanReader(func(r *spanReaderTest) {
		mockExistsService(r)

		badSpan := []byte(`{"TraceID": "123"}`)

		hits := make([]*elastic.SearchHit, 1)
		hits[0] = &elastic.SearchHit{
			Source: (*json.RawMessage)(&badSpan),
		}
		searchHits := &elastic.SearchHits{Hits: hits}

		mockSearchService("", r).On("Do", mock.AnythingOfType("*context.emptyCtx")).
			Return(&elastic.SearchResult{
				Hits: searchHits,
			}, nil)

		trace, err := r.reader.GetTrace(model.TraceID{Low: 1})
		require.Error(t, err, "span conversion error, because lacks elements")
		require.Nil(t, trace)
	})
}

func TestSpanReader_executeQuery(t *testing.T) {
	withSpanReader(func(r *spanReaderTest) {
		hits := make([]*elastic.SearchHit, 7)
		searchHits := &elastic.SearchHits{Hits: hits}

		mockSearchService("", r).On("Do", mock.AnythingOfType("*context.emptyCtx")).
			Return(&elastic.SearchResult{
				Hits: searchHits,
			}, nil)

		query := elastic.NewTermQuery("traceID", "helloo")
		hits, err := r.reader.executeQuery(query, "hello", "world", "index")

		require.NoError(t, err)
		assert.Len(t, hits, 7)
	})
}

func TestSpanReader_executeQueryError(t *testing.T) {
	withSpanReader(func(r *spanReaderTest) {
		mockSearchService("", r).On("Do", mock.AnythingOfType("*context.emptyCtx")).
			Return(nil, errors.New("query error"))

		query := elastic.NewTermQuery("traceID", "helloo")
		hits, err := r.reader.executeQuery(query, "hello", "world", "index")

		require.Error(t, err, "query error")
		assert.Nil(t, hits)
	})
}

func TestSpanReader_esJSONtoJSONSpanModel(t *testing.T) {
	withSpanReader(func(r *spanReaderTest) {
		jsonPayload := (*json.RawMessage)(&exampleESSpan)

		esSpanRaw := &elastic.SearchHit{
			Source: jsonPayload,
		}

		span, err := r.reader.unmarshallJSONSpan(esSpanRaw)
		require.NoError(t, err)

		// TODO: This is not a deep equal; does not check every element.
		assert.Equal(t, "1", string(span.TraceID))
		assert.Equal(t, "2", string(span.ParentSpanID))
		assert.Equal(t, "3", string(span.SpanID))
		assert.Equal(t, uint32(0), span.Flags)
		assert.Equal(t, "op", span.OperationName)
		assert.Equal(t, "serv", span.Process.ServiceName)
		assert.Equal(t, uint64(812965625), span.StartTime)
		assert.Equal(t, uint64(3290114992), span.Duration)
		require.Len(t, span.Tags, 1)
		assert.Equal(t, "tag", span.Tags[0].Key)
		assert.Equal(t, "int64", string(span.Tags[0].Type))
	})
}

func TestSpanReader_esJSONtoJSONSpanModelError(t *testing.T) {
	withSpanReader(func(r *spanReaderTest) {
		data := []byte(`{"TraceID": "123"asdf fadsg}`)
		jsonPayload := (*json.RawMessage)(&data)

		esSpanRaw := &elastic.SearchHit{
			Source: jsonPayload,
		}

		span, err := r.reader.unmarshallJSONSpan(esSpanRaw)
		require.Error(t, err)
		assert.Nil(t, span)
	})
}

func TestSpanReader_findIndicesEmptyQuery(t *testing.T) {
	withSpanReader(func(r *spanReaderTest) {
		mockExistsService(r)

		actual := r.reader.findIndices(spanstore.TraceQueryParameters{})

		today := time.Now()
		yesterday := today.AddDate(0, 0, -1)
		twoDaysAgo := today.AddDate(0, 0, -2)

		expected := []string{
			r.reader.indexWithDate(today),
			r.reader.indexWithDate(yesterday),
			r.reader.indexWithDate(twoDaysAgo),
		}

		assert.EqualValues(t, expected, actual)
	})
}

// TODO: Dry the below two separate findIndices test
func TestSpanReader_findIndicesNoIndices(t *testing.T) {
	withSpanReader(func(r *spanReaderTest) {
		mockExistsService(r)

		actual := r.reader.findIndices(spanstore.TraceQueryParameters{
			StartTimeMin: time.Date(1995, time.April, 21, 4, 21, 19, 95, time.UTC),
			StartTimeMax: time.Date(2017, time.April, 21, 4, 21, 19, 95, time.UTC),
		})

		var expected []string

		assert.EqualValues(t, expected, actual)
	})
}

func TestSpanReader_findIndicesOnlyRecent(t *testing.T) {
	withSpanReader(func(r *spanReaderTest) {
		mockExistsService(r)

		today := time.Now()

		actual := r.reader.findIndices(spanstore.TraceQueryParameters{
			StartTimeMin: today.AddDate(0, 0, -7),
			StartTimeMax: today.AddDate(0, 0, -1),
		})

		yesterday := today.AddDate(0, 0, -1)
		twoDaysAgo := today.AddDate(0, 0, -2)

		expected := []string{
			r.reader.indexWithDate(yesterday),
			r.reader.indexWithDate(twoDaysAgo),
		}

		assert.EqualValues(t, expected, actual)
	})
}

func TestSpanReader_indexWithDate(t *testing.T) {
	withSpanReader(func(r *spanReaderTest) {
		actual := r.reader.indexWithDate(time.Date(1995, time.April, 21, 4, 21, 19, 95, time.UTC))
		assert.Equal(t, "jaeger-1995-04-21", actual)
	})
}

func TestSpanReader_GetServices(t *testing.T) {
	testGet("services", t)
}

func TestSpanReader_getServicesAggregation(t *testing.T) {
	// this aggregation marshalled should look like this:
	//
	// "terms":{
	//    "size":3000,
	//    "field":"serviceName"
	// }
	withSpanReader(func(r *spanReaderTest) {
		serviceAggregation := r.reader.getServicesAggregation()
		actual, err := serviceAggregation.Source()
		require.NoError(t, err)
		expected := make(map[string]interface{})
		terms := make(map[string]interface{})
		expected["terms"] = terms
		terms["size"] = defaultDocCount
		terms["field"] = "serviceName"

		assert.EqualValues(t, expected, actual)
	})
}

func TestSpanReader_GetOperations(t *testing.T) {
	testGet("operations", t)
}

func testGet(typ string, t *testing.T) {
	var aggregationType string
	if typ == "services" {
		aggregationType = servicesAggregation
	} else if typ == "operations" {
		aggregationType = operationsAggregation
	}
	goodAggregations := make(map[string]*json.RawMessage)
	rawMessage := []byte(`{"buckets": [{"key": "hello","doc_count": 16}]}`)
	goodAggregations[aggregationType] = (*json.RawMessage)(&rawMessage)

	badAggregations := make(map[string]*json.RawMessage)
	badRawMessage := []byte(`{"buckets": [{bad json]}asdf`)
	badAggregations[aggregationType] = (*json.RawMessage)(&badRawMessage)

	testCases := []struct {
		caption        string
		searchResult   *elastic.SearchResult
		searchError    error
		expectedError  string
		expectedOutput []string
	}{
		{
			caption:        typ + " full behavior",
			searchResult:   &elastic.SearchResult{Aggregations: elastic.Aggregations(goodAggregations)},
			expectedOutput: []string{"hello"},
		},
		{
			caption:       typ + " search error",
			searchError:   errors.New("Search failure"),
			expectedError: "Search service failed: Search failure",
		},
		{
			caption:       typ + " search error",
			searchResult:  &elastic.SearchResult{Aggregations: elastic.Aggregations(badAggregations)},
			expectedError: "Could not find aggregation of " + typ,
		},
	}
	for _, tc := range testCases {
		testCase := tc
		t.Run(testCase.caption, func(t *testing.T) {
			withSpanReader(func(r *spanReaderTest) {
				mockExistsService(r)

				mockSearchService(typ, r).
					On("Do", mock.AnythingOfType("*context.emptyCtx")).
					Return(testCase.searchResult, testCase.searchError)

				actual, err := returnSearchFunc(typ, r)
				if testCase.expectedError != "" {
					require.EqualError(t, err, testCase.expectedError)
					assert.Nil(t, actual)
				} else {
					require.NoError(t, err)
					assert.EqualValues(t, testCase.expectedOutput, actual)
				}
			})
		})
	}
}

func returnSearchFunc(typ string, r *spanReaderTest) ([]string, error) {
	if typ == "services" {
		return r.reader.GetServices()
	} else if typ == "operations" {
		return r.reader.GetOperations("someService")
	} else {
		return nil, errors.New("Specify services or operations only")
	}
}
func TestSpanReader_bucketToStringArray(t *testing.T) {
	withSpanReader(func(r *spanReaderTest) {
		buckets := make([]*elastic.AggregationBucketKeyItem, 3)
		buckets[0] = &elastic.AggregationBucketKeyItem{Key: "hello"}
		buckets[1] = &elastic.AggregationBucketKeyItem{Key: "world"}
		buckets[2] = &elastic.AggregationBucketKeyItem{Key: "2"}

		actual, err := r.reader.bucketToStringArray(buckets)
		require.NoError(t, err)

		assert.EqualValues(t, []string{"hello", "world", "2"}, actual)
	})
}

func TestSpanReader_bucketToStringArrayError(t *testing.T) {
	withSpanReader(func(r *spanReaderTest) {
		buckets := make([]*elastic.AggregationBucketKeyItem, 3)
		buckets[0] = &elastic.AggregationBucketKeyItem{Key: "hello"}
		buckets[1] = &elastic.AggregationBucketKeyItem{Key: "world"}
		buckets[2] = &elastic.AggregationBucketKeyItem{Key: 2}

		_, err := r.reader.bucketToStringArray(buckets)
		assert.EqualError(t, err, "Non-string key found in aggregation")
	})
}

func TestSpanReader_FindTraces(t *testing.T) {
	// TODO: write test once done with function
	// currently not doing anything, only for code coverage, ignore for code review
	withSpanReader(func(r *spanReaderTest) {
		s, e := r.reader.FindTraces(nil)
		assert.Nil(t, s)
		assert.Nil(t, e)
	})
}

func mockExistsService(r *spanReaderTest) {
	existsService := &mocks.IndicesExistsService{}
	existsService.On("Do", mock.AnythingOfType("*context.emptyCtx")).Return(true, nil)
	r.client.On("IndexExists", mock.AnythingOfType("string")).Return(existsService)
}

func mockSearchService(servicesOrOperations string, r *spanReaderTest) *mocks.SearchService {
	searchService := &mocks.SearchService{}
	searchService.On("Type", stringMatcher(serviceType)).Return(searchService)
	searchService.On("Type", stringMatcher(spanType)).Return(searchService)
	searchService.On("Query", mock.Anything).Return(searchService)
	searchService.On("Size", mock.MatchedBy(func(i int) bool {
		return i == 0
	})).Return(searchService)
	if servicesOrOperations == "services" {
		searchService.On("Aggregation",
			stringMatcher(servicesAggregation),
			mock.AnythingOfType("*elastic.TermsAggregation")).Return(searchService)
	} else if servicesOrOperations == "operations" {
		searchService.On("Aggregation",
			stringMatcher(operationsAggregation),
			mock.AnythingOfType("*elastic.FilterAggregation")).Return(searchService)
	}
	r.client.On("Search", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(searchService)
	return searchService
}
