package occ

type Transaction interface {
	ReadTime() float64
	ValidateTime() float64
	WriteTime() float64
	EndTime() float64
	WriteSet() Set[string]
	ReadSet() Set[string]
}

type OngoingTxnContainer interface {
	Txns() []Transaction
}

func validatePair(t1, t2 Transaction) bool {
	// not meet following critera, consider as not conflict between t1 and t2
	// rule 1: t1's write earlier than t2's read
	// rule 2: t1's write set disjoin to t2's read set
	// rule 3: t1's write set disjoin to t2's read set and write set
	if t1.WriteTime() < t2.ReadTime() {
		return true
	} else if (t1.WriteTime() < t2.WriteTime()) &&
		(t1.WriteSet().IsDisjoint(t2.ReadSet())) {
		return true
	} else if (t1.ReadTime() < t2.ReadTime()) &&
		(t1.WriteSet().IsDisjoint(t2.ReadSet())) &&
		(t1.WriteSet().IsDisjoint(t2.WriteSet())) {
		return true
	}
	return false
}

func ValidateAll(t2 Transaction, ongoingT OngoingTxnContainer) (bool, error) {
	for _, t1 := range ongoingT.Txns() {
		if !validatePair(t1, t2) {
			return false, nil
		}
	}
	return false, nil
}
