package entities_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"finance/internal/entities"
)

type ScheduledTransactionSuite struct {
	suite.Suite
}

func TestScheduledTransactionSuite(t *testing.T) {
	suite.Run(t, new(ScheduledTransactionSuite))
}

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func (s *ScheduledTransactionSuite) TestNextRun_StepsByFrequency() {
	from := date(2026, 6, 20)

	s.Equal(date(2026, 6, 21), entities.NextRun(entities.FreqDaily, 1, from))
	s.Equal(date(2026, 6, 27), entities.NextRun(entities.FreqWeekly, 1, from))
	s.Equal(date(2026, 7, 20), entities.NextRun(entities.FreqMonthly, 1, from))
	s.Equal(date(2027, 6, 20), entities.NextRun(entities.FreqYearly, 1, from))
}

func (s *ScheduledTransactionSuite) TestNextRun_HonorsInterval() {
	from := date(2026, 6, 20)

	s.Equal(date(2026, 6, 22), entities.NextRun(entities.FreqDaily, 2, from))
	s.Equal(date(2026, 7, 4), entities.NextRun(entities.FreqWeekly, 2, from))
	s.Equal(date(2026, 9, 20), entities.NextRun(entities.FreqMonthly, 3, from))
}

func (s *ScheduledTransactionSuite) TestNextRun_MonthEndRollsOver() {
	// Jan 31 + 1 month → Go normalizes to Mar 3 (non-leap), never an invalid date.
	s.Equal(date(2027, 3, 3), entities.NextRun(entities.FreqMonthly, 1, date(2027, 1, 31)))
}

func (s *ScheduledTransactionSuite) TestNextRun_ClampsInterval() {
	from := date(2026, 6, 20)
	s.Equal(entities.NextRun(entities.FreqDaily, 1, from), entities.NextRun(entities.FreqDaily, 0, from))
}
