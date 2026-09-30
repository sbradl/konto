package domain

import "time"

// Transaction is one journal entry as hledger reports it.
type Transaction struct {
	Date        time.Time
	Description string
}
