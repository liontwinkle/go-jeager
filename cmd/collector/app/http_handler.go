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

package app

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/gorilla/mux"
	tchanThrift "github.com/uber/tchannel-go/thrift"

	tJaeger "github.com/uber/jaeger/thrift-gen/jaeger"
	"github.com/uber/jaeger/thrift-gen/zipkincore"
)

const (
	formatParam               = "format"
	unableToReadBodyErrFormat = "Unable to process request body: %v"
)

// APIHandler handles all HTTP calls to the collector
type APIHandler struct {
	jaegerBatchesHandler JaegerBatchesHandler
	zipkinSpansHandler   ZipkinSpansHandler
}

// NewAPIHandler returns a new APIHandler
func NewAPIHandler(
	jaegerBatchesHandler JaegerBatchesHandler,
	zipkinSpansHandler ZipkinSpansHandler,
) *APIHandler {
	return &APIHandler{
		jaegerBatchesHandler: jaegerBatchesHandler,
		zipkinSpansHandler:   zipkinSpansHandler,
	}
}

// RegisterRoutes registers routes for this handler on the given router
func (aH *APIHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/api/traces", aH.saveSpan).Methods(http.MethodPost)
}

func (aH *APIHandler) saveSpan(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		http.Error(w, fmt.Sprintf(unableToReadBodyErrFormat, err), http.StatusInternalServerError)
		return
	}

	format := r.FormValue(formatParam)
	switch strings.ToLower(format) {
	case "jaeger.thrift":
		tdes := thrift.NewTDeserializer()
		// (NB): We decided to use this struct instead of straight batches to be as consistent with tchannel intake as possible.
		var req tJaeger.CollectorSubmitBatchesArgs
		if err = tdes.Read(&req, bodyBytes); err != nil {
			http.Error(w, fmt.Sprintf(unableToReadBodyErrFormat, err), http.StatusBadRequest)
			return
		}
		ctx, cancel := tchanThrift.NewContext(time.Minute)
		defer cancel()
		if _, err = aH.jaegerBatchesHandler.SubmitBatches(ctx, req.Batches); err != nil {
			http.Error(w, fmt.Sprintf("Cannot submit Jaeger batch: %v", err), http.StatusInternalServerError)
			return
		}

	case "zipkin.thrift":
		spans, err := deserializeZipkin(bodyBytes)
		if err != nil {
			http.Error(w, fmt.Sprintf(unableToReadBodyErrFormat, err), http.StatusBadRequest)
			return
		}

		ctx, _ := tchanThrift.NewContext(time.Minute)
		if _, err = aH.zipkinSpansHandler.SubmitZipkinBatch(ctx, spans); err != nil {
			http.Error(w, fmt.Sprintf("Cannot submit Zipkin batch: %v", err), http.StatusInternalServerError)
			return
		}

	default:
		http.Error(w, fmt.Sprintf("Unsupported format type: %v", format), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func deserializeZipkin(b []byte) ([]*zipkincore.Span, error) {
	buffer := thrift.NewTMemoryBuffer()
	buffer.Write(b)

	transport := thrift.NewTBinaryProtocolTransport(buffer)
	_, size, err := transport.ReadListBegin() // Ignore the returned element type
	if err != nil {
		return nil, err
	}

	// We don't depend on the size returned by ReadListBegin to preallocate the array because it
	// sometimes returns a nil error on bad input and provides an unreasonably large int for size
	var spans []*zipkincore.Span
	for i := 0; i < size; i++ {
		zs := &zipkincore.Span{}
		if err = zs.Read(transport); err != nil {
			return nil, err
		}
		spans = append(spans, zs)
	}

	return spans, nil
}
