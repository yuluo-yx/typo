package engine

import "unicode"

// KeyboardWeights defines the interface for keyboard layout-aware distance calculations.
type KeyboardWeights interface {
	IsAdjacent(a, b rune) bool
}

// QWERTYKeyboard implements KeyboardWeights for standard QWERTY layout.
type QWERTYKeyboard struct {
	adjacentKeys map[rune]map[rune]bool
}

// NewQWERTYKeyboard creates a new QWERTY keyboard layout.
func NewQWERTYKeyboard() *QWERTYKeyboard {
	kb := &QWERTYKeyboard{
		adjacentKeys: make(map[rune]map[rune]bool),
	}
	kb.initAdjacentKeys()
	return kb
}

// IsAdjacent checks if two keys are adjacent on the keyboard.
func (kb *QWERTYKeyboard) IsAdjacent(a, b rune) bool {
	a = unicode.ToLower(a)
	b = unicode.ToLower(b)

	if adj, ok := kb.adjacentKeys[a]; ok {
		return adj[b]
	}
	return false
}

func (kb *QWERTYKeyboard) initAdjacentKeys() {
	// QWERTY keyboard layout rows
	// Row 1: ` 1 2 3 4 5 6 7 8 9 0 - =
	// Row 2:   q w e r t y u i o p [ ] \
	// Row 3:    a s d f g h j k l ; '
	// Row 4:     z x c v b n m , . /

	adjacencyList := map[string]string{
		// Row 1
		"`": "1",
		"1": "`2",
		"2": "13",
		"3": "24",
		"4": "35",
		"5": "46",
		"6": "57",
		"7": "68",
		"8": "79",
		"9": "80",
		"0": "9-",
		"-": "0=",
		"=": "-",

		// Row 2
		"q":  "12wa",
		"w":  "qase3",
		"e":  "wd34r",
		"r":  "ef4t5",
		"t":  "ry56",
		"y":  "tu67",
		"u":  "yi78",
		"i":  "uo89",
		"o":  "ip90",
		"p":  "o[-0",
		"[":  "p]",
		"]":  "[\\",
		"\\": "]",

		// Row 3
		"a": "qwsz",
		"s": "awedxz",
		"d": "sferxc",
		"f": "dgrtcv",
		"g": "fhtyb",
		"h": "gjyun",
		"j": "hkuim",
		"k": "jli,",
		"l": "k;.",
		";": "l'",
		"'": ";",

		// Row 4
		"z": "asx",
		"x": "zsdc",
		"c": "xdfv",
		"v": "cfgb",
		"b": "vghn",
		"n": "bhjm",
		"m": "njk,",
		",": "mkl.",
		".": ",l/",
		"/": ".",
	}

	// Build bidirectional adjacency map
	for key, neighbors := range adjacencyList {
		keyRune := []rune(key)[0]
		if kb.adjacentKeys[keyRune] == nil {
			kb.adjacentKeys[keyRune] = make(map[rune]bool)
		}

		for _, neighborRune := range neighbors {
			kb.adjacentKeys[keyRune][neighborRune] = true

			// Add reverse mapping
			if kb.adjacentKeys[neighborRune] == nil {
				kb.adjacentKeys[neighborRune] = make(map[rune]bool)
			}
			kb.adjacentKeys[neighborRune][keyRune] = true
		}
	}
}

// DefaultKeyboard is the default keyboard weights instance.
var DefaultKeyboard KeyboardWeights = NewQWERTYKeyboard()
