package collector

import (
	"strings"
	"testing"
)

func TestParseUnboundStats(t *testing.T) {
	output := strings.Join([]string{
		"total.num.queries=100",
		"total.num.cachehits=60",
		"total.requestlist.avg=1.5",
		"msg.cache.count=25",
		"unknown.stat=10",
		"total.num.cachemiss=not-a-number",
		"malformed line",
	}, "\n")

	stats := parseUnboundStats([]byte(output))
	if len(stats) != 5 {
		t.Fatalf("expected 5 parsed stats, got %d", len(stats))
	}
	for key, want := range map[string]float64{
		"total.num.queries":     100,
		"total.num.cachehits":   60,
		"total.requestlist.avg": 1.5,
		"msg.cache.count":       25,
		"unknown.stat":          10,
	} {
		if got, ok := stats[key]; !ok || got != want {
			t.Errorf("stats[%q] = %v, %t; want %v, true", key, got, ok, want)
		}
	}
	if _, ok := stats["total.num.cachemiss"]; ok {
		t.Fatal("invalid numeric value should not be parsed")
	}
}
