package utils

import "time"

// StreamMaxLen is the maximum length of the Redis stream for device updates
type StreamMaxLenType int64

const StreamMaxLen StreamMaxLenType = 100

// DebounceWindow is the debounce window duration for processing device updates
const DebounceWindow = 2000 * time.Millisecond
