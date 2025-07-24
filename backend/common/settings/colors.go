package settings

import (
	"regexp"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
)

// Strict hex color regex: #RGB, #RGBA, #RRGGBB, #RRGGBBAA
var hexColorRE = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

// All valid CSS named colors (W3C spec, lower-case)
var cssNamedColors = map[string]struct{}{
	"aliceblue": {}, "antiquewhite": {}, "aqua": {}, "aquamarine": {}, "azure": {},
	"beige": {}, "bisque": {}, "black": {}, "blanchedalmond": {}, "blue": {}, "blueviolet": {},
	"brown": {}, "burlywood": {}, "cadetblue": {}, "chartreuse": {}, "chocolate": {},
	"coral": {}, "cornflowerblue": {}, "cornsilk": {}, "crimson": {}, "cyan": {},
	"darkblue": {}, "darkcyan": {}, "darkgoldenrod": {}, "darkgray": {}, "darkgreen": {},
	"darkgrey": {}, "darkkhaki": {}, "darkmagenta": {}, "darkolivegreen": {}, "darkorange": {},
	"darkorchid": {}, "darkred": {}, "darksalmon": {}, "darkseagreen": {}, "darkslateblue": {},
	"darkslategray": {}, "darkslategrey": {}, "darkturquoise": {}, "darkviolet": {}, "deeppink": {},
	"deepskyblue": {}, "dimgray": {}, "dimgrey": {}, "dodgerblue": {}, "firebrick": {},
	"floralwhite": {}, "forestgreen": {}, "fuchsia": {}, "gainsboro": {}, "ghostwhite": {},
	"gold": {}, "goldenrod": {}, "gray": {}, "green": {}, "greenyellow": {}, "grey": {},
	"honeydew": {}, "hotpink": {}, "indianred": {}, "indigo": {}, "ivory": {},
	"khaki": {}, "lavender": {}, "lavenderblush": {}, "lawngreen": {}, "lemonchiffon": {},
	"lightblue": {}, "lightcoral": {}, "lightcyan": {}, "lightgoldenrodyellow": {}, "lightgray": {},
	"lightgreen": {}, "lightgrey": {}, "lightpink": {}, "lightsalmon": {}, "lightseagreen": {},
	"lightskyblue": {}, "lightslategray": {}, "lightslategrey": {}, "lightsteelblue": {}, "lightyellow": {},
	"lime": {}, "limegreen": {}, "linen": {}, "magenta": {}, "maroon": {}, "mediumaquamarine": {},
	"mediumblue": {}, "mediumorchid": {}, "mediumpurple": {}, "mediumseagreen": {}, "mediumslateblue": {},
	"mediumspringgreen": {}, "mediumturquoise": {}, "mediumvioletred": {}, "midnightblue": {}, "mintcream": {},
	"mistyrose": {}, "moccasin": {}, "navajowhite": {}, "navy": {}, "oldlace": {},
	"olive": {}, "olivedrab": {}, "orange": {}, "orangered": {}, "orchid": {},
	"palegoldenrod": {}, "palegreen": {}, "paleturquoise": {}, "palevioletred": {}, "papayawhip": {},
	"peachpuff": {}, "peru": {}, "pink": {}, "plum": {}, "powderblue": {},
	"purple": {}, "rebeccapurple": {}, "red": {}, "rosybrown": {}, "royalblue": {},
	"saddlebrown": {}, "salmon": {}, "sandybrown": {}, "seagreen": {}, "seashell": {},
	"sienna": {}, "silver": {}, "skyblue": {}, "slateblue": {}, "slategray": {},
	"slategrey": {}, "snow": {}, "springgreen": {}, "steelblue": {}, "tan": {},
	"teal": {}, "thistle": {}, "tomato": {}, "turquoise": {}, "violet": {},
	"wheat": {}, "white": {}, "whitesmoke": {}, "yellow": {}, "yellowgreen": {},
	// And the special values:
	"transparent": {}, "currentcolor": {},
}

func FallbackColor(val, def string) string {
	val = strings.TrimSpace(val)
	if val == "" {
		return def
	}
	if hexColorRE.MatchString(val) {
		return val
	}
	lc := strings.ToLower(val)
	// CSS color functions
	if strings.HasPrefix(lc, "rgb(") || strings.HasPrefix(lc, "rgba(") ||
		strings.HasPrefix(lc, "hsl(") || strings.HasPrefix(lc, "hsla(") {
		return val
	}
	// CSS variable
	if strings.HasPrefix(lc, "var(") {
		return val
	}
	// Named colors and special values
	if _, ok := cssNamedColors[lc]; ok {
		return val
	}
	// Log a warning if the color is invalid
	logger.Warningf("Invalid CSS color value provided: '%s'. Falling back to default: '%s'", val, def)
	return def
}
