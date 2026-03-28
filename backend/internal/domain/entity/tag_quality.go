package entity

import "strings"

var genericTagStopwords = map[string]struct{}{
	"archivo":     {},
	"archivos":    {},
	"documento":   {},
	"documentos":  {},
	"nota":        {},
	"notas":       {},
	"resumen":     {},
	"borrador":    {},
	"general":     {},
	"otros":       {},
	"varios":      {},
	"misc":        {},
	"info":        {},
	"informacion": {},
	"datos":       {},
	"reporte":     {},
	"reportes":    {},
	"lista":       {},
	"listas":      {},
}

// IsTagGeneric returns true when the tag is too generic to be useful.
func IsTagGeneric(tag string) bool {
	normalized := strings.Trim(strings.ToLower(tag), "- ")
	if normalized == "" {
		return true
	}
	normalized = foldDiacritics(normalized)
	if _, ok := genericTagStopwords[normalized]; ok {
		return true
	}

	tokens := tagTokens(normalized)
	if len(tokens) == 0 {
		return true
	}

	allGeneric := true
	for _, token := range tokens {
		if _, ok := genericTagStopwords[token]; !ok {
			allGeneric = false
			break
		}
	}
	return allGeneric
}
