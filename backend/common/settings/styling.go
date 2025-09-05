package settings

import (
	"os"
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

func FallbackColor(val, defaultColor string) string {
	val = strings.TrimSpace(val)
	if val == "" {
		return defaultColor
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
	logger.Warningf("Invalid CSS color value provided: '%s'. Falling back to default: '%s'", val, defaultColor)
	return defaultColor
}

func addCustomTheme(name, description, cssFilePath string) {
	// Store only file path in config (for YAML export)
	Config.Frontend.Styling.CustomThemes[name] = CustomTheme{
		Description: description,
		CSS:         cssFilePath, // Store file path, not content
	}

	// Load CSS content for runtime use (frontend consumption)
	cssContent := ""
	if cssFilePath != "" {
		cssContent = readCustomCSS(cssFilePath)
	}
	Config.Frontend.Styling.CustomThemeOptions[name] = CustomTheme{
		Description: description,
		CSS:         cssFilePath,
		CssRaw:      cssContent, // Store loaded content
	}
}

func readCustomCSS(path string) string {
	if path == "" {
		return ""
	}
	content, err := os.ReadFile(path)
	if err != nil {
		// Only log if the file exists but cannot be read (e.g., permission error)
		if !os.IsNotExist(err) {
			logger.Warningf("Could not read custom CSS file %s: %v", path, err)
			return ""
		}
	}
	if len(content) > 128*1024 {
		logger.Warningf("Custom CSS file %s is too large (%d bytes), ignoring", path, len(content))
		return ""
	}
	logger.Infof("Loaded custom CSS from: %s (%d bytes)", path, len(content))
	return string(content)
}
