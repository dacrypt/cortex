package entity

import "strings"

const similarityThreshold = 0.67

// AreTagsSimilar returns true when tags are likely variants of the same concept.
func AreTagsSimilar(a, b string) bool {
	normalizedA := strings.ToLower(strings.TrimSpace(a))
	normalizedB := strings.ToLower(strings.TrimSpace(b))
	if normalizedA == "" || normalizedB == "" {
		return false
	}
	if normalizedA == normalizedB {
		return true
	}
	if strings.HasPrefix(normalizedA+"-", normalizedB) || strings.HasPrefix(normalizedB+"-", normalizedA) {
		return true
	}

	tokensA := tagTokens(normalizedA)
	tokensB := tagTokens(normalizedB)
	if len(tokensA) == 0 || len(tokensB) == 0 {
		return false
	}
	if len(tokensA) == 1 && len(tokensB) == 1 {
		return tokensA[0] == tokensB[0]
	}

	setA := make(map[string]struct{}, len(tokensA))
	for _, t := range tokensA {
		setA[t] = struct{}{}
	}
	setB := make(map[string]struct{}, len(tokensB))
	for _, t := range tokensB {
		setB[t] = struct{}{}
	}

	intersection := 0
	for t := range setA {
		if _, ok := setB[t]; ok {
			intersection++
		}
	}
	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return false
	}
	score := float64(intersection) / float64(union)
	return score >= similarityThreshold
}
