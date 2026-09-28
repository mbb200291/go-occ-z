package occ

func validatePair(prevTxn, curTxn Transaction) bool {
	// When meet following critera, consider as not conflict (is valid) between prevTxn and curTxn
	if prevTxn.GetWriteTime() < curTxn.GetReadTime() { // rule 1: prevTxn's write earlier than curTxn's read
		return true
	} else if (prevTxn.GetWriteTime() < curTxn.GetWriteTime()) && // rule 2: prevTxn's write time earlier than g2's and prevTxn's write set disjoin to curTxn's read set
		(prevTxn.GetWriteSet().IsDisjoint(curTxn.GetReadSet())) {
		return true
	} else if (prevTxn.GetReadTime() < curTxn.GetReadTime()) && // rule 3: prevTxn's read time earlier than curTxn's and prevTxn's write set disjoin to curTxn's read set and write set
		(prevTxn.GetWriteSet().IsDisjoint(curTxn.GetReadSet())) &&
		(prevTxn.GetWriteSet().IsDisjoint(curTxn.GetWriteSet())) {
		return true
	}
	return false
}

func Validate(curTxn Transaction, prevTxns TransactionContainer) bool {
	outcome := true
	prevTxns.IterTill(
		curTxn.GetValidateTime(),
		func(curTxn Transaction) {
			outcome = outcome && validatePair(prevTxn, curTxn)
		},
	)
}

// func Validate(curTxn Transaction, prevTxns TransactionContainer) bool {
// 	for _, prevTxn := range prevTxns.GetTxns() {
// 		if !validatePair(prevTxn, curTxn) {
// 			return false
// 		}
// 	}
// 	return false
// }
