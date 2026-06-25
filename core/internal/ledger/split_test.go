package ledger_test

import (
	"testing"

	"finance/internal/ledger"
)

func TestEvenSplit(t *testing.T) {
	cases := []struct {
		total     int64
		people    int
		perPerson int64
		remainder int64
	}{
		{200, 4, 50, 0},
		{20, 3, 6, 2},
		{100, 1, 100, 0},
		{99, 0, 0, 99},
	}

	for _, c := range cases {
		per, rem := ledger.EvenSplit(c.total, c.people)
		if per != c.perPerson || rem != c.remainder {
			t.Errorf("EvenSplit(%d, %d) = (%d, %d), want (%d, %d)",
				c.total, c.people, per, rem, c.perPerson, c.remainder)
		}
		// the parts must always re-sum to total
		if c.people > 0 && per*int64(c.people)+rem != c.total {
			t.Errorf("EvenSplit(%d, %d) parts do not re-sum to total", c.total, c.people)
		}
	}
}

func TestValidateSplit(t *testing.T) {
	ok := []ledger.SplitParticipant{{Name: "Alice", Amount: 50}, {Name: "Bob", Amount: 50}}
	if err := ledger.ValidateSplit(50, ok); err != nil {
		t.Errorf("valid split rejected: %v", err)
	}

	if err := ledger.ValidateSplit(50, nil); err != nil {
		t.Errorf("unsplit (no participants) rejected: %v", err)
	}

	if err := ledger.ValidateSplit(-1, ok); err == nil {
		t.Error("negative my_share accepted")
	}
	if err := ledger.ValidateSplit(0, ok); err == nil {
		t.Error("zero my_share accepted")
	}
	if err := ledger.ValidateSplit(50, []ledger.SplitParticipant{{Name: "", Amount: 10}}); err == nil {
		t.Error("empty participant name accepted")
	}
	if err := ledger.ValidateSplit(50, []ledger.SplitParticipant{{Name: "Bob", Amount: 0}}); err == nil {
		t.Error("zero participant share accepted")
	}
}
