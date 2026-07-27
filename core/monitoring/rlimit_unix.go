//go:build unix

package monitoring

import "syscall"

// openFilesLimit returns the soft RLIMIT_NOFILE for this process, or 0 when
// there is no usable ceiling to measure against.
//
// The raw open-descriptor count is close to useless on its own: the soft limit
// is 1024 on some hosts and 1048576 on others, so "512 open files" is either
// half a step from an outage or utterly unremarkable, and a fixed threshold
// would be wrong on most machines. The ratio against this limit is the figure
// worth alerting on.
func openFilesLimit() uint64 {
	var lim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
		return 0
	}

	// Rlimit.Cur is uint64 on Linux, Darwin, NetBSD and OpenBSD but int64 on
	// FreeBSD, so the conversion is needed to compile everywhere. It is also why
	// there is no "is it negative" check: that comparison is a tautology on four
	// of the five, which is what gopls flags, and normalizeFDLimit catches a
	// FreeBSD negative anyway — it converts to an implausibly large unsigned
	// value and gets rejected on size.
	return normalizeFDLimit(uint64(lim.Cur))
}
