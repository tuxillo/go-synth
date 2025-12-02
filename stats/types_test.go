package stats

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "zero duration",
			duration: 0,
			want:     "00:00:00",
		},
		{
			name:     "1 second",
			duration: time.Second,
			want:     "00:00:01",
		},
		{
			name:     "1 minute",
			duration: time.Minute,
			want:     "00:01:00",
		},
		{
			name:     "1 hour",
			duration: time.Hour,
			want:     "01:00:00",
		},
		{
			name:     "complex duration",
			duration: 1*time.Hour + 23*time.Minute + 45*time.Second,
			want:     "01:23:45",
		},
		{
			name:     "very long duration",
			duration: 12*time.Hour + 5*time.Minute + 3*time.Second,
			want:     "12:05:03",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestFormatRate(t *testing.T) {
	tests := []struct {
		name string
		rate float64
		want string
	}{
		{
			name: "zero rate",
			rate: 0.0,
			want: "0.0",
		},
		{
			name: "very small rate",
			rate: 0.05,
			want: "0.0", // Below 0.1 threshold
		},
		{
			name: "small rate",
			rate: 0.5,
			want: "0.5",
		},
		{
			name: "normal rate",
			rate: 24.3,
			want: "24.3",
		},
		{
			name: "high rate",
			rate: 120.7,
			want: "120.7",
		},
		{
			name: "rate with many decimals",
			rate: 45.6789,
			want: "45.7", // Should round to 1 decimal
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatRate(tt.rate)
			if got != tt.want {
				t.Errorf("FormatRate(%v) = %q, want %q", tt.rate, got, tt.want)
			}
		})
	}
}

func TestThrottleReason(t *testing.T) {
	tests := []struct {
		name string
		info TopInfo
		want string
	}{
		{
			name: "not throttled",
			info: TopInfo{
				MaxWorkers:    8,
				DynMaxWorkers: 8,
				Load:          2.0,
				SwapPct:       0,
			},
			want: "",
		},
		{
			name: "throttled by high load",
			info: TopInfo{
				MaxWorkers:    8,
				DynMaxWorkers: 6,
				Load:          20.0, // Much higher than 2×8
				SwapPct:       0,
			},
			want: "high load",
		},
		{
			name: "throttled by high swap",
			info: TopInfo{
				MaxWorkers:    8,
				DynMaxWorkers: 6,
				Load:          2.0,
				SwapPct:       15, // Above 10% threshold
			},
			want: "high swap",
		},
		{
			name: "throttled by both (load takes precedence)",
			info: TopInfo{
				MaxWorkers:    8,
				DynMaxWorkers: 4,
				Load:          25.0,
				SwapPct:       20,
			},
			want: "high load",
		},
		{
			name: "throttled for unknown reason",
			info: TopInfo{
				MaxWorkers:    8,
				DynMaxWorkers: 6,
				Load:          4.0, // Not high enough
				SwapPct:       5,   // Not high enough
			},
			want: "system resources",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ThrottleReason(tt.info)
			if got != tt.want {
				t.Errorf("ThrottleReason() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildStatusString(t *testing.T) {
	tests := []struct {
		name   string
		status BuildStatus
		want   string
	}{
		{
			name:   "success",
			status: BuildSuccess,
			want:   "success",
		},
		{
			name:   "failed",
			status: BuildFailed,
			want:   "failed",
		},
		{
			name:   "ignored",
			status: BuildIgnored,
			want:   "ignored",
		},
		{
			name:   "skipped",
			status: BuildSkipped,
			want:   "skipped",
		},
		{
			name:   "unknown",
			status: BuildStatus(999),
			want:   "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.String()
			if got != tt.want {
				t.Errorf("BuildStatus.String() = %q, want %q", got, tt.want)
			}
		})
	}
}
