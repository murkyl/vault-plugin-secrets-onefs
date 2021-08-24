package onefss3

import (
	"testing"
)

func TestCalcMaxTTL(t *testing.T) {
	HelperCalcMaxTTL(t, -1, -1, -1)
	HelperCalcMaxTTL(t, 500, -1, 500)
	HelperCalcMaxTTL(t, 1000, 500, 500)
	HelperCalcMaxTTL(t, 80, 100, 80)
	HelperCalcMaxTTL(t, -1, 100, 100)
}

func TestCalcTTL(t *testing.T) {
	//                Req  Role   Cfg   Max Expected
	HelperCalcTTL(t, -1, -1, -1, -1, -1)
	HelperCalcTTL(t, 200, -1, -1, -1, 200)
	HelperCalcTTL(t, 200, -1, -1, 100, 100)
	HelperCalcTTL(t, 200, 150, -1, -1, 150)
	HelperCalcTTL(t, 200, -1, 120, -1, 120)
	HelperCalcTTL(t, -1, 400, 300, -1, 300)
	HelperCalcTTL(t, -1, 400, 400, -1, 400)
	HelperCalcTTL(t, -1, 400, 400, 280, 280)
	HelperCalcTTL(t, -1, -1, 400, 280, 280)
	HelperCalcTTL(t, -1, -1, 400, 580, 400)
}

func HelperCalcMaxTTL(t *testing.T, a int, b int, expected int) {
	x := CalcMaxTTL(a, b)
	if x != expected {
		t.Errorf("A: %d, B: %d, Expected: %d, Got: %d", a, b, expected, x)
	}
}

func HelperCalcTTL(t *testing.T, a int, b int, c int, d int, expected int) {
	x := CalcTTL(a, b, c, d)
	if x != expected {
		t.Errorf("Requested: %d, Role: %d, Cfg: %d, Max: %d, Expected: %d, Got: %d", a, b, c, d, expected, x)
	}
}
