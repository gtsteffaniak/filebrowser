//go:build js && wasm

// Command opensw-wasm is the OpenSW CAD reader compiled to WebAssembly: the
// Go port of the SolidWorks container readers, served to the browser worker
// for instant-start assembly previews (no Python runtime involved).
package main

import (
	"syscall/js"
	"unsafe"

	"opensw/internal/opensw"
)

func jsObj(pairs ...any) js.Value {
	o := js.Global().Get("Object").New()
	for i := 0; i+1 < len(pairs); i += 2 {
		o.Set(pairs[i].(string), jsValue(pairs[i+1]))
	}
	return o
}

func jsValue(v any) js.Value {
	switch x := v.(type) {
	case js.Value:
		return x
	case int:
		return js.ValueOf(x)
	case float64:
		return js.ValueOf(x)
	case bool:
		return js.ValueOf(x)
	case string:
		return js.ValueOf(x)
	case []byte:
		return bytesToJS(x)
	case []js.Value:
		arr := js.Global().Get("Array").New(len(x))
		for i, e := range x {
			arr.SetIndex(i, e)
		}
		return arr
	}
	panic("jsValue: unsupported type")
}

func bytesToJS(b []byte) js.Value {
	u8 := js.Global().Get("Uint8Array").New(len(b))
	if len(b) > 0 {
		js.CopyBytesToJS(u8, b)
	}
	return u8
}

func float32ArrayToJS(vs []float32) js.Value {
	u8 := js.Global().Get("Uint8Array").New(len(vs) * 4)
	if len(vs) > 0 {
		raw := unsafe.Slice((*byte)(unsafe.Pointer(&vs[0])), len(vs)*4)
		js.CopyBytesToJS(u8, raw)
	}
	return js.Global().Get("Float32Array").New(u8.Get("buffer"), 0, len(vs))
}

func viewFacesToJS(faces []opensw.ViewFace) []js.Value {
	out := make([]js.Value, 0, len(faces))
	for _, f := range faces {
		face := jsObj("nom", f.Name, "corps", f.Body, "triangles", f.Triangles)
		if len(f.Color) == 3 {
			color := js.Global().Get("Array").New(3)
			for i, channel := range f.Color {
				color.SetIndex(i, js.ValueOf(channel))
			}
			face.Set("couleur", color)
		}
		out = append(out, face)
	}
	return out
}

func jsBytes(v js.Value) []byte {
	if !v.Truthy() || v.Length() == 0 {
		return nil
	}
	out := make([]byte, v.Length())
	js.CopyBytesToGo(out, v)
	return out
}

func main() {
	js.Global().Set("openswGoAssembly", js.FuncOf(assemblyView))
	js.Global().Set("openswGoPart", js.FuncOf(partDisplayMesh))
	js.Global().Set("openswGoProperties", js.FuncOf(storedProperties))
	js.Global().Set("openswGoPreview", js.FuncOf(previewPNG))
	// Signal the host script that the globals exist (go.run is async).
	if resolve := js.Global().Get("__openswGoResolve"); resolve.Truthy() {
		resolve.Invoke()
	}
	select {}
}

func storedProperties(_ js.Value, args []js.Value) any {
	c, err := opensw.OpenContainer(jsBytes(args[0]))
	if err != nil {
		return jsObj("ok", false, "error", err.Error())
	}
	props, err := c.ReadStoredProperties()
	if err != nil {
		return jsObj("ok", false, "error", err.Error())
	}
	items := make([]js.Value, 0, len(props))
	for _, p := range props {
		obj := jsObj("available", p.Available, "configuration", p.Configuration,
			"material", p.Material, "materialBase", p.MaterialBase)
		if p.Mass != nil {
			center := []js.Value{js.ValueOf(p.Mass.CenterM[0]), js.ValueOf(p.Mass.CenterM[1]), js.ValueOf(p.Mass.CenterM[2])}
			inertia := make([]js.Value, len(p.Mass.InertiaKgM2))
			for i, v := range p.Mass.InertiaKgM2 {
				inertia[i] = js.ValueOf(v)
			}
			obj.Set("mass", jsObj("centerM", center, "volumeM3", p.Mass.VolumeM3,
				"areaM2", p.Mass.AreaM2, "massKg", p.Mass.MassKg, "inertiaKgM2", inertia))
		}
		items = append(items, obj)
	}
	return jsObj("ok", true, "properties", items)
}

// assemblyView(data Uint8Array, dire(text, kind)) reads a SolidWorks
// assembly and returns {ok, vue, composants, stl?} — the same shape the
// Python engine's "assemblage" route gives the worker.
func assemblyView(_ js.Value, args []js.Value) any {
	dire := args[1]
	say := func(text, kind string) {
		if dire.Truthy() {
			dire.Invoke(text, kind)
		}
	}
	say("Reading SolidWorks assembly (OpenSW-Go engine)...", "step")
	asm, err := opensw.ReadAssembly(jsBytes(args[0]))
	if err != nil {
		return jsObj("ok", false, "error", err.Error())
	}
	say("Placing components and display mesh...", "detail")
	view, err := asm.ToView()
	if err != nil {
		return jsObj("ok", false, "error", err.Error())
	}
	faces := viewFacesToJS(view.Faces)
	comps := make([]js.Value, 0, len(asm.Components))
	for _, c := range asm.Components {
		comps = append(comps, jsObj(
			"nom", c.Name,
			"fichier", c.File,
			"supprime", c.Suppressed,
			"cache", c.Hidden,
			"horsNomenclature", c.ExcludeFromBOM,
			"configuration", c.Config,
		))
	}
	result := jsObj(
		"ok", true,
		"vue", jsObj(
			"faces", faces,
			"corps", view.Bodies,
			"rayon", view.Radius,
			"sommets", float32ArrayToJS(view.Vertices),
		),
		"composants", comps,
	)
	if stl, err := asm.ToSTL(); err == nil {
		result.Set("stl", bytesToJS(stl))
	}
	say("Assembly ready (OpenSW-Go)", "ok")
	return result
}

// previewPNG(data Uint8Array) extracts the embedded preview image, for
// callers without the server-side extraction at hand.
func previewPNG(_ js.Value, args []js.Value) any {
	png, err := opensw.ExtractSolidWorksPreviewBytes(jsBytes(args[0]))
	if err != nil {
		return jsObj("ok", false, "error", err.Error())
	}
	return jsObj("ok", true, "png", bytesToJS(png))
}

// partDisplayMesh(data Uint8Array, dire(text, kind)) reads a SolidWorks
// part's embedded display mesh — the fallback view when the exact B-rep →
// STEP conversion cannot be written.
func partDisplayMesh(_ js.Value, args []js.Value) any {
	dire := args[1]
	say := func(text, kind string) {
		if dire.Truthy() {
			dire.Invoke(text, kind)
		}
	}
	say("Reading the SolidWorks display mesh (OpenSW-Go engine)...", "step")
	view, err := opensw.ReadPartDisplayMesh(jsBytes(args[0]))
	if err != nil {
		return jsObj("ok", false, "error", err.Error())
	}
	faces := viewFacesToJS(view.Faces)
	say("Part display mesh ready (OpenSW-Go)", "ok")
	return jsObj(
		"ok", true,
		"vue", jsObj(
			"faces", faces,
			"corps", view.Bodies,
			"rayon", view.Radius,
			"sommets", float32ArrayToJS(view.Vertices),
		),
	)
}
