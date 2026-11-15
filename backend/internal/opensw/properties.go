package opensw

import (
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf16"
)

// MassProperties are the values SolidWorks persisted, in SI units. They are
// intentionally not reconstructed from the display mesh.
type MassProperties struct {
	CenterM    [3]float64 `json:"centerM"`
	VolumeM3   float64   `json:"volumeM3"`
	AreaM2     float64   `json:"areaM2"`
	MassKg     float64   `json:"massKg"`
	InertiaKgM2 []float64 `json:"inertiaKgM2,omitempty"`
}

type StoredProperties struct {
	Available      bool            `json:"available"`
	Configuration  string          `json:"configuration,omitempty"`
	Mass           *MassProperties `json:"mass,omitempty"`
	Material       string          `json:"material,omitempty"`
	MaterialBase   string          `json:"materialBase,omitempty"`
}

const swPropertiesNS = "http://www.solidworks.com/sw2003/schema"

type propXML struct {
	XMLName  xml.Name
	Name     string `xml:"name,attr"`
	SWID     string `xml:"swID,attr"`
	Value    string `xml:"value,attr"`
	Text     string `xml:",chardata"`
	Children []propXML `xml:"property"`
}

type featureXML struct {
	XMLName xml.Name
	Name    string `xml:"swName,attr"`
	ID      string `xml:"swID,attr"`
	Child   []featureXML `xml:"swConfiguration"`
}

func collectNamedProperties(raw []byte) (map[string]string, error) {
	// SolidWorks' information document nests property nodes below sections;
	// use a token stream so the parser does not depend on a particular root.
	dec := xml.NewDecoder(strings.NewReader(string(raw)))
	values := map[string]string{}
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF { return values, nil }
			return nil, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "property" { continue }
		name := ""
		for _, a := range start.Attr { if a.Name.Local == "name" { name = a.Value } }
		if name == "" { continue }
		var value struct { Inner string `xml:",innerxml"` }
		if err := dec.DecodeElement(&value, &start); err != nil { return nil, err }
		inner := xml.NewDecoder(strings.NewReader(value.Inner))
		var text strings.Builder
		for {
			t, e := inner.Token()
			if e == io.EOF { break }
			if e != nil { return nil, e }
			if ch, ok := t.(xml.CharData); ok { text.Write([]byte(ch)) }
		}
		values[name] = strings.TrimSpace(text.String())
	}
}

func xmlPropertyValue(p propXML) string {
	if strings.TrimSpace(p.Text) != "" { return strings.TrimSpace(p.Text) }
	for _, c := range p.Children {
		if strings.TrimSpace(c.Text) != "" { return strings.TrimSpace(c.Text) }
	}
	return p.Value
}

func parseMass(raw string) *MassProperties {
	fields := strings.Split(raw, ",")
	if len(fields) < 6 { return nil }
	values := make([]float64, len(fields))
	for i, f := range fields {
		v, err := strconv.ParseFloat(strings.TrimSpace(f), 64)
		if err != nil { return nil }
		values[i] = v
	}
	m := &MassProperties{
		CenterM: [3]float64{values[0], values[1], values[2]},
		VolumeM3: values[3], AreaM2: values[4], MassKg: values[5],
	}
	if len(values) >= 12 { m.InertiaKgM2 = append([]float64(nil), values[6:12]...) }
	return m
}

func decodeUTF16(data []byte) string {
	if len(data) < 2 { return string(data) }
	u := make([]uint16, len(data)/2)
	for i := range u { u[i] = binary.LittleEndian.Uint16(data[2*i:]) }
	return string(utf16.Decode(u))
}

// ReadStoredProperties reads the same SolidWorks streams as OpenSW's
// properties.py. Missing mass properties remain unavailable.
func (c *Container) ReadStoredProperties() ([]StoredProperties, error) {
	infoRaw := c.Get("docProps/ISolidWorksInformation.xml")
	if infoRaw == nil { return nil, nil }
	values, err := collectNamedProperties(infoRaw)
	if err != nil { return nil, fmt.Errorf("opensw: properties XML: %w", err) }

	configs := map[string]string{}
	if raw := c.Get("swXmlContents/Features"); raw != nil {
		var root featureXML
		if xml.Unmarshal(raw, &root) == nil {
			var walk func(featureXML)
			walk = func(n featureXML) {
				if n.ID != "" && n.Name != "" { configs[n.ID] = n.Name }
				for _, ch := range n.Child { walk(ch) }
			}
			walk(root)
		}
	}
	if len(configs) == 0 {
		for name := range values {
			const prefix = "SW-MassProp-Config-"
			if strings.HasPrefix(name, prefix) { configs[strings.TrimPrefix(name, prefix)] = "" }
		}
	}
	out := make([]StoredProperties, 0, len(configs))
	for id, name := range configs {
		m := parseMass(values["SW-MassProp-Config-"+id])
		out = append(out, StoredProperties{Available: m != nil, Configuration: name, Mass: m})
	}
	return out, nil
}
