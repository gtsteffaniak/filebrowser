// cadWorker.js - Unified Web Worker for CAD file previews & conversions
// Uses OpenSW-Go for fast previews; Pyodide is loaded only for exact B-rep or
// CAD operations that have not yet been ported.


let PYODIDE = "https://cdn.jsdelivr.net/pyodide/v0.29.5/full/";
let OCCT = "https://cdn.jsdelivr.net/npm/occt-import-js@0.0.23/dist/";
let OPENSW_ZIP = "/public/static/wasm/opensw.zip";
let GO_OPENSW = "/public/static/wasm/opensw.wasm";

let pythonPromise = null;
let occtPromise = null;
let goPromise = null;

function log(dire, text, kind = "detail") {
  if (dire) dire(text, kind);
}

function chargerOcct(dire) {
  if (!occtPromise) {
    occtPromise = (async () => {
      log(dire, "Loading OpenCASCADE WebAssembly engine...", "step");
      importScripts(OCCT + "occt-import-js.js");
      const occt = await occtimportjs({ locateFile: (f) => OCCT + f });
      log(dire, "OpenCASCADE WASM engine ready", "ok");
      return occt;
    })();
    // A failed load must not stay cached as a rejected promise: clear it so
    // the next request can retry (same behavior as chargerGo).
    occtPromise.catch(() => { occtPromise = null; });
  }
  return occtPromise;
}

function preparerPython(dire) {
  if (!pythonPromise) {
    pythonPromise = (async () => {
      const t0 = performance.now();
      log(dire, "Loading Python WebAssembly (Pyodide)...", "step");
      importScripts(`${PYODIDE}pyodide.js`);
      const py = await loadPyodide({ indexURL: PYODIDE });
      log(dire, "Fetching OpenSW CAD engine...", "step");
      const archive = await fetch(OPENSW_ZIP).then((r) => {
        if (!r.ok) throw new Error(`Failed to load opensw.zip: HTTP ${r.status}`);
        return r.arrayBuffer();
      });
      py.unpackArchive(archive, "zip", { extractDir: "/opensw" });
      py.runPython(`
import sys, types
sys.path[:0] = ["/opensw/tools", "/opensw/src"]
for nom in ("webbrowser", "signal", "http.server", "socketserver"):
    try:
        __import__(nom)
    except Exception:
        sys.modules[nom] = types.ModuleType(nom)
`);
      try {
        await py.loadPackage("micropip");
        await py.pyimport("micropip").install("olefile");
      } catch (_) {}
      const nav = py.pyimport("navigateur");
      log(dire, `OpenSW engine loaded in ${((performance.now() - t0) / 1000).toFixed(1)}s`, "ok");
      return { nav, py };
    })();
    // Clear a failed startup so the next request can retry without a reload.
    pythonPromise.catch(() => { pythonPromise = null; });
  }
  return pythonPromise;
}

const NIVEAUX = 64;

function buildBom(components) {
  const rows = new Map();
  for (const component of components || []) {
    if (!component || typeof component !== "object") continue;
    const excluded = component.horsNomenclature ?? component.hors_nomenclature;
    const isFlagSet = (value) => value === true || value === 1 || value === "YES";
    if (isFlagSet(component.supprime) || isFlagSet(excluded)) continue;

    const file = String(component.fichier || component.nom || "").trim();
    if (!file) continue;
    const configuration = String(component.configuration || "").trim();
    const key = `${file.toLowerCase()}\0${configuration.toLowerCase()}`;
    let row = rows.get(key);
    if (!row) {
      row = { file, configuration, quantity: 0, instances: [] };
      rows.set(key, row);
    }
    row.quantity++;
    if (component.nom) row.instances.push(String(component.nom));
  }
  return [...rows.values()].sort((a, b) =>
    a.file.localeCompare(b.file) || a.configuration.localeCompare(b.configuration)
  );
}

function bomRowCsv(row) {
  const field = (v) => `"${String(v ?? "").replace(/"/g, '""')}"`;
  return `${field(row.file)},${field(row.configuration)},${row.quantity},${field(row.instances.join("; "))}\n`;
}

function bomCsv(rows) {
  let csv = "Part,Configuration,Quantity,Instances\n";
  for (const row of rows || []) csv += bomRowCsv(row);
  return csv;
}

// The OpenSW port compiled to Go/WebAssembly; loading it does not start Python.
function chargerGo(dire) {
  if (!goPromise) {
    goPromise = (async () => {
      const t0 = performance.now();
      log(dire, "Loading OpenSW-Go WebAssembly engine...", "step");
      const base = GO_OPENSW.slice(0, GO_OPENSW.lastIndexOf("/") + 1);
      importScripts(`${base}wasm_exec.js`);
      const ready = new Promise((resolve) => { self.__openswGoResolve = resolve; });
      const go = new Go();
      const resp = await fetch(GO_OPENSW);
      if (!resp.ok) throw new Error(`Failed to load opensw.wasm: HTTP ${resp.status}`);
      const bytes = await resp.arrayBuffer();
      const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
      go.run(instance); // resolves when Go's main signals readiness
      await ready;
      log(dire, `OpenSW-Go engine ready in ${((performance.now() - t0) / 1000).toFixed(2)}s`, "ok");
      return true;
    })();
    goPromise.catch(() => { goPromise = null; });
  }
  return goPromise;
}

// Assembly preview through the Go engine; throws when it cannot read the file.
async function vueAssemblageGo(octets, dire) {
  await chargerGo(dire);
  const r = openswGoAssembly(octets, (text, kind) => dire(text, kind));
  if (r?.ok !== true) {
    throw new Error((r?.error) || "OpenSW-Go could not read the assembly");
  }
  const vue = r.vue;
  const mesh = {
    faces: vue.faces,
    corps: vue.corps,
    rayon: vue.rayon,
    sommets: vue.sommets.buffer,
  };
  return { mesh, stlBytes: r.stl ? new Uint8Array(r.stl) : null, composants: r.composants };
}

const srgb = (x) => (x <= 0.0031308 ? x * 12.92 : 1.055 * x ** (1 / 2.4) - 0.055);
function rang(c) {
  if (!c) return -1;
  for (const lu of [c.map(srgb), c]) {
    const v = lu.map((x) => x * (NIVEAUX - 1));
    const r = v.map(Math.round);
    if (v.every((x, i) => Math.abs(x - r[i]) <= 0.02))
      return (r[0] * NIVEAUX + r[1]) * NIVEAUX + r[2];
  }
  return -1;
}

async function mailler(octets, table, dire) {
  const t0 = performance.now();
  const o = await chargerOcct(dire);
  const res = o.ReadStepFile(octets, {
    linearUnit: "millimeter",
    linearDeflectionType: "bounding_box_ratio",
    linearDeflection: 0.001,
    angularDeflection: 0.35,
  });
  if (!res.success) throw new Error("OpenCASCADE could not read the STEP model");

  const min = [Infinity, Infinity, Infinity], max = [-Infinity, -Infinity, -Infinity];
  let total = 0;
  for (const m of res.meshes) {
    const p = m.attributes.position.array;
    for (let i = 0; i < p.length; i += 3) {
      for (let k = 0; k < 3; k++) {
        if (p[i + k] < min[k]) min[k] = p[i + k];
        if (p[i + k] > max[k]) max[k] = p[i + k];
      }
    }
    total += m.index.array.length / 3;
  }
  if (!total) throw new Error("The model contains no mesh geometry");

  const centre = min.map((v, k) => (v + max[k]) / 2);
  const sommets = new Float32Array(total * 9);
  const faces = [];
  let s = 0, corpsMax = 0;

  res.meshes.forEach((m, rangMaillage) => {
    const p = m.attributes.position.array;
    const ix = m.index.array;
    for (const bf of m.brep_faces) {
      const k = rang(bf.color);
      const f = k >= 0 && k < table.length ? table[k] : null;
      for (let t = bf.first; t <= bf.last; t++) {
        for (let c = 0; c < 3; c++) {
          const v = ix[3 * t + c] * 3;
          for (let a = 0; a < 3; a++) sommets[s++] = p[v + a] - centre[a];
        }
      }
      const face = {
        nom: f ? f.nom || "?" : `Face ${faces.length + 1}`,
        corps: f ? f.corps : rangMaillage + 1,
        triangles: bf.last - bf.first + 1,
      };
      if (f?.couleur) face.couleur = f.couleur;
      corpsMax = Math.max(corpsMax, face.corps);
      faces.push(face);
    }
  });

  const diagonale = Math.hypot(max[0] - min[0], max[1] - min[1], max[2] - min[2]) || 1;
  log(dire, `3D Model ready: ${faces.length} faces, ${corpsMax} bodies in ${((performance.now() - t0) / 1000).toFixed(1)}s`, "ok");
  return {
    faces,
    corps: corpsMax,
    rayon: diagonale / 2,
    sommets: sommets.buffer,
    centre,
  };
}

async function maillerStepDirect(octets, dire) {
  const t0 = performance.now();
  const o = await chargerOcct(dire);
  const res = o.ReadStepFile(octets, {
    linearUnit: "millimeter",
    linearDeflectionType: "bounding_box_ratio",
    linearDeflection: 0.001,
    angularDeflection: 0.35,
  });
  if (!res.success) throw new Error("OpenCASCADE could not read the STEP file");

  const min = [Infinity, Infinity, Infinity], max = [-Infinity, -Infinity, -Infinity];
  let total = 0;
  for (const m of res.meshes) {
    const p = m.attributes.position.array;
    for (let i = 0; i < p.length; i += 3) {
      for (let k = 0; k < 3; k++) {
        if (p[i + k] < min[k]) min[k] = p[i + k];
        if (p[i + k] > max[k]) max[k] = p[i + k];
      }
    }
    total += m.index ? m.index.array.length / 3 : m.attributes.position.array.length / 9;
  }
  if (!total) throw new Error("The STEP file contains no geometry");

  const centre = min.map((v, k) => (v + max[k]) / 2);
  const sommets = new Float32Array(total * 9);
  const faces = [];
  let s = 0;

  res.meshes.forEach((m, rangMaillage) => {
    const p = m.attributes.position.array;
    const ix = m.index ? m.index.array : null;
    const color = m.color ? [m.color[0], m.color[1], m.color[2]] : [0.65, 0.72, 0.85];

    if (m.brep_faces && m.brep_faces.length > 0 && ix) {
      for (const bf of m.brep_faces) {
        for (let t = bf.first; t <= bf.last; t++) {
          for (let c = 0; c < 3; c++) {
            const v = ix[3 * t + c] * 3;
            for (let a = 0; a < 3; a++) sommets[s++] = p[v + a] - centre[a];
          }
        }
        faces.push({
          nom: `Body ${rangMaillage + 1}`,
          corps: rangMaillage + 1,
          triangles: bf.last - bf.first + 1,
          couleur: bf.color ? bf.color.slice(0, 3) : color,
        });
      }
    } else if (ix) {
      for (let i = 0; i < ix.length; i++) {
        const v = ix[i] * 3;
        for (let a = 0; a < 3; a++) sommets[s++] = p[v + a] - centre[a];
      }
      faces.push({
        nom: `Body ${rangMaillage + 1}`,
        corps: rangMaillage + 1,
        triangles: ix.length / 3,
        couleur: color,
      });
    } else {
      for (let i = 0; i < p.length; i += 3) {
        for (let a = 0; a < 3; a++) sommets[s++] = p[i + a] - centre[a];
      }
      faces.push({
        nom: `Body ${rangMaillage + 1}`,
        corps: rangMaillage + 1,
        triangles: p.length / 9,
        couleur: color,
      });
    }
  });

  const diagonale = Math.hypot(max[0] - min[0], max[1] - min[1], max[2] - min[2]) || 1;
  log(dire, `STEP model meshed: ${faces.length} parts in ${((performance.now() - t0) / 1000).toFixed(1)}s`, "ok");
  return {
    faces,
    corps: res.meshes.length,
    rayon: diagonale / 2,
    sommets: sommets.buffer,
    centre,
  };
}

self.onmessage = async (e) => {
  const m = e.data;
  if (!m) return;

  if (m.type === "init") {
    if (m.pyodideUrl) PYODIDE = m.pyodideUrl;
    if (m.occtUrl) OCCT = m.occtUrl;
    if (m.openswZipUrl) OPENSW_ZIP = m.openswZipUrl;
    if (m.goOpenSwUrl) GO_OPENSW = m.goOpenSwUrl;
    return;
  }

  const { id, action, format, filename, data, targetFormat, brep } = m;
  const dire = (text, kind = "detail") => {
    postMessage({ id, type: "log", text, kind });
  };

  try {
    const rawBytes = new Uint8Array(data);
    const ext = (format || filename.split(".").pop() || "").toLowerCase();

    if (action === "properties") {
      dire("Reading SolidWorks stored properties with OpenSW-Go...", "step");
      try {
        await chargerGo(dire);
        const p = openswGoProperties(rawBytes);
        if (p?.ok) {
          postMessage({ id, type: "properties", properties: p.properties });
          return;
        }
      } catch (e) {
        dire(`Go properties unavailable: ${e.message}`, "detail");
      }
      throw new Error("SolidWorks stored properties are unavailable for this file");
    }

    if (action === "preview") {
      // Direct STEP preview (pure C++/WASM, no Python needed)
      if (ext === "step" || ext === "stp") {
        dire("Meshing STEP with OpenCASCADE WASM...", "step");
        const mesh = await maillerStepDirect(rawBytes, dire);
        postMessage({ id, type: "preview3d", mesh }, [mesh.sommets]);
        return;
      }

      // The embedded display mesh is the fast default; exact B-rep loading is
      // an explicit user action so Pyodide is never started for this route.
      if (ext === "sldprt" && !brep) {
        dire("Reading the SolidWorks display mesh with OpenSW-Go...", "step");
        await chargerGo(dire);
        const r = openswGoPart(rawBytes, (text, kind) => dire(text, kind));
        if (r?.ok !== true || !r.vue) {
          throw new Error(r?.error || "No SolidWorks display mesh found in part");
        }
        const mesh = {
          faces: r.vue.faces,
          corps: r.vue.corps,
          rayon: r.vue.rayon,
          sommets: r.vue.sommets.buffer,
        };
        postMessage({ id, type: "preview3d", mesh, displayMesh: true }, [mesh.sommets]);
        return;
      }

      // Drawing preview (.SLDDRW)
      if (ext === "slddrw") {
        dire("Reading SolidWorks Drawing with OpenSW...", "step");
        const { nav } = await preparerPython(dire);
        const r = nav.lancer("plan", filename, rawBytes, (json) => {
          const l = JSON.parse(json);
          dire(l.text, l.kind);
        }).toJs({ dict_converter: Object.fromEntries });

        const plan = r.plan ? JSON.parse(r.plan) : null;
        let dxfBytes = null;
        if (r.resultat) {
          dxfBytes = new Uint8Array(r.resultat);
        }
        postMessage({ id, type: "previewDrawing", plan, dxfBytes }, dxfBytes ? [dxfBytes.buffer] : []);
        return;
      }

      // Assembly preview (.SLDASM) — instant-start Go engine first
      if (ext === "sldasm") {
        try {
          const r = await vueAssemblageGo(rawBytes, dire);
          const transfer = [r.mesh.sommets];
          if (r.stlBytes) transfer.push(r.stlBytes.buffer);
          const bom = buildBom(r.composants);
          postMessage({
            id,
            type: "preview3d",
            mesh: r.mesh,
            stlBytes: r.stlBytes,
            composants: r.composants,
            bom,
            bomCsv: bomCsv(bom),
          }, transfer);
          return;
        } catch (e) {
          dire(`Go engine: ${e.message} — falling back to Python OpenSW...`, "detail");
        }
        dire("Reading SolidWorks Assembly with OpenSW...", "step");
        const { nav } = await preparerPython(dire);
        const r = nav.lancer("assemblage", filename, rawBytes, (json) => {
          const l = JSON.parse(json);
          dire(l.text, l.kind);
        }).toJs({ dict_converter: Object.fromEntries });

        if (r.vue) {
          const vue = JSON.parse(r.vue);
          const brut = atob(vue.sommets);
          const octets = new Uint8Array(brut.length);
          for (let i = 0; i < brut.length; i++) octets[i] = brut.charCodeAt(i);
          const sommets = new Float32Array(octets.buffer);
          const stlBytes = r.resultat ? new Uint8Array(r.resultat) : null;
          const mesh = {
            faces: vue.faces,
            corps: vue.corps,
            rayon: vue.rayon,
            sommets: sommets.buffer,
          };
          const transfer = [sommets.buffer];
          if (stlBytes) transfer.push(stlBytes.buffer);
          const bom = buildBom(r.composants);
          postMessage({
            id,
            type: "preview3d",
            mesh,
            stlBytes,
            composants: r.composants,
            bom,
            bomCsv: bomCsv(bom),
          }, transfer);
          return;
        } else {
          throw new Error("No 3D display mesh found in assembly");
        }
      }

      // SolidWorks Part (.SLDPRT) or Parasolid (.x_t, .x_b)
      if ((ext === "sldprt" && brep) || ext === "x_t" || ext === "x_b") {
        let stepError = null;
        try {
          dire(`Converting ${ext.toUpperCase()} to STEP with OpenSW...`, "step");
          const { nav } = await preparerPython(dire);
          const r = nav.lancer("step", filename, rawBytes, (json) => {
            const l = JSON.parse(json);
            dire(l.text, l.kind);
          }).toJs({ dict_converter: Object.fromEntries });

          if (r.etiquete) {
            dire("Meshing STEP in browser with OpenCASCADE...", "step");
            const facesTable = JSON.parse(r.faces || "[]");
            const mesh = await mailler(new Uint8Array(r.etiquete), facesTable, dire);
            const stepBytes = r.resultat ? new Uint8Array(r.resultat) : null;
            const transfer = [mesh.sommets];
            if (stepBytes) transfer.push(stepBytes.buffer);
            postMessage({ id, type: "preview3d", mesh, stepBytes }, transfer);
            return;
          } else if (r.resultat) {
            dire("Meshing converted STEP model...", "step");
            const stepBytes = new Uint8Array(r.resultat);
            const mesh = await maillerStepDirect(stepBytes, dire);
            postMessage({ id, type: "preview3d", mesh, stepBytes }, [mesh.sommets, stepBytes.buffer]);
            return;
          }
          stepError = new Error("Failed to extract geometry from CAD file");
        } catch (e) {
          stepError = e;
        }

        throw stepError;
      }

      throw new Error(`Unsupported CAD format: .${ext}`);
    }

    if (action === "convert") {
      const target = (targetFormat || "").toLowerCase();
      dire(`Converting ${filename} to .${target}...`, "step");

      // Part conversions
      if (ext === "sldprt") {
        const { nav } = await preparerPython(dire);
        const route = target === "x_t" ? "parasolid" : "step";
        const r = nav.lancer(route, filename, rawBytes, (json) => {
          const l = JSON.parse(json);
          dire(l.text, l.kind);
        }).toJs({ dict_converter: Object.fromEntries });

        if (!r.resultat) throw new Error("Conversion generated no output");
        const resultat = new Uint8Array(r.resultat);
        postMessage({ id, type: "converted", filename: r.fichier, data: resultat }, [resultat.buffer]);
        return;
      }

      // Parasolid to STEP
      if (ext === "x_t" || ext === "x_b") {
        const { nav } = await preparerPython(dire);
        const r = nav.lancer("step", filename, rawBytes, (json) => {
          const l = JSON.parse(json);
          dire(l.text, l.kind);
        }).toJs({ dict_converter: Object.fromEntries });

        if (!r.resultat) throw new Error("Conversion generated no output");
        const resultat = new Uint8Array(r.resultat);
        postMessage({ id, type: "converted", filename: r.fichier, data: resultat }, [resultat.buffer]);
        return;
      }

      // Assembly conversions — STL and BOM through the Go engine first
      if (ext === "sldasm" && (target === "stl" || target === "csv")) {
        try {
          const r = await vueAssemblageGo(rawBytes, dire);
          if (target === "stl") {
            if (!r.stlBytes) throw new Error("Failed to generate STL from assembly");
            postMessage({ id, type: "converted", filename: filename.replace(/\.sldasm$/i, ".stl"), data: r.stlBytes }, [r.stlBytes.buffer]);
            return;
          }
          const enc = new TextEncoder().encode(bomCsv(buildBom(r.composants)));
          const outName = filename.replace(/\.sldasm$/i, "_BOM.csv");
          postMessage({ id, type: "converted", filename: outName, data: enc }, [enc.buffer]);
          return;
        } catch (e) {
          dire(`Go engine: ${e.message} — falling back to Python OpenSW...`, "detail");
        }
      }

      if (ext === "sldasm") {
        const { nav } = await preparerPython(dire);
        const r = nav.lancer("assemblage", filename, rawBytes, (json) => {
          const l = JSON.parse(json);
          dire(l.text, l.kind);
        }).toJs({ dict_converter: Object.fromEntries });

        if (target === "stl") {
          if (!r.resultat) throw new Error("Failed to generate STL from assembly");
          const resultat = new Uint8Array(r.resultat);
          postMessage({ id, type: "converted", filename: r.fichier, data: resultat }, [resultat.buffer]);
          return;
        } else if (target === "csv") {
          // Bill of materials CSV
          const enc = new TextEncoder().encode(bomCsv(buildBom(r.composants)));
          const outName = filename.replace(/\.sldasm$/i, "_BOM.csv");
          postMessage({ id, type: "converted", filename: outName, data: enc }, [enc.buffer]);
          return;
        }
      }

      // Drawing conversions
      if (ext === "slddrw") {
        const { nav } = await preparerPython(dire);
        const r = nav.lancer("plan", filename, rawBytes, (json) => {
          const l = JSON.parse(json);
          dire(l.text, l.kind);
        }).toJs({ dict_converter: Object.fromEntries });

        if (target === "dxf") {
          if (!r.resultat) throw new Error("Failed to generate DXF from drawing");
          const resultat = new Uint8Array(r.resultat);
          postMessage({ id, type: "converted", filename: r.fichier, data: resultat }, [resultat.buffer]);
          return;
        } else if (target === "svg") {
          const plan = r.plan ? JSON.parse(r.plan) : null;
          if (!plan?.feuilles?.length) throw new Error("No SVG sheets found in drawing");
          const activeSheet = plan.feuilles.find((f) => f.active) || plan.feuilles[0];
          const enc = new TextEncoder().encode(activeSheet.svg);
          const outName = `${filename.replace(/\.slddrw$/i, '')}_${activeSheet.nom}.svg`;
          postMessage({ id, type: "converted", filename: outName, data: enc }, [enc.buffer]);
          return;
        }
      }

      throw new Error(`Cannot convert ${ext} to ${target}`);
    }

    throw new Error(`Unknown action: ${action}`);
  } catch (err) {
    postMessage({ id, type: "error", error: String(err.message || err) });
  }
};
