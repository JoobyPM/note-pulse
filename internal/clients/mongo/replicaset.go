package mongo

import "sync/atomic"

// 0 = stand-alone, 1 = replica set
var isReplicaSet atomic.Bool

// IsReplicaSet reports whether the current deployment is a replica set.
// Callers MUST treat the result as a hint (cached & eventually consistent).
func IsReplicaSet() bool { return isReplicaSet.Load() }
