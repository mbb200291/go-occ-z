package occ

func validatePair(t1, t2 Transaction) bool {
	// not meet following critera, consider as not conflict between t1 and t2
	// rule 1: t1's write earlier than t2's read
	// rule 2: t1's write set disjoin to t2's read set
	// rule 3: t1's write set disjoin to t2's read set and write set
	if t1.GetWriteTime() < t2.GetReadTime() {
		return true
	} else if (t1.GetWriteTime() < t2.GetWriteTime()) &&
		(t1.GetWriteSet().IsDisjoint(t2.GetReadSet())) {
		return true
	} else if (t1.GetReadTime() < t2.GetReadTime()) &&
		(t1.GetWriteSet().IsDisjoint(t2.GetReadSet())) &&
		(t1.GetWriteSet().IsDisjoint(t2.GetWriteSet())) {
		return true
	}
	return false
}

func Validate(t2 Transaction, prevTxns TxnContainer) bool {
	for _, t1 := range prevTxns.GetTxns() {
		if !validatePair(t1, t2) {
			return false
		}
	}
	return false
}
