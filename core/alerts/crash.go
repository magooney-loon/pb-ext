package alerts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// "Tell me when the server crashes" cannot be answered from inside the process
// that crashed. An OOM kill, a log.Fatal, a panic on a goroutine nobody
// recovers, a kernel panic: the process is gone in microseconds, and a Telegram
// delivery needs hundreds of milliseconds. Trying to send from a dying process
// buys a hung exit and no message.
//
// So the detection happens on the way back up. Each run leaves a marker file in
// the data directory saying "running", refreshed on a heartbeat and set to
// "stopped" by the shutdown hook. A marker still reading "running" at boot means
// the previous process never reached its shutdown hook — and the heartbeat
// timestamp says roughly when it stopped, which turns "something happened" into
// "it died at 14:31", the thing you actually grep the logs for.
//
// A missing, unreadable or corrupt marker produces no alert. That is deliberate:
// a read-only or ephemeral data directory would otherwise report a crash at
// every single boot, and an integration that cries wolf gets muted within a day.
// False silence is recoverable; false alarms are not.

const (
	// markerFile lives in the data directory, next to the databases.
	//
	// A file rather than a row in auxiliary.db: deleting that database is
	// explicitly a reasonable thing to do — it holds only logs and counters —
	// and the marker has to outlive it to be worth anything. A file also
	// survives a corrupted database and can be read with cat.
	markerFile = ".pbext_lastrun.json"

	stateRunning = "running"
	stateStopped = "stopped"

	// restartLoopWindow is how soon after the previous heartbeat a fresh start
	// counts as part of the same crash loop rather than an isolated incident.
	restartLoopWindow = 10 * time.Minute

	// restartLoopThreshold is how many consecutive fast restarts switch the
	// alert from "it crashed" to "it is crash looping".
	restartLoopThreshold = 3
)

// lastRun is the marker file's contents.
type lastRun struct {
	PID      int       `json:"pid"`
	Started  time.Time `json:"started"`
	Beat     time.Time `json:"beat"`
	State    string    `json:"state"`
	Restarts int       `json:"restarts"`
}

// markerState tracks this run's marker plus whatever the previous run left.
type markerState struct {
	path        string
	previous    lastRun
	hasPrevious bool

	mu      sync.Mutex
	current lastRun
	failed  bool
}

func markerPath(dataDir string) string {
	if dataDir == "" {
		return ""
	}
	return filepath.Join(dataDir, markerFile)
}

// readMarker loads the previous run's marker. Any problem reads as "no
// information", never as a crash.
func readMarker(path string) (lastRun, bool) {
	if path == "" {
		return lastRun{}, false
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return lastRun{}, false
	}

	var lr lastRun
	if err := json.Unmarshal(data, &lr); err != nil {
		return lastRun{}, false
	}
	if lr.State == "" {
		return lastRun{}, false
	}
	return lr, true
}

// writeMarker replaces the marker atomically, so a crash mid-write cannot leave
// a truncated file that reads as "no information" on the next boot.
func writeMarker(path string, lr lastRun) error {
	if path == "" {
		return nil
	}

	data, err := json.Marshal(lr)
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// save persists the current marker, remembering a failure so a read-only data
// directory produces one debug line rather than one per heartbeat.
func (n *Notifier) saveMarker(lr lastRun) {
	n.marker.mu.Lock()
	n.marker.current = lr
	alreadyFailed := n.marker.failed
	n.marker.mu.Unlock()

	err := writeMarker(n.marker.path, lr)
	if err == nil {
		return
	}

	n.marker.mu.Lock()
	n.marker.failed = true
	n.marker.mu.Unlock()

	if !alreadyFailed && n.app != nil {
		n.app.Logger().Debug("Could not write the pb-ext run marker; crash detection is unavailable",
			"path", n.marker.path, "error", err)
	}
}

// heartbeat refreshes the marker's timestamp. Called on every evaluator tick,
// it is what lets the next boot say when the previous run stopped.
func (n *Notifier) heartbeat() {
	n.marker.mu.Lock()
	lr := n.marker.current
	n.marker.mu.Unlock()

	if lr.State != stateRunning {
		return
	}
	lr.Beat = time.Now()
	n.saveMarker(lr)
}

// NotifyStarted claims the marker for this run and reports how the previous one
// ended. Call it once the server is actually serving.
//
// It is safe on a disabled notifier: the marker is still maintained, so
// enabling alerts later does not require a clean shutdown first.
func (n *Notifier) NotifyStarted() {
	if n == nil {
		return
	}

	now := time.Now()
	prev := n.marker.previous
	unclean := n.marker.hasPrevious && prev.State == stateRunning

	// A fast restart after an unclean exit is the same incident continuing.
	restarts := 0
	if unclean && !prev.Beat.IsZero() && now.Sub(prev.Beat) < restartLoopWindow {
		restarts = prev.Restarts + 1
	}

	n.saveMarker(lastRun{
		PID:      os.Getpid(),
		Started:  now,
		Beat:     now,
		State:    stateRunning,
		Restarts: restarts,
	})

	if !n.cfg.Lifecycle {
		return
	}

	if !unclean {
		n.Send(Message{
			Level: LevelInfo,
			Key:   "server_started",
			Title: "Server started",
			Fields: map[string]string{
				"pid": fmt.Sprintf("%d", os.Getpid()),
			},
		})
		return
	}

	fields := map[string]string{
		"previous pid": fmt.Sprintf("%d", prev.PID),
	}
	if !prev.Started.IsZero() && !prev.Beat.IsZero() {
		fields["ran for"] = prev.Beat.Sub(prev.Started).Round(time.Second).String()
	}
	if !prev.Beat.IsZero() {
		fields["last heartbeat"] = prev.Beat.UTC().Format("2006-01-02 15:04:05 MST")
		fields["undetected for"] = now.Sub(prev.Beat).Round(time.Second).String()
	}

	if restarts >= restartLoopThreshold {
		n.Send(Message{
			Level:  LevelCritical,
			Key:    "server_crash_loop",
			Title:  fmt.Sprintf("Server is crash looping (%d restarts)", restarts+1),
			Text:   "Each restart has followed an unexpected exit within " + restartLoopWindow.String() + ". Further restarts will be held back by the cooldown.",
			Fields: fields,
		})
		return
	}

	n.Send(Message{
		Level:  LevelCritical,
		Key:    "server_crashed",
		Title:  "Server recovered from an unexpected exit",
		Text:   "The previous process did not run its shutdown hook — a crash, an OOM kill, or a host that went away.",
		Fields: fields,
	})
}

// NotifyStopped marks a clean shutdown and, unless this is a restart, says so.
//
// Call it before Close: this queues a message, and Close is what drains the
// queue. A PocketBase restart (the dev-mode reload) marks the run clean but
// stays silent — a file save is not an incident.
func (n *Notifier) NotifyStopped(isRestart bool) {
	if n == nil {
		return
	}

	n.marker.mu.Lock()
	lr := n.marker.current
	n.marker.mu.Unlock()

	lr.State = stateStopped
	lr.Beat = time.Now()
	if lr.Started.IsZero() {
		lr.Started = lr.Beat
	}
	n.saveMarker(lr)

	if !n.cfg.Lifecycle || isRestart {
		return
	}

	n.Send(Message{
		Level: LevelInfo,
		Key:   "server_stopped",
		Title: "Server shutting down",
		Fields: map[string]string{
			"uptime": time.Since(lr.Started).Round(time.Second).String(),
			"pid":    fmt.Sprintf("%d", os.Getpid()),
		},
	})
}
