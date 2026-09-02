package crypto

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

var defaultWordList = []string{
	"amber", "anchor", "apple", "apron", "arch", "arrow", "asteroid", "atlas", "autumn", "badge",
	"beacon", "bear", "breeze", "bridge", "bronze", "cabin", "cactus", "canyon", "castle", "cedar",
	"cliff", "clover", "comet", "compass", "coral", "crater", "crystal", "dawn", "delta", "desert",
	"diamond", "dolphin", "dragon", "drift", "dune", "eagle", "echo", "ember", "emerald", "falcon",
	"feather", "fern", "flame", "flint", "forest", "fossil", "fox", "galaxy", "glacier", "granite",
	"harbor", "hawk", "helix", "horizon", "iceberg", "island", "jade", "jungle", "lagoon", "lantern",
	"lapis", "lava", "leaf", "legacy", "lotus", "lunar", "magnet", "maple", "marble", "meadow",
	"meteor", "mist", "monolith", "moon", "moss", "nebula", "nexus", "oasis", "ocean", "onyx",
	"opal", "orbit", "peak", "pebble", "phoenix", "pine", "planet", "plasma", "prism", "pulsar",
	"quartz", "radar", "radiant", "rainbow", "raven", "reef", "ridge", "river", "rocket", "ruby",
	"sage", "sapphire", "saturn", "shadow", "sierra", "silver", "solar", "spark", "summit", "sun",
	"talon", "timber", "titan", "topaz", "torrent", "tower", "valley", "vapor", "velvet", "vortex",
	"wave", "willow", "wind", "wolf", "zenith", "zephyr",
}

// GenerateCode generates a memorable pairing code (e.g., "7-crystal-dragon-falcon")
func GenerateCode(wordCount int) (string, error) {
	if wordCount < 2 {
		wordCount = 2
	}
	prefixNum, err := rand.Int(rand.Reader, big.NewInt(90))
	if err != nil {
		return "", err
	}
	num := prefixNum.Int64() + 10 // 10-99

	words := make([]string, wordCount)
	listLen := big.NewInt(int64(len(defaultWordList)))
	for i := 0; i < wordCount; i++ {
		idx, err := rand.Int(rand.Reader, listLen)
		if err != nil {
			return "", err
		}
		words[i] = defaultWordList[idx.Int64()]
	}

	return fmt.Sprintf("%d-%s", num, strings.Join(words, "-")), nil
}

// SanitizeCode normalizes user entered code (lowercases, strips quotes/spaces/accidental chars)
func SanitizeCode(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	code = strings.Trim(code, "\"'`")
	code = strings.ReplaceAll(code, " ", "-")
	code = strings.ReplaceAll(code, "_", "-")
	code = strings.ReplaceAll(code, "@", "0") // In case of OCR typo @ -> 0
	return code
}
