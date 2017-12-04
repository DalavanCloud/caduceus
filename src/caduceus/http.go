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
	"github.com/Comcast/webpa-common/logging"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics/provider"
	"io/ioutil"
	"net/http"
	"time"
)

type Send func(inFunc func(workerID int)) error

// Below is the struct that will implement our ServeHTTP method
type ServerHandler struct {
	log.Logger
	provider.Provider
	caduceusHandler RequestHandler
	caduceusHealth  HealthTracker
	doJob           Send
}

func (sh *ServerHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	debugLog := logging.Debug(sh.Logger)
	infoLog := logging.Info(sh.Logger)
	errorLog := logging.Error(sh.Logger)
	messageKey := logging.MessageKey()
	errorKey := logging.ErrorKey()

	infoLog.Log(messageKey, "Receiving incoming request...")

	stats := CaduceusTelemetry{
		TimeReceived: time.Now(),
	}

	payload, err := ioutil.ReadAll(request.Body)
	if err != nil {
		errorLog.Log(messageKey, "Unable to retrieve the request body.", errorKey, err.Error)
		return
	}

	targetURL := request.URL.String()

	caduceusRequest := CaduceusRequest{
		RawPayload:  payload,
		ContentType: request.Header.Get("Content-Type"),
		TargetURL:   targetURL,
		Telemetry:   stats,
	}

	caduceusRequest.Telemetry.RawPayloadSize = len(payload)
	caduceusRequest.Telemetry.TimeAccepted = time.Now()

	err = sh.doJob(func(workerID int) { sh.caduceusHandler.HandleRequest(workerID, caduceusRequest) })

	if err != nil {
		// return a 408
		response.WriteHeader(http.StatusRequestTimeout)
		response.Write([]byte("Unable to handle request at this time.\n"))
		debugLog.Log(messageKey, "Unable to handle request at this time.")
	} else {
		// return a 202
		response.WriteHeader(http.StatusAccepted)
		response.Write([]byte("Request placed on to queue.\n"))
		debugLog.Log(messageKey, "Request placed on to queue.")

		sh.caduceusHealth.IncrementBucket(caduceusRequest.Telemetry.RawPayloadSize)
	}
}
