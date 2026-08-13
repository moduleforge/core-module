package fieldcrypto

import "time"

// SetReloadTuningForTest overrides the reload timings a constructor installed,
// so a test can exercise the TTL and rate-limit paths without waiting out
// production-sized intervals.
//
// Both fields are plain, written once at construction and read-only
// thereafter. Call this immediately after constructing a Cipher and before
// sharing it with any other goroutine.
func SetReloadTuningForTest(c *Cipher, keySetTTL, minReloadInterval time.Duration) {
	c.keySetTTL = keySetTTL
	c.minReloadInterval = minReloadInterval
}
