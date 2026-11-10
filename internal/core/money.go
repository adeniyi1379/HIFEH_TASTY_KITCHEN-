// Package core holds pure business rules: money, pricing and VAT.
// No I/O lives here; everything is integers (kobo, spoons, pieces).
package core

// LinePrice returns quantity x unit price, all in kobo.
func LinePrice(quantity, unitPriceKobo int64) int64 {
	return quantity * unitPriceKobo
}

// VATSettings is the global VAT configuration controlled by the manager.
type VATSettings struct {
	Enabled bool
	RateBP  int64 // basis points, e.g. 750 = 7.5%
	Mode    string
}

const (
	VATModeInclusive = "inclusive"
	VATModeExclusive = "added_on_top"
)

// divHalfUp divides x by d rounding half up (d > 0). Single rounding step.
func divHalfUp(x, d int64) int64 {
	if x >= 0 {
		return (x + d/2) / d
	}
	return -((-x + d/2) / d)
}

// ApplyVAT takes the discounted net subtotal (before VAT) and returns
// (net, vat, total). In inclusive mode the subtotal is treated as gross
// and VAT is extracted; in exclusive mode VAT is added on top.
func ApplyVAT(subtotalKobo int64, v VATSettings) (net, vat, total int64) {
	if !v.Enabled || v.RateBP <= 0 {
		return subtotalKobo, 0, subtotalKobo
	}
	if v.Mode == VATModeInclusive {
		net = divHalfUp(subtotalKobo*10000, 10000+v.RateBP)
		vat = subtotalKobo - net
		return net, vat, subtotalKobo
	}
	vat = divHalfUp(subtotalKobo*v.RateBP, 10000)
	return subtotalKobo, vat, subtotalKobo + vat
}
