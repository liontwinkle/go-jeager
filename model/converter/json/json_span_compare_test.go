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

package json

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/jaeger/model/json"
)

func CompareJSONSpans(t *testing.T, expected *json.Span, actual *json.Span) {
	// TODO: Don't manually enumerate, because if we add a new field, this test would still pass.
	assert.Equal(t, expected.TraceID, actual.TraceID)
	assert.Equal(t, expected.SpanID, actual.SpanID)
	assert.Equal(t, expected.OperationName, actual.OperationName)
	assert.Equal(t, expected.References, actual.References)
	assert.Equal(t, expected.Flags, actual.Flags)
	assert.Equal(t, expected.StartTime, actual.StartTime)
	assert.Equal(t, expected.Duration, actual.Duration)
	compareJSONTags(t, expected.Tags, actual.Tags)
	compareJSONLogs(t, expected.Logs, actual.Logs)
	compareJSONProcess(t, expected.Process, actual.Process)
}

type JSONTagByKey []json.KeyValue

func (t JSONTagByKey) Len() int           { return len(t) }
func (t JSONTagByKey) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
func (t JSONTagByKey) Less(i, j int) bool { return t[i].Key < t[j].Key }

func compareJSONTags(t *testing.T, expected []json.KeyValue, actual []json.KeyValue) {
	sort.Sort(JSONTagByKey(expected))
	sort.Sort(JSONTagByKey(actual))
	assert.Equal(t, expected, actual)
	assert.Equal(t, len(expected), len(actual))
	for i := range expected {
		assert.Equal(t, expected[i].Key, actual[i].Key)
		assert.Equal(t, expected[i].Type, actual[i].Type)
		assert.Equal(t, expected[i].Value, actual[i].Value)
	}
}

type JSONLogByTimestamp []json.Log

func (t JSONLogByTimestamp) Len() int           { return len(t) }
func (t JSONLogByTimestamp) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
func (t JSONLogByTimestamp) Less(i, j int) bool { return t[i].Timestamp < t[j].Timestamp }

// this function exists solely to make it easier for developer to find out where the difference is
func compareJSONLogs(t *testing.T, expected []json.Log, actual []json.Log) {
	sort.Sort(JSONLogByTimestamp(expected))
	sort.Sort(JSONLogByTimestamp(actual))
	assert.Equal(t, len(expected), len(actual))
	for i := range expected {
		assert.Equal(t, expected[i].Timestamp, actual[i].Timestamp)
		compareJSONTags(t, expected[i].Fields, actual[i].Fields)
	}
}

func compareJSONProcess(t *testing.T, expected *json.Process, actual *json.Process) {
	assert.Equal(t, expected.ServiceName, actual.ServiceName)
	compareJSONTags(t, expected.Tags, actual.Tags)
}
