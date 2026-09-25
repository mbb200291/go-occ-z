package occ

func validatePair(t1, t2 Transaction) bool {
	// When meet following critera, consider as not conflict (is valid) between t1 and t2
	if t1.GetWriteTime() < t2.GetReadTime() { // rule 1: t1's write earlier than t2's read
		return true
	} else if (t1.GetWriteTime() < t2.GetWriteTime()) && // rule 2: t1's write time earlier than g2's and t1's write set disjoin to t2's read set
		(t1.GetWriteSet().IsDisjoint(t2.GetReadSet())) {
		return true
	} else if (t1.GetReadTime() < t2.GetReadTime()) && // rule 3: t1's read time earlier than t2's and t1's write set disjoin to t2's read set and write set
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
