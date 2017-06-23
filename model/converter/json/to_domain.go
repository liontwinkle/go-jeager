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
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"github.com/uber/jaeger/model"
	"github.com/uber/jaeger/model/json"
)

// SpanToDomain converts json.Span with embedded Process into model.Span format.
func SpanToDomain(span *json.Span) (*model.Span, error) {
	return toDomain{}.spanToDomain(span)
}

type toDomain struct{}

func (td toDomain) spanToDomain(dbSpan *json.Span) (*model.Span, error) {
	tags, err := td.convertKeyValues(dbSpan.Tags)
	if err != nil {
		return nil, err
	}
	logs, err := td.convertLogs(dbSpan.Logs)
	if err != nil {
		return nil, err
	}
	refs, err := td.convertRefs(dbSpan.References)
	if err != nil {
		return nil, err
	}
	process, err := td.convertProcess(dbSpan.Process)
	if err != nil {
		return nil, err
	}
	traceID, err := model.TraceIDFromString(string(dbSpan.TraceID))
	if err != nil {
		return nil, err
	}

	spanIDInt, err := model.SpanIDFromString(string(dbSpan.SpanID))
	if err != nil {
		return nil, err
	}
	parentSpanIDInt, err := model.SpanIDFromString(string(dbSpan.ParentSpanID))
	if err != nil {
		return nil, err
	}

	span := &model.Span{
		TraceID:       traceID,
		SpanID:        model.SpanID(spanIDInt),
		ParentSpanID:  model.SpanID(parentSpanIDInt),
		OperationName: dbSpan.OperationName,
		References:    refs,
		Flags:         model.Flags(uint32(dbSpan.Flags)),
		StartTime:     model.EpochMicrosecondsAsTime(dbSpan.StartTime),
		Duration:      model.MicrosecondsAsDuration(dbSpan.Duration),
		Tags:          tags,
		Logs:          logs,
		Process:       process,
	}
	return span, nil
}

func (td toDomain) convertRefs(refs []json.Reference) ([]model.SpanRef, error) {
	retMe := make([]model.SpanRef, len(refs))
	for i, r := range refs {
		// There are some inconsistencies with ReferenceTypes, hence the hacky fix.
		var refType model.SpanRefType
		if r.RefType == json.ChildOf {
			refType = model.ChildOf
		} else if r.RefType == json.FollowsFrom {
			refType = model.FollowsFrom
		} else {
			return nil, fmt.Errorf("not a valid SpanRefType string %s", string(r.RefType))
		}

		traceID, err := model.TraceIDFromString(string(r.TraceID))
		if err != nil {
			return nil, err
		}

		spanID, err := strconv.ParseUint(string(r.SpanID), 16, 64)
		if err != nil {
			return nil, err
		}

		retMe[i] = model.SpanRef{
			RefType: refType,
			TraceID: traceID,
			SpanID:  model.SpanID(spanID),
		}
	}
	return retMe, nil
}

func (td toDomain) convertKeyValues(tags []json.KeyValue) ([]model.KeyValue, error) {
	retMe := make([]model.KeyValue, len(tags))
	for i := range tags {
		kv, err := td.convertKeyValue(&tags[i])
		if err != nil {
			return nil, err
		}
		retMe[i] = kv
	}
	return retMe, nil
}

func (td toDomain) convertKeyValue(tag *json.KeyValue) (model.KeyValue, error) {
	vType, err := model.ValueTypeFromString(string(tag.Type))
	if err != nil {
		return model.KeyValue{}, err
	}
	return td.convertKeyValueOfType(tag, vType)
}

func (td toDomain) convertKeyValueOfType(tag *json.KeyValue, vType model.ValueType) (model.KeyValue, error) {
	tagValue := tag.Value.(string)
	switch vType {
	case model.StringType:
		return model.String(tag.Key, tagValue), nil
	case model.BoolType:
		value, err := strconv.ParseBool(tagValue)
		if err != nil {
			return model.KeyValue{}, err
		}
		return model.Bool(tag.Key, value), nil
	case model.Int64Type:
		value, err := strconv.ParseInt(tagValue, 10, 64)
		if err != nil {
			return model.KeyValue{}, err
		}
		return model.Int64(tag.Key, value), nil
	case model.Float64Type:
		value, err := strconv.ParseFloat(tagValue, 64)
		if err != nil {
			return model.KeyValue{}, err
		}
		return model.Float64(tag.Key, value), nil
	case model.BinaryType:
		value, err := hex.DecodeString(tagValue)
		if err != nil {
			return model.KeyValue{}, err
		}
		return model.Binary(tag.Key, value), nil
	}
	return model.KeyValue{}, fmt.Errorf("not a valid ValueType string %s", vType.String())
}

func (td toDomain) convertLogs(logs []json.Log) ([]model.Log, error) {
	retMe := make([]model.Log, len(logs))
	for i, l := range logs {
		fields, err := td.convertKeyValues(l.Fields)
		if err != nil {
			return nil, err
		}
		retMe[i] = model.Log{
			Timestamp: model.EpochMicrosecondsAsTime(l.Timestamp),
			Fields:    fields,
		}
	}
	return retMe, nil
}

func (td toDomain) convertProcess(process *json.Process) (*model.Process, error) {
	if process == nil {
		return nil, errors.New("Process is nil")
	}
	tags, err := td.convertKeyValues(process.Tags)
	if err != nil {
		return nil, err
	}
	return &model.Process{
		Tags:        tags,
		ServiceName: process.ServiceName,
	}, nil
}
