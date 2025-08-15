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
	"math/rand"
	"sync"
	"time"

	"k8s.io/client-go/util/workqueue"
)

type rateLimiterWithJitter struct {
	workqueue.TypedRateLimiter[any]
	baseDelay time.Duration
	rd        *rand.Rand
	mutex     sync.Mutex
}

func (r *rateLimiterWithJitter) When(item any) time.Duration {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	delay := r.TypedRateLimiter.When(item).Nanoseconds()
	percentage := r.rd.Float64()
	jitter := int64(float64(r.baseDelay.Nanoseconds()) * percentage)
	if jitter > delay {
		return 0
	}
	return time.Duration(delay - jitter)
}

func newItemExponentialFailureRateLimiterWithJitter(baseDelay time.Duration, maxDelay time.Duration) workqueue.TypedRateLimiter[any] {
	return &rateLimiterWithJitter{
		TypedRateLimiter: workqueue.NewTypedItemExponentialFailureRateLimiter[any](baseDelay, maxDelay),
		baseDelay:        baseDelay,
		rd:               rand.New(rand.NewSource(time.Now().UTC().UnixNano())),
	}
}
