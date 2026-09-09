package web

import "strings"

func injectViteDevHTML(html string) []byte {
	const clientScript = `<script type="module" src="/__vite/@vite/client"></script>`
	const entryScript = `<script type="module" src="/__vite/src/main.ts"></script>`
	const sourceEntry = `<script type="module" src="/src/main.ts"></script>`

	if !strings.Contains(html, "</head>") || !strings.Contains(html, sourceEntry) {
		return []byte(html)
	}
	html = strings.Replace(html, "</head>", clientScript+"\n</head>", 1)
	html = strings.Replace(html, sourceEntry, entryScript, 1)
	return []byte(html)
}
