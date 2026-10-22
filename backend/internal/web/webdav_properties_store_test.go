package web

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/webdav"
)

func TestDAVPropertyStorePersistentCopy(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "props.sqlite")
	s, err := openDAVPropertyStore(filename)
	if err != nil {
		t.Fatal(err)
	}
	name := xml.Name{Space: "urn:test", Local: "value"}
	value := webdav.Property{XMLName: name, Lang: "ru", InnerXML: []byte(`<x:item xmlns:x="urn:nested">Привет &amp; hi</x:item>`)}
	status, err := s.Patch("/srv/фото", []webdav.Proppatch{{Props: []webdav.Property{value}}})
	if err != nil || status[0].Status != 200 {
		t.Fatal(status, err)
	}
	value.InnerXML[0] = '!'
	got, err := s.Get("/srv/фото")
	if err != nil || got[name].InnerXML[0] != '<' {
		t.Fatal(got, err)
	}
	got[name].InnerXML[0] = '!'
	delete(got, name)
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = openDAVPropertyStore(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err = s.Get("/srv/фото")
	if err != nil || got[name].InnerXML[0] != '<' || got[name].Lang != "ru" {
		t.Fatal(got, err)
	}
	st, err := os.Stat(filename)
	if err != nil || st.Mode().Perm() != 0600 {
		t.Fatal(st, err)
	}
}
func TestDAVPropertyStoreAtomicLimits(t *testing.T) {
	s, err := openDAVPropertyStore(filepath.Join(t.TempDir(), "props.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	name := xml.Name{Space: "urn:test", Local: "original"}
	original := webdav.Property{XMLName: name, InnerXML: []byte("original")}
	if _, err = s.Patch("/srv/file", []webdav.Proppatch{{Props: []webdav.Property{original}}}); err != nil {
		t.Fatal(err)
	}
	var many []webdav.Property
	for i := 0; i < 128; i++ {
		many = append(many, webdav.Property{XMLName: xml.Name{Space: "urn:test", Local: fmt.Sprintf("p%d", i)}})
	}
	status, err := s.Patch("/srv/file", []webdav.Proppatch{{Props: many}})
	if err != nil || status[0].Status != 403 {
		t.Fatal(status, err)
	}
	got, _ := s.Get("/srv/file")
	if len(got) != 1 || string(got[name].InnerXML) != "original" {
		t.Fatal("count failure changed stored properties")
	}
	huge := webdav.Property{XMLName: name, InnerXML: []byte(strings.Repeat("x", davPropertyMaxBytes))}
	status, err = s.Patch("/srv/file", []webdav.Proppatch{{Remove: true, Props: []webdav.Property{original}}, {Props: []webdav.Property{huge}}})
	if err != nil || status[0].Status != 507 {
		t.Fatal(status, err)
	}
	got, _ = s.Get("/srv/file")
	if string(got[name].InnerXML) != "original" {
		t.Fatal("size failure partially applied")
	}
	if _, err = s.Patch("/srv/file", []webdav.Proppatch{{Remove: true, Props: []webdav.Property{original}}}); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Get("/srv/file")
	if len(got) != 0 {
		t.Fatal(got)
	}
	if _, err = s.Get("relative"); err == nil {
		t.Fatal("relative key accepted")
	}
}
func TestDAVPropertyStoreRecursiveMoveDelete(t *testing.T) {
	s, err := openDAVPropertyStore(filepath.Join(t.TempDir(), "props.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := webdav.Property{XMLName: xml.Name{Space: "urn:test", Local: "value"}, InnerXML: []byte("yes")}
	for _, key := range []string{"/srv/a%_", "/srv/a%_/child", "/srv/a%_-sibling", "/srv/axx", "/srv/dest/stale"} {
		if _, err = s.Patch(key, []webdav.Proppatch{{Props: []webdav.Property{p}}}); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.Move("/srv/a%_", "/srv/dest"); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"/srv/dest", "/srv/dest/child", "/srv/a%_-sibling", "/srv/axx"} {
		got, getErr := s.Get(key)
		if getErr != nil || len(got) != 1 {
			t.Fatal(key, got, getErr)
		}
	}
	for _, key := range []string{"/srv/a%_", "/srv/a%_/child", "/srv/dest/stale"} {
		got, getErr := s.Get(key)
		if getErr != nil || len(got) != 0 {
			t.Fatal(key, got, getErr)
		}
	}
	if err = s.Delete("/srv/dest"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get("/srv/dest/child")
	if len(got) != 0 {
		t.Fatal(got)
	}
	got, _ = s.Get("/srv/a%_-sibling")
	if len(got) != 1 {
		t.Fatal("sibling deleted")
	}
	if err = s.Move("/srv", "/srv/nested"); err == nil {
		t.Fatal("overlapping move accepted")
	}
}
