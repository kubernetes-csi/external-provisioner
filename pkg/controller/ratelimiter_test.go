/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"testing"
	"time"
)

const factorMaxDelay = 10

func TestRateLimiter(t *testing.T) {
	maxDelay := factorMaxDelay * time.Second
	rd := newItemExponentialFailureRateLimiterWithJitter(time.Second, maxDelay)

	for range 100 {
		backoff := rd.When(1)
		if backoff > maxDelay || backoff < 0 {
			t.Errorf("expected value > 0, < %s, got %s", maxDelay, backoff)
		}
		rd.Forget(1)
	}
}
