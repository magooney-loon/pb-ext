package analytics

import (
	"sync"
	"time"
)

// counterKey identifies one row of the _analytics daily counter table.
// It is a comparable struct so it can be a map key without string concatenation.
type counterKey struct {
	Path       string
	Date       string
	DeviceType string
	Browser    string
}

// counterDelta is the accumulated, not-yet-persisted change for one counterKey.
type counterDelta struct {
	Views             int
	NewSessions       int
	ReturningSessions int
}

// visit is one recorded page view handed to the aggregator. The date is derived
// from At under the aggregator's lock so the hot path avoids a time.Format
// allocation on every request.
type visit struct {
	Path       string
	DeviceType string
	Browser    string
	OS         string
	Class      visitClass
	At         time.Time
}

// aggregator folds page views into per-key counters in memory so the request
// path never touches SQLite. It also owns the two purely in-memory dashboard
// inputs: the recent-visit ring and the trailing-hour activity buckets.
//
// Everything it holds is bounded: pending keys by maxPending, distinct paths per
// day by maxDistinctPaths, recent visits by SessionRingSize, and activity by a
// fixed 60-bucket ring.
type aggregator struct {
	mu      sync.Mutex
	pending map[counterKey]*counterDelta

	// maxPending is a flush trigger rather than a hard cap: a view is never
	// dropped on the way in. The standing memory bound comes from
	// maxDistinctPaths, since date, device and browser are all low-cardinality.
	maxPending       int
	maxDistinctPaths int

	// Memoized "YYYY-MM-DD" for the current day, so formatting happens once per
	// day rather than once per page view.
	cachedDate    string
	cachedDateDay int64

	// Per-day distinct-path budget. Reset when the date rolls over.
	pathDate  string
	pathsSeen map[string]struct{}

	// Recent-visit ring (newest written at recentHead).
	recent      [SessionRingSize]RecentVisit
	recentHead  int
	recentCount int

	// Trailing-hour activity: one bucket per minute plus a running total so
	// reads stay O(1) instead of summing 60 buckets per request.
	buckets     [hourlyBuckets]int
	bucketIdx   int
	bucketStamp int64 // unix minute currently at bucketIdx
	hourTotal   int
	peakHourly  int
}

func newAggregator(maxPending, maxDistinctPaths int, now time.Time) *aggregator {
	return &aggregator{
		pending:          make(map[counterKey]*counterDelta),
		maxPending:       maxPending,
		maxDistinctPaths: maxDistinctPaths,
		pathsSeen:        make(map[string]struct{}),
		bucketStamp:      now.Unix() / 60,
	}
}

// record folds v into the pending counters. It reports whether the pending set
// has reached its ceiling and should be flushed early.
func (a *aggregator) record(v visit) (needsFlush bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	date := a.dateLocked(v.At)
	key := counterKey{
		Path:       a.budgetPathLocked(v.Path, date),
		Date:       date,
		DeviceType: v.DeviceType,
		Browser:    v.Browser,
	}

	delta, ok := a.pending[key]
	if !ok {
		delta = &counterDelta{}
		a.pending[key] = delta
	}

	delta.Views++
	switch v.Class {
	case visitNew:
		delta.NewSessions++
	case visitReturning:
		delta.ReturningSessions++
	}

	a.pushRecentLocked(RecentVisit{
		Time:       v.At,
		Path:       key.Path,
		DeviceType: v.DeviceType,
		Browser:    v.Browser,
		OS:         v.OS,
	})
	a.tickActivityLocked(v.At)

	return len(a.pending) >= a.maxPending
}

// dateLocked returns the "YYYY-MM-DD" string for at, reformatting only when the
// local day changes. Callers must hold a.mu.
func (a *aggregator) dateLocked(at time.Time) string {
	// Local midnight boundaries, matching time.Format's calendar day.
	day := at.Unix() + int64(zoneOffsetSeconds(at))
	day /= 86400

	if a.cachedDate == "" || day != a.cachedDateDay {
		a.cachedDate = at.Format("2006-01-02")
		a.cachedDateDay = day
	}
	return a.cachedDate
}

func zoneOffsetSeconds(at time.Time) int {
	_, offset := at.Zone()
	return offset
}

// budgetPathLocked keeps per-day path cardinality bounded, collapsing the tail
// into OverflowPath. Callers must hold a.mu.
func (a *aggregator) budgetPathLocked(path, date string) string {
	if a.pathDate != date {
		a.pathDate = date
		a.pathsSeen = make(map[string]struct{}, a.maxDistinctPaths)
	}

	if _, seen := a.pathsSeen[path]; seen {
		return path
	}
	if len(a.pathsSeen) >= a.maxDistinctPaths {
		return OverflowPath
	}

	a.pathsSeen[path] = struct{}{}
	return path
}

// pushRecentLocked writes into the recent-visit ring. Callers must hold a.mu.
func (a *aggregator) pushRecentLocked(rv RecentVisit) {
	a.recent[a.recentHead] = rv
	a.recentHead = (a.recentHead + 1) % SessionRingSize
	if a.recentCount < SessionRingSize {
		a.recentCount++
	}
}

// tickActivityLocked advances the minute buckets to now and counts one visit.
// Callers must hold a.mu.
func (a *aggregator) tickActivityLocked(now time.Time) {
	a.advanceActivityLocked(now)
	a.buckets[a.bucketIdx]++
	a.hourTotal++
	if a.hourTotal > a.peakHourly {
		a.peakHourly = a.hourTotal
	}
}

// advanceActivityLocked zeroes buckets for every minute that has elapsed since
// the last update, so the trailing-hour total decays even without traffic.
// Callers must hold a.mu.
func (a *aggregator) advanceActivityLocked(now time.Time) {
	minute := now.Unix() / 60
	elapsed := minute - a.bucketStamp
	if elapsed <= 0 {
		return
	}

	if elapsed >= hourlyBuckets {
		// More than an hour of silence — the whole window is stale.
		clear(a.buckets[:])
		a.hourTotal = 0
		a.bucketIdx = 0
		a.bucketStamp = minute
		return
	}

	for i := int64(0); i < elapsed; i++ {
		a.bucketIdx = (a.bucketIdx + 1) % hourlyBuckets
		a.hourTotal -= a.buckets[a.bucketIdx]
		a.buckets[a.bucketIdx] = 0
	}
	a.bucketStamp = minute
}

// drain removes and returns the pending counters, leaving the aggregator empty.
func (a *aggregator) drain() map[counterKey]*counterDelta {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.pending) == 0 {
		return nil
	}

	drained := a.pending
	a.pending = make(map[counterKey]*counterDelta, len(drained))
	return drained
}

// restore folds a failed flush back into the pending set so a transient
// database error doesn't lose counts. Keys beyond maxPending are dropped rather
// than allowed to accumulate without bound.
func (a *aggregator) restore(deltas map[counterKey]*counterDelta) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for key, delta := range deltas {
		existing, ok := a.pending[key]
		if !ok {
			if len(a.pending) >= a.maxPending {
				continue
			}
			a.pending[key] = delta
			continue
		}
		existing.Views += delta.Views
		existing.NewSessions += delta.NewSessions
		existing.ReturningSessions += delta.ReturningSessions
	}
}

// pendingLen reports how many counter keys are awaiting a flush.
func (a *aggregator) pendingLen() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.pending)
}

// recentVisits returns the ring newest-first.
func (a *aggregator) recentVisits(limit int) []RecentVisit {
	a.mu.Lock()
	defer a.mu.Unlock()

	if limit > a.recentCount {
		limit = a.recentCount
	}
	out := make([]RecentVisit, 0, limit)
	for i := 0; i < limit; i++ {
		// recentHead points at the next write slot, so step backwards from it.
		idx := (a.recentHead - 1 - i + SessionRingSize*2) % SessionRingSize
		out = append(out, a.recent[idx])
	}
	return out
}

// activity reports visits in the trailing hour and the denominator to render
// them against: the running peak, floored at MaxExpectedHourlyVisits so the bar
// stays sane on a quiet site and never pegs at 100% on a busy one.
func (a *aggregator) activity(now time.Time) (count int, scale int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.advanceActivityLocked(now)

	scale = a.peakHourly
	if scale < MaxExpectedHourlyVisits {
		scale = MaxExpectedHourlyVisits
	}
	return a.hourTotal, scale
}
