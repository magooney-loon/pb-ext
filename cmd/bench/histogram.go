package main

import (
	"math/bits"
	"sync/atomic"
	"time"
)

// numBuckets covers every representable time.Duration (nanoseconds fit in an
// int64, so bits.Len64 never exceeds 63).
const numBuckets = 64

// histogram is a bounded-memory latency histogram: one atomic counter per
// power-of-two nanosecond bucket, so a stage can run for any duration or
// request volume without storing a per-request sample.
type histogram struct {
	buckets [numBuckets]atomic.Int64
}

// record adds one observation. Bucket i holds values in [2^(i-1), 2^i) for
// i >= 1, and bucket 0 holds exactly zero.
func (h *histogram) record(d time.Duration) {
	if d < 0 {
		d = 0
	}
	idx := bits.Len64(uint64(d))
	if idx >= numBuckets {
		idx = numBuckets - 1
	}
	h.buckets[idx].Add(1)
}

// count returns the total number of observations recorded.
func (h *histogram) count() int64 {
	var total int64
	for i := range h.buckets {
		total += h.buckets[i].Load()
	}
	return total
}

// percentile estimates the p-th percentile (0 < p <= 1) as the upper bound of
// the bucket containing that rank. This is a coarse (2x worst-case)
// approximation, adequate for a stress tool that needs to run unattended
// rather than store every latency.
func (h *histogram) percentile(p float64) time.Duration {
	total := h.count()
	if total == 0 {
		return 0
	}

	target := int64(p * float64(total))
	if target < 1 {
		target = 1
	}

	var cumulative int64
	for i := range h.buckets {
		cumulative += h.buckets[i].Load()
		if cumulative >= target {
			return bucketUpperBound(i)
		}
	}
	return bucketUpperBound(numBuckets - 1)
}

// max returns the upper bound of the highest non-empty bucket.
func (h *histogram) max() time.Duration {
	for i := numBuckets - 1; i >= 0; i-- {
		if h.buckets[i].Load() > 0 {
			return bucketUpperBound(i)
		}
	}
	return 0
}

func bucketUpperBound(i int) time.Duration {
	if i <= 0 {
		return 0
	}
	if i >= 63 {
		return time.Duration(1<<63 - 1)
	}
	return time.Duration(1) << uint(i)
}
