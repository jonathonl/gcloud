// Copyright 2015 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package httputil

import (
	"fmt"
	"net/http"
	"reflect"
	"time"

	"golang.org/x/net/context"
)

// An interface for transports that support the signature of
// http.Transport.CancelRequest.
type canceller interface {
	CancelRequest(*http.Request)
}

// Wait until the context is cancelled, then cancel the supplied HTTP request.
func waitForCancellation(
	ctx context.Context,
	c canceller,
	req *http.Request) {
	// If there is no done channel, there's nothing we can do.
	done := ctx.Done()
	if done == nil {
		return
	}

	// Wait, then cancel.
	<-done
	c.CancelRequest(req)

	// HACK(jacobsa): The http package's design for cancellation seems flawed:
	// the canceller must naturally race with the transport receiving the
	// request. If it wins the race (which is not unlikely if our Do function is
	// called with a pre-cancelled context), the cancellation will be lost.
	// Attempt to work around this by sleeping a bit and cancelling again.
	time.Sleep(10 * time.Millisecond)
	c.CancelRequest(req)
}

// Call client.Do with the supplied request, cancelling the request if the
// context is cancelled. Return an error if the client does not support
// cancellation.
func Do(
	ctx context.Context,
	client http.Client,
	req *http.Request) (resp *http.Response, err error) {
	// Make sure the transport supports cancellation.
	c, ok := client.Transport.(canceller)
	if !ok {
		err = fmt.Errorf(
			"Transport of type %v doesn't support cancellation",
			reflect.TypeOf(client.Transport))
		return
	}

	// Wait for cancellation in the background. Note that this goroutine should
	// eventually return, because context.WithCancel requires the caller to
	// arrange for cancellation to happen eventually.
	go waitForCancellation(ctx, c, req)

	// Call through.
	resp, err = client.Do(req)

	return
}
