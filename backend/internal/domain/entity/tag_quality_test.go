package entity

import "testing"

func TestIsTagGeneric(t *testing.T) {
	tests := []struct {
		tag      string
		expected bool
	}{
		{"documento", true},
		{"información", true},
		{"nota", true},
		{"notas-reunion", false},
		{"facturas-2024", false},
		{"reporte-financiero", false},
	}

	for _, test := range tests {
		if IsTagGeneric(test.tag) != test.expected {
			t.Fatalf("IsTagGeneric(%q) expected %v", test.tag, test.expected)
		}
	}
}

func TestAreTagsSimilar(t *testing.T) {
	tests := []struct {
		a        string
		b        string
		expected bool
	}{
		{"factura", "facturas", true},
		{"reunion-equipo", "reunión-equipo", true},
		{"nota-proyecto", "notas-proyecto", true},
		{"resumen", "analisis", false},
	}

	for _, test := range tests {
		if AreTagsSimilar(test.a, test.b) != test.expected {
			t.Fatalf("AreTagsSimilar(%q, %q) expected %v", test.a, test.b, test.expected)
		}
	}
}
