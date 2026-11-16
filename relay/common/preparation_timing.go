package common

import "time"

// RecordPreparationDuration accumulates local preparation work, including retry
// attempts. It never includes the upstream round trip and does not reset when an
// upstream attempt starts. Stage names are fixed by the callers, not user input.
func (info *RelayInfo) RecordPreparationDuration(stage string, elapsed time.Duration) {
	if info == nil || elapsed < 0 {
		return
	}
	info.upstreamTimingMu.Lock()
	defer info.upstreamTimingMu.Unlock()
	if info.preparationDurations == nil {
		info.preparationDurations = make(map[string]time.Duration)
	}
	info.preparationDurations[stage] += elapsed
}

func (info *RelayInfo) PreparationTimingMs() map[string]float64 {
	info.upstreamTimingMu.Lock()
	defer info.upstreamTimingMu.Unlock()
	result := make(map[string]float64, len(info.preparationDurations))
	for stage, elapsed := range info.preparationDurations {
		result[stage] = float64(elapsed.Microseconds()) / 1000
	}
	return result
}
