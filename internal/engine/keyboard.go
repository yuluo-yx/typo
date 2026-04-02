package engine

import (
	"fmt"
	"strings"
	"unicode"
)

// KeyboardWeights defines adjacency checks for a keyboard layout.
type KeyboardWeights interface {
	IsAdjacent(a, b rune) bool
}

type adjacencyKeyboard struct {
	adjacentKeys map[rune]map[rune]bool
}

// IsAdjacent reports whether two characters are adjacent on the current layout.
func (kb *adjacencyKeyboard) IsAdjacent(a, b rune) bool {
	a = unicode.ToLower(a)
	b = unicode.ToLower(b)

	if adj, ok := kb.adjacentKeys[a]; ok {
		return adj[b]
	}
	return false
}

// QWERTYKeyboard represents the standard QWERTY layout.
type QWERTYKeyboard struct {
	adjacencyKeyboard
}

// DvorakKeyboard represents the Dvorak layout.
type DvorakKeyboard struct {
	adjacencyKeyboard
}

// ColemakKeyboard represents the Colemak layout.
type ColemakKeyboard struct {
	adjacencyKeyboard
}

// NewQWERTYKeyboard creates a QWERTY keyboard layout instance.
func NewQWERTYKeyboard() *QWERTYKeyboard {
	return &QWERTYKeyboard{
		adjacencyKeyboard: adjacencyKeyboard{
			adjacentKeys: buildAdjacentKeyMap(qwertyAdjacencyList),
		},
	}
}

// NewDvorakKeyboard creates a Dvorak keyboard layout instance.
func NewDvorakKeyboard() *DvorakKeyboard {
	return &DvorakKeyboard{
		adjacencyKeyboard: adjacencyKeyboard{
			adjacentKeys: buildAdjacentKeyMap(dvorakAdjacencyList),
		},
	}
}

// NewColemakKeyboard creates a Colemak keyboard layout instance.
func NewColemakKeyboard() *ColemakKeyboard {
	return &ColemakKeyboard{
		adjacencyKeyboard: adjacencyKeyboard{
			adjacentKeys: buildAdjacentKeyMap(colemakAdjacencyList),
		},
	}
}

// KeyboardByName returns a keyboard layout instance by name.
func KeyboardByName(name string) (KeyboardWeights, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "qwerty":
		return NewQWERTYKeyboard(), nil
	case "dvorak":
		return NewDvorakKeyboard(), nil
	case "colemak":
		return NewColemakKeyboard(), nil
	default:
		return nil, fmt.Errorf("unsupported keyboard layout: %s", name)
	}
}

func buildAdjacentKeyMap(adjacencyList map[string]string) map[rune]map[rune]bool {
	adjacentKeys := make(map[rune]map[rune]bool, len(adjacencyList))

	for key, neighbors := range adjacencyList {
		keyRune := []rune(key)[0]
		if adjacentKeys[keyRune] == nil {
			adjacentKeys[keyRune] = make(map[rune]bool)
		}

		for _, neighborRune := range neighbors {
			adjacentKeys[keyRune][neighborRune] = true

			if adjacentKeys[neighborRune] == nil {
				adjacentKeys[neighborRune] = make(map[rune]bool)
			}
			adjacentKeys[neighborRune][keyRune] = true
		}
	}

	return adjacentKeys
}

var qwertyAdjacencyList = map[string]string{
	"`":  "1",
	"1":  "`2",
	"2":  "13",
	"3":  "24",
	"4":  "35",
	"5":  "46",
	"6":  "57",
	"7":  "68",
	"8":  "79",
	"9":  "80",
	"0":  "9-",
	"-":  "0=",
	"=":  "-",
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
	"a":  "qwsz",
	"s":  "awedxz",
	"d":  "sferxc",
	"f":  "dgrtcv",
	"g":  "fhtyb",
	"h":  "gjyun",
	"j":  "hkuim",
	"k":  "jli,",
	"l":  "k;.",
	";":  "l'",
	"'":  ";",
	"z":  "asx",
	"x":  "zsdc",
	"c":  "xdfv",
	"v":  "cfgb",
	"b":  "vghn",
	"n":  "bhjm",
	"m":  "njk,",
	",":  "mkl.",
	".":  ",l/",
	"/":  ".",
}

var dvorakAdjacencyList = map[string]string{
	"'": "1,",
	",": "'ao.",
	".": ",ep/",
	"p": ".uy",
	"y": "pfiu",
	"f": "ygdi",
	"g": "fchd",
	"c": "grht",
	"r": "cltn",
	"l": "rns/",
	"/": "l-",
	"a": ",;oeq",
	"o": ",aeu;q",
	"e": "o.uij",
	"u": "eipk",
	"i": "uydkx",
	"d": "ifhtx",
	"h": "dgtmb",
	"t": "hcrnw",
	"n": "trlsw",
	"s": "nl-v",
	";": "qoa",
	"q": ";aj",
	"j": "qek",
	"k": "juix",
	"x": "kidb",
	"b": "xhmv",
	"m": "btwv",
	"w": "mnvz",
	"v": "wbsz",
	"z": "vw",
	"-": "s",
}

var colemakAdjacencyList = map[string]string{
	"q": "wa",
	"w": "qarf",
	"f": "wprst",
	"p": "fgtd",
	"g": "pjdh",
	"j": "glhy",
	"l": "june",
	"u": "lyei",
	"y": "uio",
	";": "op",
	"a": "qwrxz",
	"r": "wfstx",
	"s": "arfcdx",
	"t": "spdgvc",
	"d": "tghvb",
	"h": "djnkb",
	"n": "hlemk",
	"e": "nlu,m",
	"i": "euo,",
	"o": "iy.;",
	"z": "asx",
	"x": "zsrc",
	"c": "xstv",
	"v": "ctdb",
	"b": "vdhk",
	"k": "bhnm",
	"m": "kne,",
	",": "mei.",
	".": ",io/",
	"/": ".",
}

// DefaultKeyboard is the default QWERTY keyboard layout.
var DefaultKeyboard KeyboardWeights = NewQWERTYKeyboard()
