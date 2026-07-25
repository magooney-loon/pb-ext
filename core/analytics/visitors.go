package analytics

import (
	"sync"
	"time"
)

// visitClass describes how a request relates to the visitor's session history.
type visitClass int

const (
	// visitContinued is a page view inside an already-open session.
	visitContinued visitClass = iota
	// visitNew opens a session for a visitor not present in the tracker.
	visitNew
	// visitReturning opens a session for a visitor whose previous session lapsed.
	visitReturning
)

// visitorTracker answers "is this a new session, and have we seen this visitor
// before?" from memory, with a hard bound on how much memory it can use.
//
// Entries are held in a fixed number of generations. The newest generation
// absorbs writes; a rotation drops the oldest generation wholesale. Rotation is
// triggered by age (every sessionWindow) or by size (the newest generation
// filling up), so a flood of forged visitor keys costs bounded memory and O(1)
// amortized work instead of an unbounded map plus a full scan under lock.
//
// The trade-off is that visitor identity is only remembered for up to
// generations*sessionWindow; a visitor returning after that reads as new.
type visitorTracker struct {
	mu   sync.Mutex
	gens []map[uint64]time.Time

	maxPerGen     int
	sessionWindow time.Duration
	lastRotate    time.Time
}

func newVisitorTracker(generations, maxPerGen int, sessionWindow time.Duration, now time.Time) *visitorTracker {
	if generations < 1 {
		generations = 1
	}
	if maxPerGen < 1 {
		maxPerGen = 1
	}

	gens := make([]map[uint64]time.Time, generations)
	for i := range gens {
		gens[i] = make(map[uint64]time.Time)
	}

	return &visitorTracker{
		gens:          gens,
		maxPerGen:     maxPerGen,
		sessionWindow: sessionWindow,
		lastRotate:    now,
	}
}

// classify records a visit for key and reports how it relates to prior activity.
func (v *visitorTracker) classify(key uint64, now time.Time) visitClass {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.rotateIfNeededLocked(now)

	for i, gen := range v.gens {
		lastSeen, ok := gen[key]
		if !ok {
			continue
		}

		// Promote to the newest generation so active visitors survive rotation.
		if i != 0 {
			delete(gen, key)
		}
		v.gens[0][key] = now

		if now.Sub(lastSeen) < v.sessionWindow {
			return visitContinued
		}
		return visitReturning
	}

	v.gens[0][key] = now
	return visitNew
}

// rotateIfNeededLocked ages out the oldest generation when the newest is full
// or a full session window has elapsed. Callers must hold v.mu.
func (v *visitorTracker) rotateIfNeededLocked(now time.Time) {
	if len(v.gens[0]) < v.maxPerGen && now.Sub(v.lastRotate) < v.sessionWindow {
		return
	}

	// Shift right, reusing the evicted map's allocation for the new generation.
	oldest := v.gens[len(v.gens)-1]
	copy(v.gens[1:], v.gens[:len(v.gens)-1])
	clear(oldest)
	v.gens[0] = oldest

	v.lastRotate = now
}

// size reports the total number of tracked visitors across all generations.
func (v *visitorTracker) size() int {
	v.mu.Lock()
	defer v.mu.Unlock()

	total := 0
	for _, gen := range v.gens {
		total += len(gen)
	}
	return total
}

// capacity is the hard upper bound on tracked entries.
func (v *visitorTracker) capacity() int {
	return len(v.gens) * v.maxPerGen
}
