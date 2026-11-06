package domain

import (
	"testing"
	"time"
)

func TestPeriodsCycleAndWrap(t *testing.T) {
	order := []ReportPeriod{MonthToDate, QuarterToDate, YearToDate, AllTime, MonthToDate}
	for i := 1; i < len(order); i++ {
		if got := order[i-1].Next(); got != order[i] {
			t.Errorf("%v.Next() = %v, want %v", order[i-1], got, order[i])
		}
	}
}

func TestPeriodNames(t *testing.T) {
	want := map[ReportPeriod]string{
		MonthToDate: "month to date", QuarterToDate: "quarter to date", YearToDate: "year to date", AllTime: "all time",
	}
	for period, name := range want {
		if period.String() != name {
			t.Errorf("%d reads %q, want %q", period, period.String(), name)
		}
	}
}

func TestPeriodStart(t *testing.T) {
	date := func(s string) time.Time {
		d, _ := time.Parse(time.DateOnly, s)

		return d
	}
	for _, tc := range []struct {
		period ReportPeriod
		today  string
		want   string
		has    bool
	}{
		{MonthToDate, "2026-09-30", "2026-09-01", true},
		{QuarterToDate, "2026-09-30", "2026-07-01", true},
		{QuarterToDate, "2026-07-01", "2026-07-01", true},
		{QuarterToDate, "2026-02-28", "2026-01-01", true},
		{QuarterToDate, "2026-12-31", "2026-10-01", true},
		{YearToDate, "2026-09-30", "2026-01-01", true},
		{AllTime, "2026-09-30", "0001-01-01", false},
	} {
		got, has := tc.period.Start(date(tc.today))
		if has != tc.has || (has && !got.Equal(date(tc.want))) {
			t.Errorf("%v at %s starts %v/%v, want %s/%v", tc.period, tc.today, got, has, tc.want, tc.has)
		}
	}
}
