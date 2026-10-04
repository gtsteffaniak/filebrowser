package state

// bootstrapWords is a fixed list of short, common English nouns for one-time bootstrap passwords.
// Words are chosen to be easy to say, spell, and type (including for non-native English speakers).
var bootstrapWords = []string{
	"apple", "ball", "barn", "bean", "bed", "belt", "bike", "bird", "boat", "book",
	"boot", "bowl", "box", "bus", "cake", "camp", "cart", "cat", "clock", "cloud",
	"coat", "coin", "corn", "cup", "desk", "dog", "door", "duck", "egg", "farm",
	"fish", "flag", "food", "foot", "game", "gift", "goat", "gold", "grass", "hand",
	"hat", "hill", "home", "horse", "house", "ice", "jar", "key", "kid", "lake",
	"lamp", "land", "leaf", "leg", "light", "line", "lion", "list", "mail", "map",
	"milk", "moon", "mouse", "nest", "net", "night", "nose", "note", "page", "park",
	"path", "pen", "pet", "pie", "pig", "pin", "pipe", "plan", "plant", "pot",
	"rain", "red", "ring", "road", "rock", "room", "rope", "rose", "sand", "sea",
	"ship", "shoe", "shop", "sky", "snow", "sock", "star", "stone", "sun", "tree",
}

// bootstrapSpeakableCharset excludes ambiguous characters for read-aloud (no 0/o, 1/i/l).
const bootstrapSpeakableCharset = "abcdefghjkmnpqrstuvwxyz23456789"

// bootstrapSpeakableCodeLen is the number of random charset characters after the word (e.g. word-xxxxx).
const bootstrapSpeakableCodeLen = 5
