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

package mocks

import cassandra "github.com/uber/jaeger/pkg/cassandra"
import mock "github.com/stretchr/testify/mock"

// Session is an autogenerated mock type for the Session type
type Session struct {
	mock.Mock
}

// Close provides a mock function with given fields:
func (_m *Session) Close() {
	_m.Called()
}

// Query provides a mock function with given fields: stmt, values
func (_m *Session) Query(stmt string, values ...interface{}) cassandra.Query {
	ret := _m.Called(stmt, values)

	var r0 cassandra.Query
	if rf, ok := ret.Get(0).(func(string, ...interface{}) cassandra.Query); ok {
		r0 = rf(stmt, values...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(cassandra.Query)
		}
	}

	return r0
}

var _ cassandra.Session = (*Session)(nil)
