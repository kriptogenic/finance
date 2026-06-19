package telegram

import (
	"testing"
	"time"
)

func TestNextRun(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Tashkent")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "before 8am schedules today",
			now:  time.Date(2026, 6, 19, 7, 59, 0, 0, loc),
			want: time.Date(2026, 6, 19, 8, 0, 0, 0, loc),
		},
		{
			name: "after 8am schedules tomorrow",
			now:  time.Date(2026, 6, 19, 8, 1, 0, 0, loc),
			want: time.Date(2026, 6, 20, 8, 0, 0, 0, loc),
		},
		{
			name: "exactly 8am rolls to tomorrow",
			now:  time.Date(2026, 6, 19, 8, 0, 0, 0, loc),
			want: time.Date(2026, 6, 20, 8, 0, 0, 0, loc),
		},
		{
			name: "crosses month boundary",
			now:  time.Date(2026, 6, 30, 9, 0, 0, 0, loc),
			want: time.Date(2026, 7, 1, 8, 0, 0, 0, loc),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := nextRun(tc.now, scheduleHour, scheduleMinute)
			if !got.Equal(tc.want) {
				t.Errorf("nextRun(%s) = %s, want %s", tc.now, got, tc.want)
			}
		})
	}
}
