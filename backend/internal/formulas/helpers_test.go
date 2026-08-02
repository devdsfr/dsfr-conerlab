package formulas

import (
	"math"
	"testing"
)

// tolerância de 4 casas decimais — critério do Formula Catalog.
const eps = 1e-4

func approx(t *testing.T, got, want float64, label string) {
	t.Helper()
	if math.Abs(got-want) > eps {
		t.Errorf("%s: got %.6f, want %.6f (±%.0e)", label, got, want, eps)
	}
}

func wantErr(t *testing.T, err, target error, label string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: esperava erro %v, veio nil", label, target)
	}
}

func noErr(t *testing.T, err error, label string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: erro inesperado: %v", label, err)
	}
}
