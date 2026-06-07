package keys

import "strings"

// layoutToLatin maps characters produced by non-Latin keyboard layouts back to
// the Latin (QWERTY) key on the same physical position, so bindings defined with
// Latin keys also fire under those layouts. Terminals report the produced
// character (e.g. "к"), not the physical key, so translation must be per-layout.
//
// Only the Russian (ЙЦУКЕН) layout is mapped today; add another layout's table
// here to support it. Lower- and upper-case are both covered (Shift combos).
var layoutToLatin = map[rune]rune{
	// Russian ЙЦУКЕН → QWERTY, lower case.
	'й': 'q', 'ц': 'w', 'у': 'e', 'к': 'r', 'е': 't', 'н': 'y', 'г': 'u',
	'ш': 'i', 'щ': 'o', 'з': 'p', 'х': '[', 'ъ': ']',
	'ф': 'a', 'ы': 's', 'в': 'd', 'а': 'f', 'п': 'g', 'р': 'h', 'о': 'j',
	'л': 'k', 'д': 'l', 'ж': ';', 'э': '\'',
	'я': 'z', 'ч': 'x', 'с': 'c', 'м': 'v', 'и': 'b', 'т': 'n', 'ь': 'm',
	'б': ',', 'ю': '.', 'ё': '`',
	// Russian ЙЦУКЕН → QWERTY, upper case (Shift).
	'Й': 'Q', 'Ц': 'W', 'У': 'E', 'К': 'R', 'Е': 'T', 'Н': 'Y', 'Г': 'U',
	'Ш': 'I', 'Щ': 'O', 'З': 'P', 'Х': '{', 'Ъ': '}',
	'Ф': 'A', 'Ы': 'S', 'В': 'D', 'А': 'F', 'П': 'G', 'Р': 'H', 'О': 'J',
	'Л': 'K', 'Д': 'L', 'Ж': ':', 'Э': '"',
	'Я': 'Z', 'Ч': 'X', 'С': 'C', 'М': 'V', 'И': 'B', 'Т': 'N', 'Ь': 'M',
	'Б': '<', 'Ю': '>', 'Ё': '~',
}

// normalizeKey translates a single key token typed on a supported non-Latin
// layout to the equivalent Latin key on the same physical position. A modifier
// prefix (e.g. "ctrl+") is preserved; only the final key rune is translated.
// Named keys ("enter", "esc", "up") and already-Latin keys pass through.
func normalizeKey(token string) string {
	if token == "" {
		return token
	}
	prefix, last := "", token
	if i := strings.LastIndex(token, "+"); i >= 0 {
		prefix, last = token[:i+1], token[i+1:]
	}
	r := []rune(last)
	if len(r) != 1 {
		return token // named key like "enter" / "esc"
	}
	if latin, ok := layoutToLatin[r[0]]; ok {
		return prefix + string(latin)
	}
	return token
}
