package chihttp_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/c2h5oh/datasize"
	chihttp "github.com/go-modulus/chihttp"
	modhttp "github.com/go-modulus/modulus/http"
	"github.com/go-modulus/modulus/http/errhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func emptyPipeline() *errhttp.ErrorPipeline { return &errhttp.ErrorPipeline{} }

type ErrorMessage struct {
	Message string `json:"message"`
}

type ErrorsResponse struct {
	Errors []ErrorMessage `json:"errors"`
}

func TestNewRouter(t *testing.T) {
	t.Parallel()

	t.Run(
		"route not found", func(t *testing.T) {
			t.Parallel()

			r := chihttp.NewRouter(emptyPipeline(), modhttp.ServeConfig{})
			r.Get(
				"/exists", http.HandlerFunc(
					func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
					},
				),
			)

			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/does-not-exist", nil))

			require.Equal(t, http.StatusBadRequest, rr.Code)
			var body ErrorsResponse
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
			assert.Contains(t, rr.Header().Get("Content-Type"), "application/json")
			require.Len(t, body.Errors, 1)
			require.Equal(t, "Not found", body.Errors[0].Message)
		},
	)

	t.Run(
		"method not allowed", func(t *testing.T) {
			t.Parallel()

			r := chihttp.NewRouter(emptyPipeline(), modhttp.ServeConfig{})
			r.Get(
				"/resource", http.HandlerFunc(
					func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
					},
				),
			)

			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/resource", nil))

			require.Equal(t, http.StatusBadRequest, rr.Code)
			var body ErrorsResponse
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
			assert.Contains(t, rr.Header().Get("Content-Type"), "application/json")
			require.Len(t, body.Errors, 1)
			require.Equal(t, "Method not allowed", body.Errors[0].Message)
		},
	)

	t.Run(
		"registered route reachable", func(t *testing.T) {
			t.Parallel()
			r := chihttp.NewRouter(emptyPipeline(), modhttp.ServeConfig{})
			r.Get(
				"/ping", http.HandlerFunc(
					func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
					},
				),
			)

			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/ping", nil))

			assert.Equal(t, http.StatusOK, rr.Code)
		},
	)

	t.Run(
		"context has deadline", func(t *testing.T) {
			t.Parallel()

			var hasDeadline bool
			r := chihttp.NewRouter(emptyPipeline(), modhttp.ServeConfig{TTL: 500 * time.Millisecond})
			r.Get(
				"/", http.HandlerFunc(
					func(w http.ResponseWriter, req *http.Request) {
						_, hasDeadline = req.Context().Deadline()
						w.WriteHeader(http.StatusOK)
					},
				),
			)

			r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

			assert.True(t, hasDeadline, "context should have a deadline when TTL > 0")
		},
	)

	t.Run(
		"context has no deadline", func(t *testing.T) {
			t.Parallel()

			var hasDeadline bool
			r := chihttp.NewRouter(emptyPipeline(), modhttp.ServeConfig{TTL: 0})
			r.Get(
				"/", http.HandlerFunc(
					func(w http.ResponseWriter, req *http.Request) {
						_, hasDeadline = req.Context().Deadline()
						w.WriteHeader(http.StatusOK)
					},
				),
			)

			r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

			assert.False(t, hasDeadline, "context should have no deadline when TTL is 0")
		},
	)

	t.Run(
		"size limit reached", func(t *testing.T) {
			t.Parallel()

			var readErr error
			r := chihttp.NewRouter(emptyPipeline(), modhttp.ServeConfig{RequestSizeLimit: 5 * datasize.B})
			r.Post(
				"/upload", http.HandlerFunc(
					func(w http.ResponseWriter, req *http.Request) {
						_, readErr = io.ReadAll(req.Body)
						w.WriteHeader(http.StatusOK)
					},
				),
			)

			body := strings.NewReader("more than five bytes")
			r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/upload", body))

			require.Error(t, readErr, "reading past the size limit should return an error")
		},
	)

	t.Run(
		"size limit not reached", func(t *testing.T) {
			t.Parallel()

			var readErr error
			r := chihttp.NewRouter(emptyPipeline(), modhttp.ServeConfig{RequestSizeLimit: 100 * datasize.B})
			r.Post(
				"/upload", http.HandlerFunc(
					func(w http.ResponseWriter, req *http.Request) {
						_, readErr = io.ReadAll(req.Body)
						w.WriteHeader(http.StatusOK)
					},
				),
			)

			body := bytes.NewReader([]byte("small"))
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/upload", body))

			require.NoError(t, readErr)
			assert.Equal(t, http.StatusOK, rr.Code)
		},
	)

	t.Run(
		"no size limit if RequestSizeLimit is 0", func(t *testing.T) {
			t.Parallel()

			var readErr error
			r := chihttp.NewRouter(emptyPipeline(), modhttp.ServeConfig{RequestSizeLimit: 0})
			r.Post(
				"/upload", http.HandlerFunc(
					func(w http.ResponseWriter, req *http.Request) {
						_, readErr = io.ReadAll(req.Body)
						w.WriteHeader(http.StatusOK)
					},
				),
			)

			body := strings.NewReader("this is a fairly large body that would exceed a small limit")
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/upload", body))

			require.NoError(t, readErr)
			assert.Equal(t, http.StatusOK, rr.Code)
		},
	)
}
