// Package core: role-based capability rules.
//
// Roles are strict and non-overlapping by design:
//   - cashier: sells; the only role mirrored on the customer display.
//   - manager: sells and corrects (voids, manual discounts, adjustments).
//     Every corrective action lands in the audit log.
//   - owner: workers, settings, backups, sync - and the only role that
//     can read the audit trail. Owners do not sell.
package core

const (
	CapSell       = "sell"       // take orders, record payments
	CapCorrect    = "correct"    // voids, manual discounts, adjustments
	CapAdminister = "administer" // workers, settings, backup, audit trail
	CapMirror     = "mirror"     // can be mirrored on the customer display
)

var roleCaps = map[string][]string{
	"cashier": {CapSell, CapMirror},
	"manager": {CapSell, CapCorrect, CapAdminister},
	"owner":   {CapAdminister},
}

func Allowed(role, cap string) bool {
	for _, c := range roleCaps[role] {
		if c == cap {
			return true
		}
	}
	return false
}
