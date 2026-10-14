package occ

func validatePair(prevTxn, curTxn Transaction) bool {
	// When meet following critera, consider as not conflict (is valid) between prevTxn and curTxn
	if prevTxn.GetWriteTime() < curTxn.GetReadTime() { // rule 1: prevTxn's write earlier than curTxn's read
		return true
	} else if (prevTxn.GetWriteTime() < curTxn.GetWriteTime()) && // rule 2: prevTxn's write time earlier than g2's and prevTxn's write set disjoin to curTxn's read set
		(prevTxn.GetWriteSet().IsDisjoint(curTxn.GetReadSet())) {
		return true
	} else if (prevTxn.GetWriteSet().IsDisjoint(curTxn.GetReadSet())) && // rule 3: prevTxn complete read phase earlier than curTxn's (i use validate time order to acheive this rule) and prevTxn's write set disjoin to curTxn's read set and write set
		(prevTxn.GetWriteSet().IsDisjoint(curTxn.GetWriteSet())) {
		return true
	}
	return false
}

func Validate(curTxn Transaction, prevTxns TransactionContainer, minReadTimeOgTxns uint64) bool {
	outcome := true

	// validate
	prevTxns.IterTill(
		curTxn.GetValidateTime(),
		func(prevKey uint64, prevTxn Transaction) bool {
			if prevKey == curTxn.GetValidateTime() {
				return true
			}
			outcome = outcome && validatePair(prevTxn, curTxn)
			if !outcome {
				return false
			}
			// remove txn which write time early than onging txn's read time
			if prevTxn.GetWriteTime() < minReadTimeOgTxns {
				prevTxns.Remove(prevTxn)
			}
			return true
		},
	)

	return outcome
}

// func Validate(curTxn Transaction, prevTxns TransactionContainer) bool {
// 	for _, prevTxn := range prevTxns.GetTxns() {
// 		if !validatePair(prevTxn, curTxn) {
// 			return false
// 		}
// 	}
// 	return false
// }
