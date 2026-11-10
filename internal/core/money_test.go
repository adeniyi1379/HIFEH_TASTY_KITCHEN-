package core

import "testing"

func TestLinePrice(t *testing.T) {
	cases := []struct {
		qty, price, want int64
	}{
		{3, 50000, 150000},   // 3 spoons of jollof at ₦500
		{5, 10000, 50000},    // 5 pieces of dodo at ₦100
		{0, 50000, 0},
	}
	for _, c := range cases {
		if got := LinePrice(c.qty, c.price); got != c.want {
			t.Errorf("LinePrice(%d,%d)=%d want %d", c.qty, c.price, got, c.want)
		}
	}
}

func TestApplyVATDisabled(t *testing.T) {
	net, vat, total := ApplyVAT(100000, VATSettings{})
	if net != 100000 || vat != 0 || total != 100000 {
		t.Errorf("got %d/%d/%d", net, vat, total)
	}
}

func TestApplyVATExclusive(t *testing.T) {
	// 7.5% on ₦1000 -> vat ₦75, total ₦1075
	net, vat, total := ApplyVAT(100000, VATSettings{Enabled: true, RateBP: 750, Mode: VATModeExclusive})
	if net != 100000 || vat != 7500 || total != 107500 {
		t.Errorf("got %d/%d/%d", net, vat, total)
	}
}

func TestApplyVATInclusive(t *testing.T) {
	// ₦1075 gross at 7.5% -> net ₦1000, vat ₦75
	net, vat, total := ApplyVAT(107500, VATSettings{Enabled: true, RateBP: 750, Mode: VATModeInclusive})
	if net != 100000 || vat != 7500 || total != 107500 {
		t.Errorf("got %d/%d/%d", net, vat, total)
	}
}

func TestApplyVATInclusiveRoundsOnce(t *testing.T) {
	// ₦1000 gross at 7.5% -> net = round(100000*10000/10750) = 93023.25 -> 93023, vat = 6977
	net, vat, total := ApplyVAT(100000, VATSettings{Enabled: true, RateBP: 750, Mode: VATModeInclusive})
	if net != 93023 || vat != 6977 || total != 100000 {
		t.Errorf("got %d/%d/%d", net, vat, total)
	}
}
