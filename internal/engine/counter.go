package engine

import (
	"math"
	"sync/atomic"
)

type TrafficCounter struct {
	total atomic.Int64
	peak  atomic.Int64 // 用 atomic.Int64 模拟 float64
}

func (s *TrafficCounter) Add(n int64) {
	if n > 0 {
		s.total.Add(n)
	}
}

func (s *TrafficCounter) Sub(n int64) {
	if n > 0 {
		s.total.Add(-n)
	}
}

func (s *TrafficCounter) Total() int64 { return s.total.Load() }

func (s *TrafficCounter) Peak() float64 {
	return math.Float64frombits(uint64(s.peak.Load()))
}

func (s *TrafficCounter) UpdatePeak(bps float64) {
	if bps < 0 || math.IsNaN(bps) {
		return
	}
	for {
		old := s.Peak()
		if bps <= old {
			return
		}
		if s.peak.CompareAndSwap(int64(math.Float64bits(old)), int64(math.Float64bits(bps))) {
			return
		}
	}
}
