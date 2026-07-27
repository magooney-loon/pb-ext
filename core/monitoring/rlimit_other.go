//go:build !unix

package monitoring

// openFilesLimit reports 0 on platforms without RLIMIT_NOFILE.
//
// Zero means "unknown", and every consumer treats it as a reason to skip the
// descriptor-saturation check rather than to report 0%. Windows has no
// equivalent per-process descriptor ceiling to compare against.
func openFilesLimit() uint64 {
	return 0
}
