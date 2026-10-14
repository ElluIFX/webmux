//go:build darwin

package session

// macOS has no subreaper: launchd adopts and reaps orphaned descendants, while
// cleanup still finds them through the session ID and environment marker.
func enableReaper() error { return nil }
