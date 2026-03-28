package entity

import "strings"

func tagTokens(tag string) []string {
	parts := strings.Split(strings.Trim(tag, "-"), "-")
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		token := normalizeToken(part)
		if token != "" {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func normalizeToken(token string) string {
	trimmed := strings.TrimSpace(strings.ToLower(token))
	if trimmed == "" {
		return ""
	}
	trimmed = foldDiacritics(trimmed)
	if len(trimmed) > 4 && strings.HasSuffix(trimmed, "es") {
		return strings.TrimSuffix(trimmed, "es")
	}
	if len(trimmed) > 3 && strings.HasSuffix(trimmed, "s") {
		return strings.TrimSuffix(trimmed, "s")
	}
	return trimmed
}

func foldDiacritics(input string) string {
	replacer := strings.NewReplacer(
		"á", "a", "à", "a", "ä", "a", "â", "a", "ã", "a",
		"é", "e", "è", "e", "ë", "e", "ê", "e",
		"í", "i", "ì", "i", "ï", "i", "î", "i",
		"ó", "o", "ò", "o", "ö", "o", "ô", "o", "õ", "o",
		"ú", "u", "ù", "u", "ü", "u", "û", "u",
		"ñ", "n",
	)
	return replacer.Replace(input)
}
