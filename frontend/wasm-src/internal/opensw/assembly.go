package opensw

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Shared traversal budget: repeated instances are kept while they fit, but a
// crafted tree cannot expand forever or allocate unbounded components.
type assemblyBudget struct{ work, output int }

const (
	maxAssemblyWork  = 1 << 20
	maxAssemblyParts = 1 << 20
)

// Assembly reading, ported from OpenSW (sldprt assembly.py). An assembly is
// the same container as a part; two streams suffice to show it, both written
// already computed by SolidWorks:
//
//   - swXmlContents/COMPINSTANCETREE: each component, its file, its states
//     and its swTransform matrix.
//   - FaceTessellations/*: the display mesh of each distinct part, face by
//     face, as triangle strips (older files: Contents/DisplayLists).
//
// Mates are not read: the browser worker never uses them.

const swNS = "http://www.solidworks.com/sw2003/schema"

// utf16Mark is the mark of an MFC UTF-16 string: FF FE FF then its length.
var utf16Mark = []byte{0xFF, 0xFE, 0xFF}

// ErrAssembly marks files that are not assemblies this reader supports.
var ErrAssembly = errors.New("opensw: not a readable assembly")

// Component is a placed part instance.
type Component struct {
	Name string // "cube-1": part name and instance number, subassemblies prefixed.
	File string
	// Matrix as MathTransform.ArrayData: R by rows, t, scale, zeros.
	Matrix     [16]float64
	Hidden     bool
	Suppressed bool
	// ExcludeFromBOM: excluded from the bill of materials.
	ExcludeFromBOM bool
	// Config is the part configuration the instance shows.
	Config string
	// Faces is the display mesh, in metres, in the part's frame: face →
	// triangle → 3 points.
	Faces [][][3][3]float64
}

// Place transforms a part point (m) into the assembly frame: p·R + t.
func (c *Component) Place(p [3]float64) [3]float64 {
	m := &c.Matrix
	var q [3]float64
	for j := 0; j < 3; j++ {
		q[j] = p[0]*m[j] + p[1]*m[3+j] + p[2]*m[6+j] + m[9+j]
	}
	return q
}

// Assembly is a document ready to show: placed components with meshes.
type Assembly struct {
	Components []*Component
}

// --- component tree -----------------------------------------------------------

// xmlNode is a minimal element tree; attribute names are not namespaced here.
type xmlNode struct {
	space    string
	local    string
	attrs    map[string]string
	children []*xmlNode
}

func (n *xmlNode) is(local string) bool {
	return n.local == local && n.space == swNS
}

func parseXMLTree(data []byte) (*xmlNode, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	var stack []*xmlNode
	var root *xmlNode
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("opensw: bad component tree XML: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			node := &xmlNode{space: t.Name.Space, local: t.Name.Local, attrs: map[string]string{}}
			for _, a := range t.Attr {
				node.attrs[a.Name.Local] = a.Value
			}
			if len(stack) > 0 {
				stack[len(stack)-1].children = append(stack[len(stack)-1].children, node)
			} else if root == nil {
				root = node
			}
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if root == nil {
		return nil, errors.New("opensw: empty component tree")
	}
	return root, nil
}

// swMatrix parses swTransform (16 numbers, MathTransform.ArrayData layout):
// R by rows, t, scale, three zeros.
func swMatrix(text string) ([16]float64, error) {
	fields := strings.Fields(text)
	if len(fields) != 16 {
		return [16]float64{}, fmt.Errorf("%w: swTransform with %d numbers", ErrAssembly, len(fields))
	}
	var v [16]float64
	for i, f := range fields {
		x, err := strconv.ParseFloat(f, 64)
		if err != nil {
			return [16]float64{}, fmt.Errorf("%w: swTransform number %q", ErrAssembly, f)
		}
		v[i] = x
	}
	// v[15] is the scale. Neither Place nor compose applies it, so a non-unit
	// value would silently produce wrongly-sized geometry; reject it instead.
	if math.Abs(v[15]-1) > 1e-9 {
		return [16]float64{}, fmt.Errorf("%w: non-unit swTransform scale %g", ErrAssembly, v[15])
	}
	return [16]float64{
		v[0], v[1], v[2],
		v[4], v[5], v[6],
		v[8], v[9], v[10],
		v[12], v[13], v[14],
		v[15], 0, 0, 0,
	}, nil
}

// compose gives a subassembly component's matrix in the assembly:
// R = child_R·parent_R, t = child_t·parent_R + parent_t.
func compose(child, parent [16]float64) [16]float64 {
	var r [9]float64
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			for k := 0; k < 3; k++ {
				r[3*i+j] += child[3*i+k] * parent[3*k+j]
			}
		}
	}
	var t [3]float64
	for j := 0; j < 3; j++ {
		for k := 0; k < 3; k++ {
			t[j] += child[9+k] * parent[3*k+j]
		}
		t[j] += parent[9+j]
	}
	return [16]float64{
		r[0], r[1], r[2], r[3], r[4], r[5], r[6], r[7], r[8],
		t[0], t[1], t[2], 1, 0, 0, 0,
	}
}

var identityMatrix = [16]float64{1, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0}

// treeComponents walks the COMPINSTANCETREE XML into placed components.
func treeComponents(xmldata []byte) ([]*Component, error) {
	root, err := parseXMLTree(xmldata)
	if err != nil {
		return nil, err
	}
	var files, models []*xmlNode
	var walk func(n *xmlNode)
	walk = func(n *xmlNode) {
		if n.is("swFile") {
			files = append(files, n)
		} else if n.is("swModel") {
			models = append(models, n)
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	walk(root)

	var head *xmlNode
	for _, f := range files {
		if f.attrs["swDocType"] == "ASSEMBLY" {
			head = f
			break
		}
	}
	if head == nil {
		return nil, fmt.Errorf("%w: no ASSEMBLY file in the tree", ErrAssembly)
	}
	fileByID := map[string]*xmlNode{}
	for _, f := range files {
		fileByID[f.attrs["id"]] = f
	}
	modelByID := map[string]*xmlNode{}
	for _, m := range models {
		modelByID[m.attrs["id"]] = m
	}
	modelOfFile := func(fileID string) *xmlNode {
		for _, m := range models {
			if m.attrs["swFileRef"] == fileID {
				return m
			}
		}
		return nil
	}
	var out []*Component
	// activePath holds the assembly file IDs on the current recursion path;
	// revisiting one means the instance tree is cyclic and would recurse
	// forever. Entries are removed on unwind so the same subassembly stays
	// valid when referenced from separate branches.
	var descend func(model *xmlNode, parent [16]float64, prefix string, hidden, suppressed, excluded bool, activePath map[string]bool, budget *assemblyBudget) error
	descend = func(model *xmlNode, parent [16]float64, prefix string, hidden, suppressed, excluded bool, activePath map[string]bool, budget *assemblyBudget) error {
		for _, ref := range model.children {
			if !ref.is("swReference") {
				continue
			}
			budget.work++
			if budget.work > maxAssemblyWork {
				return fmt.Errorf("%w: assembly work budget exceeded", ErrAssembly)
			}
			a := ref.attrs
			name := fmt.Sprintf("%s-%s", a["swName"], a["swReferenceNumber"])
			m, err := swMatrix(a["swTransform"])
			if err != nil {
				continue
			}
			m = compose(m, parent)
			child := modelByID[a["swModelRef"]]
			var file *xmlNode
			if child != nil {
				file = fileByID[child.attrs["swFileRef"]]
			}
			h := hidden || a["swHidden"] == "YES"
			s := suppressed || a["swSuppressed"] == "YES"
			x := excluded || a["swExcludeFromBOM"] == "YES"
			if file != nil && file.attrs["swDocType"] == "ASSEMBLY" {
				if sub := modelOfFile(file.attrs["id"]); sub != nil {
					fileID := file.attrs["id"]
					if activePath[fileID] {
						return fmt.Errorf("%w: cyclic assembly reference to file %q", ErrAssembly, fileID)
					}
					activePath[fileID] = true
					err := descend(sub, m, prefix+name+"/", h, s, x, activePath, budget)
					delete(activePath, fileID)
					if err != nil {
						return err
					}
					continue
				}
			}
			path := ""
			if file != nil {
				path = file.attrs["swPath"]
			}
			budget.output++
			if budget.output > maxAssemblyParts {
				return fmt.Errorf("%w: assembly component budget exceeded", ErrAssembly)
			}
			out = append(out, &Component{
				Name:           prefix + name,
				File:           winBaseName(path),
				Matrix:         m,
				Hidden:         h,
				Suppressed:     s,
				ExcludeFromBOM: x,
				Config:         a["swConfigurationName"],
			})
		}
		return nil
	}
	model := modelOfFile(head.attrs["id"])
	if model == nil {
		return nil, fmt.Errorf("%w: the assembly model has no reference", ErrAssembly)
	}
	if err := descend(model, identityMatrix, "", false, false, false, map[string]bool{head.attrs["id"]: true}, &assemblyBudget{}); err != nil {
		return nil, err
	}
	return out, nil
}

// winBaseName is the last path segment of a Windows path.
func winBaseName(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

// --- display mesh ---------------------------------------------------------------

// mfcCString reads an MFC UTF-16 string: FF FE FF, one length byte, then the
// text. Returns the text and the offset past it.
func mfcCString(d []byte, at int) (string, int, bool) {
	if at+4 > len(d) || !bytes.Equal(d[at:at+3], utf16Mark) {
		return "", 0, false
	}
	n := int(d[at+3])
	fin := at + 4 + 2*n
	if fin > len(d) {
		return "", 0, false
	}
	u16 := make([]uint16, n)
	for i := 0; i < n; i++ {
		u16[i] = binary.LittleEndian.Uint16(d[at+4+2*i:])
	}
	return string(utf16.Decode(u16)), fin, true
}

type dirEntry struct {
	name    string
	payload []byte
}

// directoryEntries lists the entries of the Directory stream. An entry starts
// with "name@Assem…"; what follows it, up to the next one, holds the 8-byte
// key of the mesh it reads.
func directoryEntries(d []byte) []dirEntry {
	type site struct {
		at, fin int
		name    string
	}
	var names []site
	for at := 0; at+3 <= len(d); {
		pos := bytes.Index(d[at:], utf16Mark)
		if pos < 0 {
			break
		}
		pos += at
		if s, fin, ok := mfcCString(d, pos); ok && strings.Contains(s, "@") {
			names = append(names, site{pos, fin, strings.SplitN(s, "@", 2)[0]})
			at = fin
		} else {
			at = pos + 3
		}
	}
	out := make([]dirEntry, 0, len(names))
	for k, s := range names {
		next := len(d)
		if k+1 < len(names) {
			next = names[k+1].at
		}
		out = append(out, dirEntry{s.name, d[s.fin:next]})
	}
	return out
}

var (
	bandesHeader = [3]uint32{4, 8, 2}
	pointsHeader = [3]uint32{12, 0x64, 2}
)

type displayFaceBlock struct {
	start     int
	end       int
	triangles [][3][3]float64
}

// blockFaceBlocks reads face strips and their optional normals. A face is a
// list of triangles with offsets delimiting its following appearance data.
func blockFaceBlocks(d []byte, debut, fin int) []displayFaceBlock {
	if debut < 0 || fin > len(d) || debut >= fin {
		return nil
	}
	var faces []displayFaceBlock
	i := debut
	for i < fin-16 {
		h := [3]uint32{
			binary.LittleEndian.Uint32(d[i:]),
			binary.LittleEndian.Uint32(d[i+4:]),
			binary.LittleEndian.Uint32(d[i+8:]),
		}
		if h != bandesHeader {
			i++
			continue
		}
		k := int(binary.LittleEndian.Uint32(d[i+12:]))
		if k < 1 || k > 100_000 || i+16+4*k+16 > fin {
			i++
			continue
		}
		longueurs := make([]uint32, k)
		var n uint64
		minL := uint32(math.MaxUint32)
		for j := 0; j < k; j++ {
			longueurs[j] = binary.LittleEndian.Uint32(d[i+16+4*j:])
			n += uint64(longueurs[j])
			if longueurs[j] < minL {
				minL = longueurs[j]
			}
		}
		j := i + 16 + 4*k
		ph := [4]uint32{
			binary.LittleEndian.Uint32(d[j:]),
			binary.LittleEndian.Uint32(d[j+4:]),
			binary.LittleEndian.Uint32(d[j+8:]),
			binary.LittleEndian.Uint32(d[j+12:]),
		}
		if minL < 3 || ph[0] != pointsHeader[0] || ph[1] != pointsHeader[1] ||
			ph[2] != pointsHeader[2] || ph[3] != uint32(n) || n > uint64((fin-j-16)/12) {
			i++
			continue
		}
		pointCount := int(n)
		pts := make([][3]float32, pointCount)
		for a := 0; a < pointCount; a++ {
			for c := 0; c < 3; c++ {
				pts[a][c] = math.Float32frombits(binary.LittleEndian.Uint32(d[j+16+12*a+4*c:]))
			}
		}
		var tris [][3][3]float64
		o := 0
		for _, L := range longueurs {
			for a := 0; a < int(L)-2; a++ {
				p, q, r := o+a, o+a+1, o+a+2
				tri := [3][3]float64{
					{float64(pts[p][0]), float64(pts[p][1]), float64(pts[p][2])},
					{float64(pts[q][0]), float64(pts[q][1]), float64(pts[q][2])},
					{float64(pts[r][0]), float64(pts[r][1]), float64(pts[r][2])},
				}
				if a%2 != 0 { // strip winding alternates
					tri[0], tri[1] = tri[1], tri[0]
				}
				tris = append(tris, tri)
			}
			o += int(L)
		}
		end := j + 16 + 12*pointCount
		if end+16 <= fin {
			nh := [4]uint32{
				binary.LittleEndian.Uint32(d[end:]),
				binary.LittleEndian.Uint32(d[end+4:]),
				binary.LittleEndian.Uint32(d[end+8:]),
				binary.LittleEndian.Uint32(d[end+12:]),
			}
			if nh == [4]uint32{pointsHeader[0], pointsHeader[1], pointsHeader[2], uint32(n)} &&
				n <= uint64((fin-end-16)/12) {
				end += 16 + 12*pointCount
			}
		}
		faces = append(faces, displayFaceBlock{start: i, end: end, triangles: tris})
		i = end
	}
	return faces
}

// blockFaces reads the faces of a block as triangle lists.
func blockFaces(d []byte, debut, fin int) [][][3][3]float64 {
	blocks := blockFaceBlocks(d, debut, fin)
	faces := make([][][3][3]float64, len(blocks))
	for i, block := range blocks {
		faces[i] = block.triangles
	}
	return faces
}

type meshKey struct {
	stream string
	at     int
	key    string
}

// tessellations maps component name → faces from FaceTessellations streams,
// falling back to Contents/DisplayLists for older files.
func tessellations(streams map[string][]byte) (map[string][][][3][3]float64, error) {
	directory := streams["FaceTessellations/Directory"]
	data := map[string][]byte{}
	for name, payload := range streams {
		if strings.HasPrefix(name, "FaceTessellations/") && name != "FaceTessellations/Directory" {
			data[name] = payload
		}
	}
	if directory == nil || len(data) == 0 {
		if dl := streams["Contents/DisplayLists"]; dl != nil {
			return legacyTessellations(dl)
		}
		return nil, fmt.Errorf("%w: no display mesh streams", ErrAssembly)
	}

	// Entry keys, found again in the data: an eight-byte window with at most
	// two zero bytes, present in a mesh stream.
	type entry struct {
		name string
		keys []string
	}
	var entries []entry
	starts := map[string][]meshKey{}
	for _, e := range directoryEntries(directory) {
		var keys []string
		for a := 0; a+8 <= len(e.payload); a++ {
			cle := e.payload[a : a+8]
			if bytes.Count(cle, []byte{0}) > 2 {
				continue
			}
			k := string(cle)
			if containsStr(keys, k) {
				continue
			}
			for stream, d := range data {
				at := bytes.Index(d, cle)
				if at < 0 {
					continue
				}
				keys = append(keys, k)
				mk := meshKey{stream, at, k}
				if !containsMeshKey(starts[stream], mk) {
					starts[stream] = append(starts[stream], mk)
				}
			}
		}
		entries = append(entries, entry{e.name, keys})
	}

	blocks := map[string][][][3][3]float64{}
	streamNames := make([]string, 0, len(data))
	for stream := range data {
		streamNames = append(streamNames, stream)
	}
	sort.Strings(streamNames)
	for _, stream := range streamNames {
		d := data[stream]
		tries := starts[stream]
		sortMeshKeys(tries)
		for x, mk := range tries {
			fin := len(d)
			if x+1 < len(tries) {
				fin = tries[x+1].at
			}
			candidate := blockFaces(d, mk.at+8, fin)
			if len(candidate) == 0 {
				continue
			}
			// Keep the key bound to its stream/offset until validated; an
			// empty or later match must never discard a valid mesh.
			if _, done := blocks[mk.key]; !done {
				blocks[mk.key] = candidate
			}
		}
	}
	out := map[string][][][3][3]float64{}
	for _, e := range entries {
		for _, k := range e.keys {
			if len(blocks[k]) > 0 {
				out[e.name] = blocks[k]
				break
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: no display mesh decoded", ErrAssembly)
	}
	return out, nil
}

// legacyTessellations reads Contents/DisplayLists: the same strip arrays,
// each component headed by "name-N@assembly" then its part path; the mesh of
// a part follows its first instance only and the others share it.
func legacyTessellations(d []byte) (map[string][][][3][3]float64, error) {
	type site struct {
		at   int
		text string
	}
	var chaines []site
	for at := 0; at+3 <= len(d); {
		pos := bytes.Index(d[at:], utf16Mark)
		if pos < 0 {
			break
		}
		pos += at
		if s, _, ok := mfcCString(d, pos); ok {
			chaines = append(chaines, site{pos, s})
		}
		at = pos + 3
	}
	// A component header is followed, a few strings later, by a part path;
	// annotation references ("name@assembly" too) are not.
	type tete struct {
		at   int
		nom  string
		file string
	}
	var tetes []tete
	for x, s := range chaines {
		if !strings.Contains(s.text, "@") {
			continue
		}
		chemin := ""
		for _, next := range chaines[min(x+1, len(chaines)):min(x+5, len(chaines))] {
			if strings.HasSuffix(strings.ToLower(next.text), ".sldprt") {
				chemin = next.text
				break
			}
		}
		if chemin != "" {
			tetes = append(tetes, tete{s.at, strings.SplitN(s.text, "@", 2)[0],
				strings.ToLower(winBaseName(chemin))})
		}
	}
	if len(tetes) == 0 {
		return nil, fmt.Errorf("%w: no display mesh in DisplayLists", ErrAssembly)
	}
	// The mesh of a part follows its first instance only; the other
	// instances share it.
	byFile := map[string][][][3][3]float64{}
	var noms [][2]string
	for k, t := range tetes {
		fin := len(d)
		if k+1 < len(tetes) {
			fin = tetes[k+1].at
		}
		if faces := blockFaces(d, t.at, fin); len(faces) > 0 {
			if _, done := byFile[t.file]; !done {
				byFile[t.file] = faces
			}
		}
		noms = append(noms, [2]string{t.nom, t.file})
	}
	out := map[string][][][3][3]float64{}
	for _, nf := range noms {
		if faces, ok := byFile[nf[1]]; ok {
			out[nf[0]] = faces
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: no display mesh decoded from DisplayLists", ErrAssembly)
	}
	return out, nil
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func containsMeshKey(list []meshKey, k meshKey) bool {
	for _, x := range list {
		if x == k {
			return true
		}
	}
	return false
}

func sortMeshKeys(list []meshKey) {
	// Insertion sort keeps duplicate-free small lists ordered by offset.
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && list[j].at < list[j-1].at; j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
}

// --- assembly ------------------------------------------------------------------

// ReadAssembly opens the document and returns it ready to show: placed
// components, each with its display mesh.
func ReadAssembly(data []byte) (*Assembly, error) {
	c, err := OpenContainer(data)
	if err != nil {
		return nil, err
	}
	xmlTree := c.Get("swXmlContents/COMPINSTANCETREE")
	if xmlTree == nil {
		return nil, fmt.Errorf("%w: no swXmlContents/COMPINSTANCETREE stream", ErrAssembly)
	}
	components, err := treeComponents(xmlTree)
	if err != nil {
		return nil, err
	}
	meshes, err := tessellations(c.Streams)
	if err != nil {
		return nil, err
	}
	for _, comp := range components {
		// The directory names a component by its last level.
		if faces, ok := meshes[lastSegment(comp.Name)]; ok {
			comp.Faces = faces
		}
	}
	return &Assembly{Components: components}, nil
}

func lastSegment(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}

// BoundingBox (m) of the placed mesh of visible components.
func (a *Assembly) BoundingBox() (lo, hi [3]float64, ok bool) {
	lo = [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}
	hi = [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for _, c := range a.Components {
		if c.Hidden || c.Suppressed {
			continue
		}
		for _, face := range c.Faces {
			for _, tri := range face {
				for _, p := range tri {
					q := c.Place(p)
					for j := 0; j < 3; j++ {
						lo[j] = math.Min(lo[j], q[j])
						hi[j] = math.Max(hi[j], q[j])
					}
				}
			}
		}
	}
	return lo, hi, !math.IsInf(lo[0], 1)
}

// ViewFace is one face of the 3D view mesh.
type ViewFace struct {
	Name      string    `json:"nom"`
	Body      int       `json:"corps"`
	Triangles int       `json:"triangles"`
	Color     []float64 `json:"couleur,omitempty"`
}

// View is the assembly in the page's 3D view format: millimetres, centred; a
// face is named "<component>:<index>".
type View struct {
	Faces    []ViewFace
	Bodies   int
	Radius   float64
	Vertices []float32
}

func (a *Assembly) ToView() (*View, error) {
	lo, hi, ok := a.BoundingBox()
	if !ok {
		return nil, fmt.Errorf("%w: no visible component has a mesh", ErrAssembly)
	}
	var centre [3]float64
	diag := 0.0
	for j := 0; j < 3; j++ {
		centre[j] = (lo[j] + hi[j]) / 2
		diag += (hi[j] - lo[j]) * (hi[j] - lo[j])
	}
	diag = 1000 * math.Sqrt(diag)

	var visible []*Component
	for _, c := range a.Components {
		if !c.Hidden && !c.Suppressed && len(c.Faces) > 0 {
			visible = append(visible, c)
		}
	}
	if len(visible) == 0 {
		return nil, fmt.Errorf("%w: no visible component has a mesh", ErrAssembly)
	}
	v := &View{Bodies: len(visible), Radius: diag / 2}
	for rang, c := range visible {
		for k, face := range c.Faces {
			for _, tri := range face {
				for _, p := range tri {
					q := c.Place(p)
					for j := 0; j < 3; j++ {
						v.Vertices = append(v.Vertices, float32(1000*(q[j]-centre[j])))
					}
				}
			}
			v.Faces = append(v.Faces, ViewFace{
				Name:      fmt.Sprintf("%s:%d", c.Name, k+1),
				Body:      rang + 1,
				Triangles: len(face),
			})
		}
	}
	return v, nil
}

// ToSTL writes the visible components' display mesh as binary STL, placed,
// in mm — SolidWorks' display mesh, not the exact geometry.
func (a *Assembly) ToSTL() ([]byte, error) {
	count := 0
	for _, c := range a.Components {
		if c.Hidden || c.Suppressed {
			continue
		}
		for _, face := range c.Faces {
			count += len(face)
		}
	}
	if count == 0 {
		return nil, fmt.Errorf("%w: no visible component has a mesh", ErrAssembly)
	}
	var out bytes.Buffer
	header := []byte("openSW: SolidWorks display mesh, mm")
	if len(header) < 80 {
		header = append(header, bytes.Repeat([]byte{' '}, 80-len(header))...)
	}
	out.Write(header[:80])
	_ = binary.Write(&out, binary.LittleEndian, uint32(count))
	var rec [50]byte // 12 floats + uint16 attribute count
	for _, c := range a.Components {
		if c.Hidden || c.Suppressed {
			continue
		}
		for _, face := range c.Faces {
			for _, tri := range face {
				var pts [3][3]float64
				for i, p := range tri {
					q := c.Place(p)
					pts[i] = [3]float64{1000 * q[0], 1000 * q[1], 1000 * q[2]}
				}
				u := [3]float64{pts[1][0] - pts[0][0], pts[1][1] - pts[0][1], pts[1][2] - pts[0][2]}
				w := [3]float64{pts[2][0] - pts[0][0], pts[2][1] - pts[0][1], pts[2][2] - pts[0][2]}
				n := [3]float64{
					u[1]*w[2] - u[2]*w[1],
					u[2]*w[0] - u[0]*w[2],
					u[0]*w[1] - u[1]*w[0],
				}
				lg := math.Sqrt(n[0]*n[0] + n[1]*n[1] + n[2]*n[2])
				if lg == 0 {
					lg = 1
				}
				at := 0
				for _, x := range [...]float64{n[0] / lg, n[1] / lg, n[2] / lg, pts[0][0], pts[0][1], pts[0][2], pts[1][0], pts[1][1], pts[1][2], pts[2][0], pts[2][1], pts[2][2]} {
					binary.LittleEndian.PutUint32(rec[at:], math.Float32bits(float32(x)))
					at += 4
				}
				binary.LittleEndian.PutUint16(rec[48:], 0)
				out.Write(rec[:])
			}
		}
	}
	return out.Bytes(), nil
}
