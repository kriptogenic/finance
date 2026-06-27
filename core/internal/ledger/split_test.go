package ledger_test

import (
	"testing"

	"finance/internal/ledger"
	"finance/pkg/money"
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
		per, rem := ledger.EvenSplit(money.New(c.total, "UZS"), c.people)
		if per.Minor() != c.perPerson || rem.Minor() != c.remainder {
			t.Errorf("EvenSplit(%d, %d) = (%d, %d), want (%d, %d)",
				c.total, c.people, per.Minor(), rem.Minor(), c.perPerson, c.remainder)
		}
		// the parts must always re-sum to total
		if c.people > 0 && per.Minor()*int64(c.people)+rem.Minor() != c.total {
			t.Errorf("EvenSplit(%d, %d) parts do not re-sum to total", c.total, c.people)
		}
	}
}

func TestValidateSplit(t *testing.T) {
	uzs := func(v int64) money.Money { return money.New(v, "UZS") }
	ok := []ledger.SplitParticipant{{Name: "Alice", Amount: uzs(50)}, {Name: "Bob", Amount: uzs(50)}}
	if err := ledger.ValidateSplit(uzs(50), ok); err != nil {
		t.Errorf("valid split rejected: %v", err)
	}

	if err := ledger.ValidateSplit(uzs(50), nil); err != nil {
		t.Errorf("unsplit (no participants) rejected: %v", err)
	}

	if err := ledger.ValidateSplit(uzs(-1), ok); err == nil {
		t.Error("negative my_share accepted")
	}
	if err := ledger.ValidateSplit(uzs(0), ok); err == nil {
		t.Error("zero my_share accepted")
	}
	if err := ledger.ValidateSplit(uzs(50), []ledger.SplitParticipant{{Name: "", Amount: uzs(10)}}); err == nil {
		t.Error("empty participant name accepted")
	}
	if err := ledger.ValidateSplit(uzs(50), []ledger.SplitParticipant{{Name: "Bob", Amount: uzs(0)}}); err == nil {
		t.Error("zero participant share accepted")
	}
}
