/**
 * Copyright 2017 Comcast Cable Communications Management, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */
package main

import (
	"errors"
	"github.com/Comcast/webpa-common/logging"
	"github.com/Comcast/webpa-common/secure/handler"
	"github.com/gorilla/mux"
	"github.com/justinas/alice"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

/*
Simply tests that no bad requests make it to the caduceus listener.
*/

func TestMuxServerConfig(t *testing.T) {
	assert := assert.New(t)

	logger := logging.DefaultLogger()
	fakeHandler := new(mockHandler)
	fakeHandler.On("HandleRequest", mock.AnythingOfType("int"),
		mock.AnythingOfType("CaduceusRequest")).Return().Once()

	fakeHealth := new(mockHealthTracker)
	fakeHealth.On("IncrementBucket", mock.AnythingOfType("int")).Return().Once()

	forceTimeOut := func(func(workerID int)) error {
		return errors.New("time out!")
	}

	fakeBodyError := new(mockCounter)
	fakeBodyError.On("Add", mock.AnythingOfType("float64")).Return().Times(0)

	fakeEnqueued := new(mockCounter)
	fakeEnqueued.On("Add", mock.AnythingOfType("float64")).Return().Once()

	fakeDropped := new(mockCounter)
	fakeDropped.On("Add", mock.AnythingOfType("float64")).Return().Times(0)

	serverWrapper := &ServerHandler{
		bodyErrorCounter:   fakeBodyError,
		msgEnqueuedCounter: fakeEnqueued,
		msgDroppedCounter:  fakeDropped,
		Logger:             logger,
		caduceusHandler:    fakeHandler,
		caduceusHealth:     fakeHealth,
		doJob:              forceTimeOut,
	}

	authHandler := handler.AuthorizationHandler{Validator: nil}
	caduceusHandler := alice.New(authHandler.Decorate)
	router := configServerRouter(mux.NewRouter(), caduceusHandler, serverWrapper)

	req := httptest.NewRequest("POST", "/api/v3/notify", nil)
	req.Header.Set("Content-Type", "application/json")

	t.Run("TestMuxResponseCorrectJSON", func(t *testing.T) {
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		resp := w.Result()

		assert.Equal(http.StatusRequestTimeout, resp.StatusCode)
	})

	t.Run("TestMuxResponseCorrectMSP", func(t *testing.T) {
		req.Header.Set("Content-Type", "application/msgpack")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		resp := w.Result()

		assert.Equal(http.StatusRequestTimeout, resp.StatusCode)
	})

	t.Run("TestMuxResponseManyHeaders", func(t *testing.T) {
		req.Header.Add("Content-Type", "too/many/headers")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp := w.Result()

		assert.Equal(http.StatusNotFound, resp.StatusCode)
	})

	t.Run("TestServeHTTPNoContentType", func(t *testing.T) {
		req.Header.Del("Content-Type")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp := w.Result()

		assert.Equal(http.StatusNotFound, resp.StatusCode)
	})

	t.Run("TestServeHTTPBadContentType", func(t *testing.T) {
		req.Header.Set("Content-Type", "something/unsupported")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp := w.Result()

		assert.Equal(http.StatusNotFound, resp.StatusCode)
	})
}
