package web

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// collaboraDiscovery is an excerpt of what Collabora CODE serves: its
// discovery.xml with %SRV_PROTO% and urlsrc filled in, including the image
// actions and the legacy MIME-type apps it also declares.
const collaboraDiscovery = `<?xml version="1.0" encoding="utf-8"?>
<wopi-discovery>
  <net-zone name="external-https">
    <app name="writer" favIconUrl="images/x-office-document.svg">
      <action name="edit" default="true" ext="docx" urlsrc="https://office.example/browser/abc123/cool.html?"/>
      <action name="view" ext="docx" urlsrc="https://office.example/browser/abc123/cool.html?"/>
      <action name="editnew" ext="docx" urlsrc="https://office.example/browser/abc123/cool.html?"/>
      <action name="view" default="true" ext="sxw" urlsrc="https://office.example/browser/abc123/cool.html?"/>
    </app>
    <app name="calc">
      <action name="edit" default="true" ext="XLSX" urlsrc="https://office.example/browser/abc123/cool.html?"/>
    </app>
    <app name="draw">
      <action name="view" ext="png" urlsrc="https://office.example/browser/abc123/cool.html?"/>
      <action name="view" ext="svg" urlsrc="https://office.example/browser/abc123/cool.html?"/>
    </app>
    <app name="application/pdf">
      <action name="view_comment" ext="" urlsrc="https://office.example/browser/abc123/cool.html?"/>
    </app>
    <app name="Capabilities">
      <action name="getinfo" ext="" urlsrc="https://office.example/hosting/capabilities"/>
    </app>
  </net-zone>
</wopi-discovery>`

// onlyOfficeDiscovery follows the shape of OnlyOffice Docs' WOPI discovery:
// urlsrc carries escaped placeholders such as <ui=UI_LLCC&>.
const onlyOfficeDiscovery = `<?xml version="1.0" encoding="utf-8"?>
<wopi-discovery>
  <net-zone name="external-http">
    <app name="Word">
      <action name="view" ext="docx" urlsrc="http://oo.internal/hosting/wopi/word/view?&amp;&lt;rs=DC_LLCC&amp;&gt;&lt;ui=UI_LLCC&amp;&gt;&lt;wopisrc=WOPI_SOURCE&amp;&gt;&amp;"/>
      <action name="edit" ext="docx" urlsrc="http://oo.internal/hosting/wopi/word/edit?&amp;&lt;rs=DC_LLCC&amp;&gt;&lt;ui=UI_LLCC&amp;&gt;&lt;wopisrc=WOPI_SOURCE&amp;&gt;&amp;"/>
    </app>
  </net-zone>
  <net-zone name="external-https">
    <app name="Word">
      <action name="view" ext="docx" urlsrc="https://oo.example/hosting/wopi/word/view?&amp;&lt;ui=UI_LLCC&amp;&gt;&amp;"/>
      <action name="edit" ext="docx" urlsrc="https://oo.example/hosting/wopi/word/edit?&amp;&lt;ui=UI_LLCC&amp;&gt;&amp;"/>
      <action name="embedview" ext="docx" urlsrc="https://oo.example/hosting/wopi/word/embedview?"/>
    </app>
  </net-zone>
</wopi-discovery>`

func TestParseWopiDiscoveryCollabora(t *testing.T) {
	d, err := parseWopiDiscovery([]byte(collaboraDiscovery), "https://office.example", "")
	if err != nil {
		t.Fatalf("parseWopiDiscovery: %v", err)
	}
	if d.Product != wopiProductCollabora {
		t.Errorf("Product = %q, want collabora (Capabilities app present)", d.Product)
	}
	exts := d.extensions()
	want := map[string]string{"docx": "edit", "xlsx": "edit", "sxw": "view"}
	if len(exts) != len(want) {
		t.Fatalf("extensions = %v, want %v", exts, want)
	}
	for ext, mode := range want {
		if exts[ext] != mode {
			t.Errorf("extensions[%q] = %q, want %q", ext, exts[ext], mode)
		}
	}
	for _, native := range []string{"png", "svg"} {
		if _, ok := exts[native]; ok {
			t.Errorf("%s must stay with FileBrowser's own viewer", native)
		}
	}
	if src, ok := d.editorURL("DOCX", false); !ok || !strings.Contains(src, "cool.html") {
		t.Errorf("editorURL(DOCX, view) = %q, %v", src, ok)
	}
	if _, ok := d.editorURL("pdf", false); ok {
		t.Error("pdf is only declared by MIME type and must not be routed")
	}
}

func TestParseWopiDiscoveryOnlyOfficePrefersSchemeZone(t *testing.T) {
	d, err := parseWopiDiscovery([]byte(onlyOfficeDiscovery), "https://oo.example", "")
	if err != nil {
		t.Fatalf("parseWopiDiscovery: %v", err)
	}
	if d.Product != wopiProductOnlyOffice {
		t.Errorf("Product = %q, want onlyoffice", d.Product)
	}
	edit, _ := d.editorURL("docx", true)
	if !strings.HasPrefix(edit, "https://oo.example/hosting/wopi/word/edit") {
		t.Errorf("edit urlsrc = %q, want the external-https zone's edit action", edit)
	}
	view, _ := d.editorURL("docx", false)
	if !strings.Contains(view, "/word/view") {
		t.Errorf("view urlsrc = %q", view)
	}

	forced, err := parseWopiDiscovery([]byte(onlyOfficeDiscovery), "https://oo.example", wopiProductGeneric)
	if err != nil {
		t.Fatal(err)
	}
	if forced.Product != wopiProductGeneric {
		t.Errorf("configured product must override detection, got %q", forced.Product)
	}
}

func TestParseWopiDiscoveryRejectsUnusable(t *testing.T) {
	for name, doc := range map[string]string{
		"not xml":       "<wopi-discovery",
		"internal only": `<wopi-discovery><net-zone name="internal-https"><app name="a"><action name="edit" ext="docx" urlsrc="https://x/"/></app></net-zone></wopi-discovery>`,
		"no actions":    `<wopi-discovery><net-zone name="external-https"><app name="Capabilities"><action name="getinfo" ext="" urlsrc="https://x/"/></app></net-zone></wopi-discovery>`,
	} {
		if _, err := parseWopiDiscovery([]byte(doc), "https://x", ""); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestBuildWopiEditorURL(t *testing.T) {
	const wopiSrc = "https://files.example/wopi/files/abc?x=1"

	got, err := buildWopiEditorURL("https://office.example/browser/abc123/cool.html?", wopiSrc, wopiProductCollabora, "fr")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(got)
	if u.Query().Get("WOPISrc") != wopiSrc {
		t.Errorf("WOPISrc = %q, want %q", u.Query().Get("WOPISrc"), wopiSrc)
	}
	if !strings.Contains(u.RawQuery, "WOPISrc=https%3A%2F%2Ffiles.example") {
		t.Errorf("WOPISrc must be percent-encoded, got %q", u.RawQuery)
	}
	if u.Query().Get("lang") != "fr" || u.Query().Get("closebutton") != "1" {
		t.Errorf("collabora params missing in %q", got)
	}

	got, err = buildWopiEditorURL("https://oo.example/hosting/wopi/word/edit?&<rs=DC_LLCC&><ui=UI_LLCC&>&keep=yes", wopiSrc, wopiProductOnlyOffice, "de")
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(got, "<>") {
		t.Errorf("placeholders must be dropped, got %q", got)
	}
	u, _ = url.Parse(got)
	if u.Query().Get("ui") != "de" || u.Query().Get("keep") != "yes" || u.Query().Get("lang") != "" {
		t.Errorf("onlyoffice params wrong in %q", got)
	}
}

func TestWopiLocale(t *testing.T) {
	for in, want := range map[string]string{"fr": "fr", "ptBR": "pt-BR", "zhTW": "zh-TW", "cz": "cs", "ua": "uk", "": ""} {
		if got := wopiLocale(in); got != want {
			t.Errorf("wopiLocale(%q) = %q, want %q", in, got, want)
		}
	}
}

func setWopiTestConfig(t *testing.T) {
	t.Helper()
	orig := settings.Config.Integrations.OnlyOffice
	origKey := settings.Config.Auth.Key
	t.Cleanup(func() {
		settings.Config.Integrations.OnlyOffice = orig
		settings.Config.Auth.Key = origKey
	})
	settings.Config.Auth.Key = "test-signing-key-for-length-check"
	settings.Config.Integrations.OnlyOffice = settings.OnlyOffice{Url: "https://office.example", Product: settings.OfficeProductCollabora, TokenExpirationHours: 10}
}

func TestOfficeProductSelection(t *testing.T) {
	orig := settings.Config.Integrations.OnlyOffice
	t.Cleanup(func() { settings.Config.Integrations.OnlyOffice = orig })

	cases := []struct {
		office    settings.OnlyOffice
		product   string
		collabora bool
	}{
		{settings.OnlyOffice{}, "", false},
		{settings.OnlyOffice{Product: settings.OfficeProductCollabora}, "", false},
		{settings.OnlyOffice{Url: "https://oo.example", Secret: "s"}, "onlyoffice", false},
		{settings.OnlyOffice{Url: "https://oo.example", Secret: "s", Product: "onlyoffice"}, "onlyoffice", false},
		{settings.OnlyOffice{Url: "https://cool.example", Product: "collabora"}, "collabora", true},
	}
	for _, c := range cases {
		settings.Config.Integrations.OnlyOffice = c.office
		if got := officeProductForSPA(); got != c.product {
			t.Errorf("%+v: officeProductForSPA = %q, want %q", c.office, got, c.product)
		}
		if wopiEnabled() != c.collabora {
			t.Errorf("%+v: wopiEnabled = %v, want %v", c.office, wopiEnabled(), c.collabora)
		}
		if origins := onlyOfficeScriptSrcOrigins(); c.collabora && len(origins) != 0 {
			t.Errorf("collabora must not add script-src origins, got %v", origins)
		}
	}
}

func TestWopiDiscoveryRefresherRetriesUntilEditorIsUp(t *testing.T) {
	setWopiTestConfig(t)
	var up atomic.Bool
	editor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !up.Load() || r.URL.Path != "/hosting/discovery" {
			http.Error(w, "starting", http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, collaboraDiscovery)
	}))
	t.Cleanup(editor.Close)
	settings.Config.Integrations.OnlyOffice.Url = editor.URL

	origRetry := wopiDiscoveryRetry
	wopiDiscoveryCache.Store(nil)
	t.Cleanup(func() {
		wopiDiscoveryRetry = origRetry
		wopiDiscoveryCache.Store(nil)
	})
	wopiDiscoveryRetry = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	runWopiDiscoveryRefresher(ctx)

	time.Sleep(100 * time.Millisecond)
	if len(wopiExtensionsForSPA()) != 0 {
		t.Fatal("no extensions are expected while the editor is down")
	}
	up.Store(true)
	deadline := time.Now().Add(2 * time.Second)
	for len(wopiExtensionsForSPA()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("discovery was never loaded after the editor came up")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if wopiExtensionsForSPA()["docx"] != wopiActionEdit {
		t.Errorf("extensions = %v", wopiExtensionsForSPA())
	}
}

func TestWopiTokenRoundTrip(t *testing.T) {
	setWopiTestConfig(t)
	now := time.Now()
	raw, expires, err := mintWopiToken(wopiClaims{FileID: "fid1", Source: "srv", Path: "/a.docx", UserID: 7, Write: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if got := expires.Sub(now); got != 10*time.Hour {
		t.Errorf("ttl = %v, want 10h", got)
	}
	c, err := parseWopiToken(raw, "fid1")
	if err != nil {
		t.Fatalf("parseWopiToken: %v", err)
	}
	if c.UserID != 7 || !c.Write || c.Path != "/a.docx" {
		t.Errorf("claims = %+v", c)
	}
	if _, err := parseWopiToken(raw, "fid2"); err == nil {
		t.Error("a token must not open another file")
	}
	if _, err := parseWopiToken("", "fid1"); err == nil {
		t.Error("missing token must be rejected")
	}

	// Rotating the secret invalidates outstanding tokens.
	settings.Config.Integrations.OnlyOffice.Secret = "explicit-secret"
	if _, err := parseWopiToken(raw, "fid1"); err == nil {
		t.Error("token signed with the derived key must fail under an explicit secret")
	}
}

func TestWopiTokenRejectsExpiredAndForeignTokens(t *testing.T) {
	setWopiTestConfig(t)
	raw, _, err := mintWopiToken(wopiClaims{FileID: "fid1"}, time.Now().Add(-11*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseWopiToken(raw, "fid1"); err == nil {
		t.Error("expired token must be rejected")
	}

	// A token signed with the session key (what the auth middleware issues)
	// must not be accepted here, even with a matching file id.
	session := jwt.NewWithClaims(jwt.SigningMethodHS256, wopiClaims{
		FileID:           "fid1",
		RegisteredClaims: jwt.RegisteredClaims{Audience: jwt.ClaimStrings{wopiTokenAudience}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	})
	signed, _ := session.SignedString([]byte(settings.Config.Auth.Key))
	if _, err := parseWopiToken(signed, "fid1"); err == nil {
		t.Error("a token signed with the session key must be rejected")
	}

	// Right key, wrong audience.
	key, _ := wopiSigningKey()
	noAud := jwt.NewWithClaims(jwt.SigningMethodHS256, wopiClaims{
		FileID:           "fid1",
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	})
	signed, _ = noAud.SignedString(key)
	if _, err := parseWopiToken(signed, "fid1"); err == nil {
		t.Error("a token without the wopi audience must be rejected")
	}
}

func TestWopiLockTable(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	tbl := newWopiLockTable()
	tbl.now = func() time.Time { return now }

	if ok, _ := tbl.lock("f", "A"); !ok {
		t.Fatal("first lock must succeed")
	}
	if ok, _ := tbl.lock("f", "A"); !ok {
		t.Error("relocking with the same id refreshes")
	}
	if ok, cur := tbl.lock("f", "B"); ok || cur != "A" {
		t.Errorf("lock by another id = %v,%q; want conflict with A", ok, cur)
	}
	if ok, cur := tbl.checkPut("f", "B"); ok || cur != "A" {
		t.Errorf("put with wrong lock = %v,%q; want conflict with A", ok, cur)
	}
	if ok, _ := tbl.checkPut("f", "A"); !ok {
		t.Error("put with the held lock must pass")
	}
	if ok, cur := tbl.unlockAndRelock("f", "X", "B"); ok || cur != "A" {
		t.Errorf("relock from wrong old id = %v,%q", ok, cur)
	}
	if ok, _ := tbl.unlockAndRelock("f", "A", "B"); !ok || tbl.get("f") != "B" {
		t.Error("unlockAndRelock must swap A for B")
	}
	if ok, cur := tbl.refresh("f", "A"); ok || cur != "B" {
		t.Errorf("refresh with stale id = %v,%q", ok, cur)
	}
	if ok, cur := tbl.unlock("f", "A"); ok || cur != "B" {
		t.Errorf("unlock with stale id = %v,%q", ok, cur)
	}
	if ok, _ := tbl.unlock("f", "B"); !ok || tbl.get("f") != "" {
		t.Error("unlock with the held id must release")
	}
	if ok, cur := tbl.refresh("f", "B"); ok || cur != "" {
		t.Errorf("refresh on unlocked file = %v,%q; want conflict with empty lock", ok, cur)
	}

	// Unlocked files accept a save, whatever lock id the editor still holds.
	if ok, _ := tbl.checkPut("f", "lost-after-restart"); !ok {
		t.Error("put on an unlocked file must pass")
	}

	// Locks expire after wopiLockDuration.
	tbl.lock("g", "A")
	now = now.Add(wopiLockDuration + time.Second)
	if tbl.get("g") != "" {
		t.Error("lock must expire")
	}
	if ok, _ := tbl.lock("g", "B"); !ok {
		t.Error("an expired lock must not block a new one")
	}
}

func TestWopiStorageChanged(t *testing.T) {
	mod := time.Date(2026, 10, 8, 9, 30, 15, 123456789, time.UTC)
	seen := wopiTimestamp(mod)
	if wopiStorageChanged(seen, mod) {
		t.Errorf("the timestamp we rendered (%s) must match the file it came from", seen)
	}
	if wopiStorageChanged("", mod) {
		t.Error("no header means overwrite unconditionally")
	}
	if !wopiStorageChanged(seen, mod.Add(time.Second)) {
		t.Error("a later modification must be reported")
	}
	if wopiStorageChanged("garbage", mod) {
		t.Error("an unparsable header must not block saving")
	}
}

func TestRedactAccessToken(t *testing.T) {
	got := redactAccessToken("access_token=secret.jwt.value&access_token_ttl=1")
	if strings.Contains(got, "secret") || !strings.Contains(got, "access_token_ttl=1") {
		t.Errorf("redactAccessToken = %q", got)
	}
	if redactAccessToken("path=/a&source=b") != "path=/a&source=b" {
		t.Error("queries without a token must be untouched")
	}
}

// wopiHostFixture serves the WOPI endpoints for one real file in a temp dir,
// with the user store and file lookups stubbed.
type wopiHostFixture struct {
	t        *testing.T
	user     *users.User
	realPath string
	fileID   string
	handler  http.Handler
}

func newWopiHostFixture(t *testing.T, perms users.SourceFilePermissions) *wopiHostFixture {
	t.Helper()
	initStreamTestSources(t)
	setWopiTestConfig(t)

	dir := t.TempDir()
	realPath := filepath.Join(dir, "report.docx")
	if err := os.WriteFile(realPath, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}

	user := testUserWithView(42, "srv")
	user.BackendSourcePermissions["/srv"] = perms
	user.BackendScopes[0].Permissions = perms

	origUser, origInfo, origWrite := wopiUserByID, files.FileInfoFasterFunc, files.WriteFileFunc
	origLocks := wopiLocks
	origCacheDir := settings.Config.Server.CacheDir
	t.Cleanup(func() {
		wopiUserByID, files.FileInfoFasterFunc, files.WriteFileFunc = origUser, origInfo, origWrite
		wopiLocks = origLocks
		settings.Config.Server.CacheDir = origCacheDir
	})
	wopiLocks = newWopiLockTable()
	settings.Config.Server.CacheDir = dir
	wopiUserByID = func(id uint64) (users.User, error) { return *user, nil }
	files.FileInfoFasterFunc = func(opts utils.FileOptions, _ *users.User) (*iteminfo.ExtendedFileInfo, error) {
		if opts.Path != "/docs/report.docx" {
			return nil, os.ErrNotExist
		}
		return &iteminfo.ExtendedFileInfo{
			FileInfo: iteminfo.FileInfo{ItemInfo: iteminfo.ItemInfo{Name: "report.docx"}},
			RealPath: realPath,
		}, nil
	}
	files.WriteFileFunc = func(source, path string, in io.Reader) error {
		if source != "srv" || path != "/docs/report.docx" {
			t.Errorf("WriteFile(%q, %q): unexpected target", source, path)
		}
		data, err := io.ReadAll(in)
		if err != nil {
			return err
		}
		return os.WriteFile(realPath, data, 0o600)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /wopi/files/{id}", withWopiToken(wopiCheckFileInfoHandler))
	mux.HandleFunc("POST /wopi/files/{id}", withWopiToken(wopiFilesPostHandler))
	mux.HandleFunc("GET /wopi/files/{id}/contents", withWopiToken(wopiGetFileHandler))
	mux.HandleFunc("POST /wopi/files/{id}/contents", withWopiToken(wopiPutFileHandler))

	return &wopiHostFixture{
		t: t, user: user, realPath: realPath,
		fileID:  wopiFileID("srv", realPath),
		handler: mux,
	}
}

func (f *wopiHostFixture) token(write bool) string {
	f.t.Helper()
	raw, _, err := mintWopiToken(wopiClaims{
		FileID: f.fileID, Source: "srv", Path: "/docs/report.docx",
		UserID: f.user.ID, Write: write, Origin: "https://files.example",
	}, time.Now())
	if err != nil {
		f.t.Fatal(err)
	}
	return raw
}

func (f *wopiHostFixture) do(method, suffix, token string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	f.t.Helper()
	target := "/wopi/files/" + f.fileID + suffix + "?access_token=" + url.QueryEscape(token)
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	return rec
}

func TestWopiHostEditSession(t *testing.T) {
	f := newWopiHostFixture(t, users.SourceFilePermissions{View: true, Download: true, Modify: true})
	tok := f.token(true)

	rec := f.do(http.MethodGet, "", tok, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("CheckFileInfo status = %d", rec.Code)
	}
	var info wopiCheckFileInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info.BaseFileName != "report.docx" || info.Size != int64(len("original")) || !info.UserCanWrite || info.ReadOnly {
		t.Errorf("CheckFileInfo = %+v", info)
	}
	if info.PostMessageOrigin != "https://files.example" || info.DisableExport {
		t.Errorf("origin/export flags wrong: %+v", info)
	}

	rec = f.do(http.MethodGet, "/contents", tok, nil, nil)
	if rec.Code != http.StatusOK || rec.Body.String() != "original" || rec.Header().Get("X-WOPI-ItemVersion") == "" {
		t.Fatalf("GetFile = %d %q", rec.Code, rec.Body.String())
	}

	if rec = f.do(http.MethodPost, "", tok, nil, map[string]string{"X-WOPI-Override": "LOCK", "X-WOPI-Lock": "L1"}); rec.Code != http.StatusOK {
		t.Fatalf("LOCK = %d", rec.Code)
	}
	rec = f.do(http.MethodPost, "/contents", tok, []byte("intruder"), map[string]string{"X-WOPI-Override": "PUT", "X-WOPI-Lock": "L2"})
	if rec.Code != http.StatusConflict || rec.Header().Get("X-WOPI-Lock") != "L1" {
		t.Fatalf("PutFile with foreign lock = %d lock=%q", rec.Code, rec.Header().Get("X-WOPI-Lock"))
	}

	rec = f.do(http.MethodPost, "/contents", tok, []byte("edited"), map[string]string{
		"X-WOPI-Override":       "PUT",
		"X-WOPI-Lock":           "L1",
		"X-COOL-WOPI-Timestamp": info.LastModifiedTime,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("PutFile = %d %s", rec.Code, rec.Body.String())
	}
	if got, _ := os.ReadFile(f.realPath); string(got) != "edited" {
		t.Errorf("file content = %q, want edited", got)
	}
	var saved map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &saved)
	if saved["LastModifiedTime"] == "" {
		t.Error("PutFile must return the new LastModifiedTime")
	}

	// The file changes behind the editor's back (SFTP, another user...).
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(f.realPath, later, later); err != nil {
		t.Fatal(err)
	}
	rec = f.do(http.MethodPost, "/contents", tok, []byte("stale"), map[string]string{
		"X-WOPI-Override":       "PUT",
		"X-WOPI-Lock":           "L1",
		"X-COOL-WOPI-Timestamp": saved["LastModifiedTime"],
	})
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "1010") {
		t.Fatalf("PutFile over an external change = %d %s, want 409 COOLStatusCode 1010", rec.Code, rec.Body.String())
	}
	if got, _ := os.ReadFile(f.realPath); string(got) != "edited" {
		t.Errorf("conflicting save must not write, file = %q", got)
	}

	if rec = f.do(http.MethodPost, "", tok, nil, map[string]string{"X-WOPI-Override": "RENAME_FILE"}); rec.Code != http.StatusNotImplemented {
		t.Errorf("RENAME_FILE = %d, want 501", rec.Code)
	}
}

func TestWopiHostRejections(t *testing.T) {
	f := newWopiHostFixture(t, users.SourceFilePermissions{View: true, Download: false})

	if rec := f.do(http.MethodGet, "", "", nil, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("no token: %d, want 401", rec.Code)
	}

	// A write token is capped by the user's actual permissions.
	tok := f.token(true)
	rec := f.do(http.MethodGet, "", tok, nil, nil)
	var info wopiCheckFileInfo
	_ = json.Unmarshal(rec.Body.Bytes(), &info)
	if info.UserCanWrite || !info.DisableExport || !info.DisablePrint {
		t.Errorf("view-only user without download: %+v", info)
	}
	if rec := f.do(http.MethodPost, "/contents", tok, []byte("x"), map[string]string{"X-WOPI-Override": "PUT"}); rec.Code != http.StatusUnauthorized {
		t.Errorf("PutFile without modify = %d, want 401", rec.Code)
	}
	if rec := f.do(http.MethodPost, "", tok, nil, map[string]string{"X-WOPI-Override": "LOCK", "X-WOPI-Lock": "L"}); rec.Code != http.StatusUnauthorized {
		t.Errorf("LOCK without modify = %d, want 401", rec.Code)
	}

	// Permission revoked mid-session.
	f.user.BackendSourcePermissions["/srv"] = users.SourceFilePermissions{}
	f.user.BackendScopes[0].Permissions = users.SourceFilePermissions{}
	if rec := f.do(http.MethodGet, "", tok, nil, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("after revocation: %d, want 401", rec.Code)
	}
}

func TestWopiHostRejectsMovedFile(t *testing.T) {
	// The session opened on a file whose real path is no longer the one at
	// the token's path: it was moved, and another file took its name.
	f := newWopiHostFixture(t, users.SourceFilePermissions{View: true})
	f.fileID = wopiFileID("srv", "/somewhere/else.docx")
	if rec := f.do(http.MethodGet, "", f.token(false), nil, nil); rec.Code != http.StatusNotFound {
		t.Errorf("moved file: %d, want 404", rec.Code)
	}
}
