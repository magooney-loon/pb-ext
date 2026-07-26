package alerts

import (
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"
)

// A Rule is a periodic check. The evaluator runs every rule on each tick and
// turns the boolean into edges: a rule that becomes true fires once and then
// stays quiet until it becomes false again, at which point it can send a
// recovery notice.
//
// Edges rather than levels is the difference between a useful alert channel and
// a muted one. "CPU above 90%" evaluated every 30 seconds without a state
// machine is 120 identical messages an hour.
type Rule struct {
	// Key identifies the rule. It is also the cooldown bucket, so a condition
	// that flaps across its threshold is damped by Config.Cooldown.
	Key string
	// Level is used when Check returns a message without one set.
	Level Level
	// Check reports whether the condition holds, plus the message to send on the
	// rising edge. It runs on the evaluator goroutine and may query the
	// database, but it must return promptly — it delays every other rule.
	Check func() (bool, Message)
	// Sustain is how many consecutive true results are required before firing.
	// Zero means fire on the first.
	Sustain int
	// Recovery sends a follow-up when the condition clears.
	Recovery bool
}

// maxRulePanics is how many times a rule may panic before it is disabled. A
// broken check must not take out the evaluator, and must not alert forever.
const maxRulePanics = 3

// ruleState carries the state machine for one rule. Every field except rule is
// guarded by ruleSet.mu.
type ruleState struct {
	rule     Rule
	firing   bool
	holds    int
	panics   int
	disabled bool
}

// ruleSet is the registered rules. Its zero value is an empty, usable set,
// which is what lets a disabled Notifier answer Stats.
type ruleSet struct {
	mu    sync.Mutex
	items []*ruleState
}

func (rs *ruleSet) add(r Rule) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.items = append(rs.items, &ruleState{rule: r})
}

func (rs *ruleSet) snapshot() []*ruleState {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return append([]*ruleState(nil), rs.items...)
}

func (rs *ruleSet) len() int {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return len(rs.items)
}

func (rs *ruleSet) counts() (total, firing int) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	for _, item := range rs.items {
		total++
		if item.firing {
			firing++
		}
	}
	return total, firing
}

// AddRule registers a custom check. Rules added before Initialize are lost, so
// call it on the notifier returned by Initialize (or via Get).
func (n *Notifier) AddRule(r Rule) error {
	if n == nil {
		return nil
	}
	if r.Key == "" {
		return errors.New("alerts: rule Key is required")
	}
	if r.Check == nil {
		return errors.New("alerts: rule Check is required")
	}
	n.rules.add(r)
	return nil
}

// evaluator runs the rules and refreshes the crash marker's heartbeat.
func (n *Notifier) evaluator() {
	ticker := time.NewTicker(n.cfg.EvaluateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-n.stop:
			return
		case <-ticker.C:
			n.heartbeat()
			n.evaluate(time.Now())
		}
	}
}

// evaluate takes one metrics sample and advances every rule's state machine.
func (n *Notifier) evaluate(now time.Time) {
	n.sample(now)

	for _, state := range n.rules.snapshot() {
		n.rules.mu.Lock()
		skip := state.disabled
		sustain := state.rule.Sustain
		n.rules.mu.Unlock()
		if skip {
			continue
		}
		if sustain <= 0 {
			sustain = 1
		}

		firing, msg, err := runCheck(state.rule)
		if err != nil {
			n.handleRulePanic(state, err)
			continue
		}

		n.rules.mu.Lock()
		if firing {
			state.holds++
		} else {
			state.holds = 0
		}
		wasFiring := state.firing
		nowFiring := state.holds >= sustain

		switch {
		case nowFiring && !wasFiring:
			state.firing = true
		case !firing && wasFiring:
			state.firing = false
		default:
			// Still firing, or still quiet: nothing to say.
			n.rules.mu.Unlock()
			continue
		}
		rule := state.rule
		n.rules.mu.Unlock()

		if nowFiring {
			if msg.Title == "" {
				msg.Title = rule.Key
			}
			if msg.Level == LevelInfo && rule.Level != LevelInfo {
				msg.Level = rule.Level
			}
			msg.Key = rule.Key
			n.Send(msg)
			continue
		}

		if rule.Recovery {
			n.Send(Message{
				Level: LevelInfo,
				// A separate cooldown bucket from the rule itself, so a recovery
				// is never swallowed by the alert that preceded it.
				Key:   rule.Key + ":recovered",
				Title: "Recovered: " + recoveryTitle(rule, msg),
			})
		}
	}
}

// runCheck isolates a rule's check from the evaluator. A panic in user code
// becomes an error rather than the end of alerting.
func runCheck(r Rule) (firing bool, msg Message, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("rule %q panicked: %v\n%s", r.Key, rec, debug.Stack())
		}
	}()

	firing, msg = r.Check()
	return firing, msg, nil
}

// handleRulePanic disables a repeatedly broken rule and says so, once.
func (n *Notifier) handleRulePanic(state *ruleState, err error) {
	n.rules.mu.Lock()
	state.panics++
	count := state.panics
	disable := count >= maxRulePanics
	if disable {
		state.disabled = true
	}
	key := state.rule.Key
	n.rules.mu.Unlock()

	if n.app != nil {
		n.app.Logger().Error("Alert rule panicked", "rule", key, "count", count, "error", err)
	}

	if disable {
		n.Send(Message{
			Level:     LevelWarn,
			Key:       "rule_disabled:" + key,
			Title:     "Alert rule disabled after repeated panics",
			Text:      err.Error(),
			Monospace: true,
			Fields:    map[string]string{"rule": key},
		})
	}
}

func recoveryTitle(r Rule, msg Message) string {
	if msg.Title != "" {
		return msg.Title
	}
	return r.Key
}

// --- metrics sampling ---

// evaluation is the derived view of the metric counters for one tick.
//
// Rates are differentiated here rather than read from a pre-computed field: a
// rate that is only recalculated when a request arrives keeps reporting the old
// figure forever once traffic stops, which is exactly when it matters.
type evaluation struct {
	prev    Metrics
	prevAt  time.Time
	hasPrev bool

	// valid is false on the first tick, when there is no interval to divide by.
	valid          bool
	current        Metrics
	requestRate    float64
	errorRate      float64
	windowRequests uint64

	baseline    float64
	hasBaseline bool
}

// baselineAlpha weights each new sample into the rolling request-rate baseline.
// At 0.2 with a 30s tick, the baseline follows roughly the last few minutes —
// slow enough that a spike stands out, fast enough to track a daily cycle.
const baselineAlpha = 0.2

// sample refreshes the derived metrics. Only the evaluator goroutine touches
// n.eval, so it needs no lock.
func (n *Notifier) sample(now time.Time) {
	n.eval.valid = false

	if n.metrics == nil {
		return
	}
	m := n.metrics()
	n.eval.current = m

	prev, prevAt, hasPrev := n.eval.prev, n.eval.prevAt, n.eval.hasPrev
	n.eval.prev, n.eval.prevAt, n.eval.hasPrev = m, now, true

	if !hasPrev {
		return
	}
	seconds := now.Sub(prevAt).Seconds()
	if seconds <= 0 {
		return
	}
	// Counters are monotonic within a process; going backwards means the source
	// was replaced, so skip the window rather than compute a negative rate.
	if m.Requests < prev.Requests || m.ServerErrors < prev.ServerErrors {
		return
	}

	requests := m.Requests - prev.Requests
	serverErrors := m.ServerErrors - prev.ServerErrors

	n.eval.windowRequests = requests
	n.eval.requestRate = float64(requests) / seconds
	n.eval.errorRate = 0
	if requests > 0 {
		n.eval.errorRate = float64(serverErrors) / float64(requests) * 100
	}
	n.eval.valid = true

	if n.eval.hasBaseline {
		n.eval.baseline = baselineAlpha*n.eval.requestRate + (1-baselineAlpha)*n.eval.baseline
	} else {
		n.eval.baseline = n.eval.requestRate
		n.eval.hasBaseline = true
	}
}

// registerBuiltinRules installs the threshold rules whose limits were
// configured. Each one is off unless its threshold is set: a figure that suits
// one deployment is either silent or deafening on another, so there is no
// defensible default.
func (n *Notifier) registerBuiltinRules() {
	t := n.cfg.Thresholds

	if t.ErrorRatePercent > 0 {
		_ = n.AddRule(Rule{
			Key:      "error_rate",
			Level:    LevelError,
			Recovery: true,
			Check: func() (bool, Message) {
				e := n.eval
				if !e.valid || e.windowRequests < uint64(t.ErrorRateMinRequests) {
					// Too few requests to judge: 1 error in 3 requests is not a
					// 33% error rate worth waking anyone for.
					return false, Message{}
				}
				if e.errorRate <= t.ErrorRatePercent {
					return false, Message{}
				}
				return true, Message{
					Level: LevelError,
					Title: fmt.Sprintf("Error rate %.1f%%", e.errorRate),
					Fields: map[string]string{
						"threshold": fmt.Sprintf("%.1f%%", t.ErrorRatePercent),
						"requests":  fmt.Sprintf("%d", e.windowRequests),
						"window":    n.cfg.EvaluateInterval.String(),
					},
				}
			},
		})
	}

	if t.SurgeFactor > 0 {
		_ = n.AddRule(Rule{
			Key:      "traffic_surge",
			Level:    LevelWarn,
			Recovery: true,
			Check: func() (bool, Message) {
				e := n.eval
				if !e.valid || !e.hasBaseline {
					return false, Message{}
				}
				// The floor is what stops traffic doubling from 1 to 2 req/s
				// from paging someone at 3am.
				if e.requestRate < t.SurgeFloorPerSec {
					return false, Message{}
				}
				if e.baseline <= 0 || e.requestRate < t.SurgeFactor*e.baseline {
					return false, Message{}
				}
				return true, Message{
					Level: LevelWarn,
					Title: fmt.Sprintf("Traffic surge: %.0f req/s", e.requestRate),
					Fields: map[string]string{
						"baseline": fmt.Sprintf("%.1f req/s", e.baseline),
						"factor":   fmt.Sprintf("%.1fx", t.SurgeFactor),
						"floor":    fmt.Sprintf("%.0f req/s", t.SurgeFloorPerSec),
					},
				}
			},
		})
	}

	n.addResourceRule("cpu_high", "CPU", t.CPUPercent, func() float64 { return n.eval.current.CPUPercent })
	n.addResourceRule("memory_high", "Memory", t.MemoryPercent, func() float64 { return n.eval.current.MemoryPercent })
	n.addResourceRule("disk_high", "Disk", t.DiskPercent, func() float64 { return n.eval.current.DiskPercent })

	if t.Goroutines > 0 {
		_ = n.AddRule(Rule{
			Key:      "goroutines_high",
			Level:    LevelWarn,
			Sustain:  t.SustainTicks,
			Recovery: true,
			Check: func() (bool, Message) {
				count := n.eval.current.Goroutines
				if count <= t.Goroutines {
					return false, Message{}
				}
				return true, Message{
					Level: LevelWarn,
					Title: fmt.Sprintf("Goroutine count %d", count),
					Fields: map[string]string{
						"threshold": fmt.Sprintf("%d", t.Goroutines),
					},
				}
			},
		})
	}
}

// addResourceRule registers a sustained-usage rule for one resource. Sustain
// filters out the momentary spikes that every host produces.
func (n *Notifier) addResourceRule(key, label string, threshold float64, read func() float64) {
	if threshold <= 0 {
		return
	}

	_ = n.AddRule(Rule{
		Key:      key,
		Level:    LevelWarn,
		Sustain:  n.cfg.Thresholds.SustainTicks,
		Recovery: true,
		Check: func() (bool, Message) {
			usage := read()
			if usage <= threshold {
				return false, Message{}
			}
			return true, Message{
				Level: LevelWarn,
				Title: fmt.Sprintf("%s usage %.1f%%", label, usage),
				Fields: map[string]string{
					"threshold": fmt.Sprintf("%.0f%%", threshold),
					"sustained": fmt.Sprintf("%d checks", n.cfg.Thresholds.SustainTicks),
				},
			}
		},
	})
}
