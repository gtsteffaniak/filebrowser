package users

import "strings"

// PrepForFrontend fills response-only fields (scopes, links, locale) for a user value returned from GET handlers.
// FrontendScopes are not loaded from or saved to the database.
func (u *User) PrepForFrontend() {
	u.FrontendScopes = u.GetFrontendScopes()
	u.SidebarLinks = u.GetFrontendSidebarLinks()
	u.Password = ""
	u.OtpEnabled = u.TOTPSecret != ""
	u.Locale = normalizeLocaleForFrontend(u.Locale)
}

// normalizeLocaleForFrontend converts various locale formats to camelCase (e.g. zhCN, ptBR).
func normalizeLocaleForFrontend(locale string) string {
	if locale == "" {
		return locale
	}

	lower := strings.ToLower(locale)

	specialCases := map[string]string{
		"cs":    "cz",
		"uk":    "ua",
		"zh-cn": "zhCN",
		"zh_cn": "zhCN",
		"zhcn":  "zhCN",
		"zh-tw": "zhTW",
		"zh_tw": "zhTW",
		"zhtw":  "zhTW",
		"pt-br": "ptBR",
		"pt_br": "ptBR",
		"ptbr":  "ptBR",
		"sv-se": "svSE",
		"sv_se": "svSE",
		"svse":  "svSE",
		"nl-be": "nlBE",
		"nl_be": "nlBE",
		"nlbe":  "nlBE",
	}

	if normalized, ok := specialCases[lower]; ok {
		return normalized
	}

	if len(locale) >= 4 {
		knownCamelCase := map[string]bool{
			"zhCN": true, "zhTW": true, "ptBR": true, "svSE": true, "nlBE": true,
		}
		if knownCamelCase[locale] {
			return locale
		}
	}

	parts := strings.FieldsFunc(lower, func(r rune) bool {
		return r == '_' || r == '-'
	})

	if len(parts) == 2 {
		first := parts[0]
		second := parts[1]
		if len(second) > 0 {
			second = strings.ToUpper(second[:1]) + second[1:]
		}
		normalized := first + second

		knownCompound := map[string]string{
			"zhcn": "zhCN", "zhtw": "zhTW", "ptbr": "ptBR",
			"svse": "svSE", "nlbe": "nlBE",
		}
		if normalizedVal, ok := knownCompound[normalized]; ok {
			return normalizedVal
		}
		return normalized
	}

	return lower
}
