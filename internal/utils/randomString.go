package utils

import (
	cryptoRand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

var letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func LittleContainsString(slice []string, item string) bool {
	for _, a := range slice {
		if a == item {
			return true
		}
	}
	return false
}

var adjectives = []string{
	"brave", "silent", "lucky", "fuzzy", "fancy", "sleepy", "rapid", "wild",
	"flying", "hidden", "mighty", "epic", "gentle", "icy", "fiery", "shadow",
	"noisy", "clever", "bright", "cosmic", "electric", "legendary", "tiny",
	"massive", "neon", "retro", "chill", "frozen", "golden", "silver", "iron",
	"mystic", "liquid", "ghost", "swift", "solar", "lunar", "stellar", "atomic",
	"velvet", "crimson", "emerald", "azure", "purple", "turbo", "chaotic",
	"cosmic", "digital", "vivid", "rapid", "endless", "floating", "angry",
	"gentle", "dynamic", "funky", "oceanic", "forest", "dusty", "shiny",
	"soft", "burning", "stormy", "rainy", "windy", "wooden", "foggy", "smart",
	"hyper", "crazy", "loud", "quiet", "odd", "prime", "ultimate", "primal",
}

var nouns = []string{
	"wolf", "fox", "bear", "lion", "tiger", "dragon", "eagle", "falcon",
	"shark", "octopus", "panther", "hawk", "owl", "rhino", "duck",
	"penguin", "otter", "panda", "koala", "gorilla", "lemur", "buffalo",
	"salmon", "viper", "python", "unicorn", "griffin", "phoenix", "hydra",
	"robot", "android", "drone", "satellite", "meteor", "comet", "nebula",
	"galaxy", "planet", "star", "rocket", "module", "probe", "asteroid",
	"cloud", "storm", "cyclone", "whirlwind", "blizzard", "tsunami",
	"ghost", "spirit", "demon", "wizard", "mage", "knight", "pirate",
	"ninja", "samurai", "viking", "ranger", "archer", "monk", "explorer",
	"raccoon", "hamster", "whale", "mole", "badger", "lizard", "frog",
	"crab", "turtle", "dolphin", "foxhound", "lynx", "beetle", "ant",
	"spider", "hornet", "mammoth", "boar", "stallion", "mustang", "gecko",
	"raptor", "saber", "cobra", "serpent", "jaguar", "mongoose",
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func cryptoRandom4Digits() int {
	b := make([]byte, 2)
	_, _ = cryptoRand.Read(b)
	return int(binary.BigEndian.Uint16(b))%9000 + 1000
}

func GenerateWorkspaceName() string {
	adj := adjectives[rand.Intn(len(adjectives))]
	n := nouns[rand.Intn(len(nouns))]
	num := cryptoRandom4Digits()

	return adj + "-" + n + "-" + strconv.Itoa(num)
}

var nicknameAdjectives = []string{
	"silent", "brave", "cosmic", "lucky", "fuzzy", "fancy", "sleepy", "rapid", "wild",
	"flying", "hidden", "mighty", "epic", "gentle", "icy", "fiery", "shadow",
	"bright", "clever", "golden", "silver", "iron", "retro", "chill", "frozen",
	"neon", "digital", "velvet", "crimson", "emerald", "azure", "purple", "ghost",
	"hyper", "soft", "stormy", "foggy", "smart", "electric", "prime", "stellar",
	"quiet", "bold", "soft", "whispering", "burning", "shining", "distant", "secret",
	"lunar", "solar", "galactic", "ancient", "mythic", "enchanted", "radiant", "blurred",
	"frosty", "windy", "stormborn", "lucky", "arcane", "mystic", "sacred", "divine",
	"gentle", "kind", "peaceful", "harmonic", "playful", "curious", "restless", "floating",
	"timeless", "endless", "eternal", "infinite", "tiny", "massive", "majestic", "primal",
	"sparkling", "shimmering", "glowing", "sonic", "turbo", "vivid", "vintage", "quantum",
	"plasma", "dynamic", "magnetic", "holographic", "future", "parallel", "virtual",
	"tactical", "steady", "nimble", "swift", "fluffy", "cozy", "sunny", "iridescent",
	"noble", "arcadian", "crystal", "organic", "synthetic", "elegant", "bold", "fresh",
	"cloudy", "skybound", "glacial", "wired",
}

var nicknameNouns = []string{
	"lion", "wolf", "fox", "eagle", "bear", "tiger", "otter", "panda", "dragon",
	"owl", "falcon", "phoenix", "unicorn", "wizard", "ninja", "samurai", "pirate",
	"viking", "ranger", "explorer", "robot", "android", "panther", "lynx",
	"raptor", "cobra", "mammoth", "stallion", "whale", "shark", "gecko", "crab",
	"rabbit", "mouse", "hedgehog", "sparrow", "hawk", "pigeon", "swan", "seal",
	"walrus", "dolphin", "orca", "jellyfish", "octopus", "koala", "sloth", "lemur",
	"chimp", "gorilla", "bull", "buffalo", "boar", "camel", "yak", "horse",
	"puma", "jaguar", "fennec", "reindeer", "ferret", "weasel", "badger", "bison",
	"griffin", "wyvern", "golem", "spirit", "sprite", "fairy", "druid", "monk",
	"knight", "archer", "warrior", "scholar", "sage", "scribe", "bard", "traveler",
	"guardian", "sentinel", "keeper", "wanderer", "seer", "pilot", "captain",
	"astronaut", "cosmonaut", "starwalker", "voyager", "nomad", "runner", "sprinter",
	"stalker", "echo", "shadow", "ember", "flame", "frost", "glacier", "comet",
	"meteor", "nebula", "quantum", "pixel", "module", "orb", "cube", "beacon",
	"anchor", "harbor", "river", "forest", "meadow", "mountain", "valley", "horizon",
	"machine", "device", "kernel", "protocol", "signal", "circuit",
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

// cryptoSecureSuffix генерирует безопасный трёхсимвольный суффикс.
// Например: "x3d", "p9k".
func cryptoSecureSuffix() string {
	b := make([]byte, 2)
	_, _ = cryptoRand.Read(b)
	val := binary.BigEndian.Uint16(b)

	// берём 3 символа из [a-z0-9]
	chars := []rune("abcdefghijklmnopqrstuvwxyz0123456789")

	return string([]rune{
		chars[val%36],
		chars[(val/36)%36],
		chars[(val/1296)%36],
	})
}

// GenerateUserNickname генерирует красивый никнейм вида:
//
//	silent-fox-x3d
//	cosmic-panda-9a1
//	lucky-dragon-qw7
//
// Можно чистить капитализацию, если хочешь.
func GenerateUserNickname() string {
	adj := nicknameAdjectives[rand.Intn(len(nicknameAdjectives))]
	noun := nicknameNouns[rand.Intn(len(nicknameNouns))]

	// нижний регистр — более стильный и универсальный
	adj = strings.ToLower(adj)
	noun = strings.ToLower(noun)

	suffix := cryptoSecureSuffix()

	return adj + "-" + noun + "-" + suffix
}
