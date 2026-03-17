package service

import (
	"fmt"
	"testing"
)

func TestBuildScheduleTransitionAnalysisUsesDeltaBuffer(t *testing.T) {
	originalLookup := integrationTravelDurationLookupFn
	originalThresholdProvider := scheduleTravelThresholdsProviderFn
	defer func() {
		integrationTravelDurationLookupFn = originalLookup
		scheduleTravelThresholdsProviderFn = originalThresholdProvider
	}()

	scheduleTravelThresholdsProviderFn = func() (ScheduleTravelThresholds, error) {
		return ScheduleTravelThresholds{
			EasyThresholdMinutes:      20,
			DifficultThresholdMinutes: 45,
		}, nil
	}

	tests := []struct {
		name           string
		nextAt         string
		durationSecond int
		expected       string
	}{
		{
			name:           "easy when buffer is large",
			nextAt:         "2026-03-07T10:40:00Z",
			durationSecond: 7 * 60,
			expected:       scheduleTravelDifficultyEasy,
		},
		{
			name:           "moderate when buffer is tight",
			nextAt:         "2026-03-07T10:25:00Z",
			durationSecond: 7 * 60,
			expected:       scheduleTravelDifficultyModerate,
		},
		{
			name:           "difficult when schedule is impossible",
			nextAt:         "2026-03-07T10:05:00Z",
			durationSecond: 7 * 60,
			expected:       scheduleTravelDifficultyDifficult,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			integrationTravelDurationLookupFn = func(float64, float64, float64, float64) (int, error) {
				return testCase.durationSecond, nil
			}
			currentAt := "2026-03-07T10:00:00Z"
			points := []ScheduleTravelPoint{
				{
					ScheduleID: 1,
					Latitude:   floatPtr(22.3027),
					Longitude:  floatPtr(114.1772),
					SelectedAt: &currentAt,
				},
				{
					ScheduleID: 2,
					Latitude:   floatPtr(22.3193),
					Longitude:  floatPtr(114.1694),
					SelectedAt: &testCase.nextAt,
				},
			}

			analysis := BuildScheduleTransitionAnalysis(points)
			if len(analysis) != 1 {
				t.Fatalf("expected one transition, got %d", len(analysis))
			}
			if analysis[0].Difficulty != testCase.expected {
				t.Fatalf("expected difficulty %s, got %s", testCase.expected, analysis[0].Difficulty)
			}
			if analysis[0].DeltaSecond == nil {
				t.Fatalf("expected delta_seconds to be set")
			}
		})
	}
}

func TestBuildScheduleTransitionAnalysisUnavailableOnLookupFailure(t *testing.T) {
	originalLookup := integrationTravelDurationLookupFn
	originalThresholdProvider := scheduleTravelThresholdsProviderFn
	defer func() {
		integrationTravelDurationLookupFn = originalLookup
		scheduleTravelThresholdsProviderFn = originalThresholdProvider
	}()

	scheduleTravelThresholdsProviderFn = func() (ScheduleTravelThresholds, error) {
		return ScheduleTravelThresholds{
			EasyThresholdMinutes:      20,
			DifficultThresholdMinutes: 45,
		}, nil
	}
	integrationTravelDurationLookupFn = func(float64, float64, float64, float64) (int, error) {
		return 0, fmt.Errorf("forced failure")
	}

	currentAt := "2026-03-07T10:00:00Z"
	nextAt := "2026-03-07T10:40:00Z"
	points := []ScheduleTravelPoint{
		{
			ScheduleID: 1,
			Latitude:   floatPtr(22.3027),
			Longitude:  floatPtr(114.1772),
			SelectedAt: &currentAt,
		},
		{
			ScheduleID: 2,
			Latitude:   floatPtr(22.3193),
			Longitude:  floatPtr(114.1694),
			SelectedAt: &nextAt,
		},
	}

	analysis := BuildScheduleTransitionAnalysis(points)
	if len(analysis) != 1 {
		t.Fatalf("expected one transition, got %d", len(analysis))
	}
	if analysis[0].Status != scheduleTravelStatusUnavailable {
		t.Fatalf("expected unavailable status, got %s", analysis[0].Status)
	}
	if analysis[0].Difficulty != scheduleTravelDifficultyUnavailable {
		t.Fatalf("expected unavailable difficulty, got %s", analysis[0].Difficulty)
	}
	if analysis[0].ScheduledGap == nil {
		t.Fatalf("expected scheduled_gap_seconds to be set even when travel lookup fails")
	}
	if *analysis[0].ScheduledGap != 40*60 {
		t.Fatalf("expected scheduled gap 2400, got %d", *analysis[0].ScheduledGap)
	}
}

func TestScheduleGapSecondsSupportsLegacyTimestampFormat(t *testing.T) {
	origin := "2026-03-13 17:08:00+00"
	destination := "2026-03-13 23:00:00+00"
	gap, ok := scheduleGapSeconds(&origin, &destination)
	if !ok {
		t.Fatalf("expected legacy format timestamps to be parsed")
	}
	expected := 5 * 60 * 60
	expected += 52 * 60
	if gap != expected {
		t.Fatalf("expected gap %d, got %d", expected, gap)
	}
}

func TestScheduleGapSecondsSupportsLocalDateTimeFormat(t *testing.T) {
	origin := "2026-03-16T23:00:00"
	destination := "2026-03-17T01:30:00"
	gap, ok := scheduleGapSeconds(&origin, &destination)
	if !ok {
		t.Fatalf("expected local datetime timestamps to be parsed")
	}
	expected := 2 * 60 * 60
	expected += 30 * 60
	if gap != expected {
		t.Fatalf("expected gap %d, got %d", expected, gap)
	}
}

func floatPtr(value float64) *float64 {
	return &value
}
