# Third-Party Notices — AcornDrive (FileBrowser Fork)

This document acknowledges the open-source software incorporated into AcornDrive, a fork of
[FileBrowser Quantum](https://github.com/gtsteffaniak/filebrowser). It is provided in compliance
with the attribution requirements of the Apache Licence 2.0 (upstream) and to satisfy
best-practice disclosure obligations for a publicly distributed derivative work.

---

## Upstream Project

**FileBrowser Quantum**
- Repository: https://github.com/gtsteffaniak/filebrowser
- Derived from: https://github.com/filebrowser/filebrowser
- Licence: Apache Licence 2.0
- Copyright: Copyright 2018 File Browser contributors

A copy of the Apache Licence 2.0 is available in the `LICENSE` file at the repository root.
All modifications made by Nansen Limited are documented in `Fork.md`.

---

## Go Backend — Direct Production Dependencies

### github.com/asdine/storm/v3
- Licence: MIT
- Copyright: Copyright (c) 2016 Asdine El Hrychy

### github.com/coreos/go-oidc/v3
- Licence: Apache-2.0
- Copyright: Copyright 2015 CoreOS, Inc.

### github.com/dhowden/tag
- Licence: BSD-2-Clause
- Copyright: Copyright (c) 2015, David Howden

### github.com/dsoprea/go-exif/v3
- Licence: MIT
- Copyright: Copyright (c) 2018 Dustin Oprea

### github.com/gen2brain/go-fitz
- Licence: **AGPL-3.0** (via MuPDF)
- Copyright: Copyright (C) Artifex Software, Inc.
- Note: This package wraps the MuPDF rendering library. AcornDrive's complete source
  including all Nansen modifications is publicly available in accordance with AGPL-3.0
  Section 13. See `README.md` for the user notice and source location.

### github.com/go-playground/validator/v10
- Licence: MIT
- Copyright: Copyright (c) 2015 Dean Karn

### github.com/goccy/go-yaml
- Licence: MIT
- Copyright: Copyright (c) 2019 Masaaki Goshima

### github.com/golang-jwt/jwt/v4
- Licence: MIT
- Copyright: Copyright (c) 2012 Dave Grijalva; Copyright (c) 2021 golang-jwt authors

### github.com/google/go-cmp
- Licence: BSD-3-Clause
- Copyright: Copyright (c) 2017 The Go Authors. All rights reserved.

### github.com/gtsteffaniak/go-cache
- Licence: MIT
- Copyright: Copyright (c) 2023 gtsteffaniak (fork of patrickmn/go-cache)

### github.com/gtsteffaniak/go-logger
- Licence: MIT
- Copyright: Copyright (c) 2023 gtsteffaniak

### github.com/kovidgoyal/imaging
- Licence: MIT
- Copyright: Copyright (c) 2012 Grigory Dryapak

### github.com/pquerna/otp
- Licence: Apache-2.0
- Copyright: Copyright 2014 Paul Querna

### github.com/spf13/afero
- Licence: Apache-2.0
- Copyright: Copyright © 2014 Steve Francia

### github.com/swaggo/http-swagger
- Licence: MIT
- Copyright: Copyright (c) 2020 Swagger

### github.com/swaggo/swag
- Licence: MIT
- Copyright: Copyright (c) 2017 Swagger

### go.etcd.io/bbolt
- Licence: MIT
- Copyright: Copyright (c) 2013 Ben Johnson

### golang.org/x/crypto
- Licence: BSD-3-Clause
- Copyright: Copyright (c) 2009 The Go Authors. All rights reserved.

### golang.org/x/image
- Licence: BSD-3-Clause
- Copyright: Copyright (c) 2009 The Go Authors. All rights reserved.

### golang.org/x/mod
- Licence: BSD-3-Clause
- Copyright: Copyright (c) 2009 The Go Authors. All rights reserved.

### golang.org/x/oauth2
- Licence: BSD-3-Clause
- Copyright: Copyright (c) 2009 The Go Authors. All rights reserved.

### golang.org/x/sys
- Licence: BSD-3-Clause
- Copyright: Copyright (c) 2009 The Go Authors. All rights reserved.

### golang.org/x/time
- Licence: BSD-3-Clause
- Copyright: Copyright (c) 2009 The Go Authors. All rights reserved.

### gopkg.in/yaml.v3
- Licence: MIT and Apache-2.0
- Copyright: Copyright (c) 2011-2019 Canonical Ltd

### modernc.org/sqlite
- Licence: BSD-style (zlib-equivalent)
- Copyright: Copyright (c) 2017 modernc.org authors

---

## Go Backend — Notable Indirect Dependencies

### github.com/hashicorp/hcl
- Licence: MPL-2.0 (Mozilla Public Licence 2.0)
- Copyright: Copyright (c) 2013 HashiCorp, Inc.
- Note: Indirect dependency via `spf13/viper`. MPL-2.0 is file-level copyleft; Nansen does
  not modify any HCL source files. No distribution obligation arises.

### go.uber.org/zap
- Licence: MIT
- Copyright: Copyright (c) 2016 Uber Technologies, Inc.

### go.uber.org/multierr
- Licence: MIT
- Copyright: Copyright (c) 2017 Uber Technologies, Inc.

---

## Vue 3 Frontend Dependencies

### vue / vue-router / vue-i18n / vue-lazyload
- Licence: MIT
- Copyright: Copyright (c) 2013-present Yuxi (Evan) You and contributors

### axios
- Licence: MIT
- Copyright: Copyright (c) 2014-present Matt Zabriskie & Collaborators

### @onlyoffice/document-editor-vue
- Licence: Apache-2.0
- Copyright: Copyright (c) Ascensio System SIA

### @skjnldsv/vue-plyr
- Licence: MIT
- Copyright: Copyright (c) 2022 Samuel Kacer (fork of github.com/redxtech/vue-plyr)

### ace-builds
- Licence: BSD-3-Clause
- Copyright: Copyright (c) 2010, Ajax.org B.V.

### clipboard
- Licence: MIT
- Copyright: Copyright (c) 2017 Zeno Rocha

### css-vars-ponyfill
- Licence: MIT
- Copyright: Copyright (c) 2018 John Hildenbiddle

### dompurify
- Licence: Apache-2.0 (selected; dual Apache-2.0 / MPL-2.0)
- Copyright: Copyright (c) 2015 Mario Heiderich

### epubjs
- Licence: BSD-2-Clause
- Copyright: Copyright (c) 2013 Futurepress Inc.

### highlight.js
- Licence: BSD-3-Clause
- Copyright: Copyright (c) 2006, Ivan Sagalaev

### mammoth
- Licence: BSD-2-Clause
- Copyright: Copyright (c) 2013, Michael Williamson

### marked
- Licence: MIT
- Copyright: Copyright (c) 2011-2022, Christopher Jeffrey

### normalize.css
- Licence: MIT
- Copyright: Copyright (c) Nicolas Gallagher and Jonathan Neal

### qrcode.vue
- Licence: MIT
- Copyright: Copyright (c) 2021 scopewu

### srt-support-for-html5-videos
- Licence: MIT
- Copyright: Copyright (c) 2019 silviugrumazescu

---

## Build and Development Tools (not distributed)

The following tools are used during development and CI/CD only and are not distributed to
end users. Attribution is provided as best practice:

| Tool | Licence |
|------|---------|
| vite, @vitejs/plugin-vue | MIT |
| vitest, @playwright/test | MIT |
| eslint and plugins | MIT |
| vue-tsc | MIT |
| deepl-node | MIT |
| golangci-lint | GPL-3.0 (tool only — not linked into the binary) |
| swaggo/swag CLI | MIT |
| air-verse/air | MIT |

---

*This notices file was last updated May 2026. It covers direct production dependencies and
notable indirect dependencies. A full transitive inventory can be produced on request via
`go mod graph` (backend) and `npm ls --all` (frontend).*
