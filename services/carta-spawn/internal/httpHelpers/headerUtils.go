package httpHelpers

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Timings map[string]time.Duration

func WriteTimings(w http.ResponseWriter, timings Timings) {
	timingEntries := make([]string, 0, len(timings))
	for k, v := range timings {
		timingEntries = append(timingEntries, fmt.Sprintf("%s;dur=%.2f", k, v.Seconds()*1000.0))
	}
	w.Header().Set("Server-Timing", strings.Join(timingEntries, ","))
}
