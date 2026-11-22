import { globalVars } from "@/utils/constants.js";

let workerPromise: Promise<Worker> | null = null;
let messageIdCounter = 0;
const pendingRequests = new Map<number, {
  resolve: (val: unknown) => void;
  reject: (err: unknown) => void;
  onProgress?: (msg: string, kind: string) => void;
}>();

async function createWorker(): Promise<Worker> {
  const base = `${globalVars?.baseURL || ""}public/static`;
  // The worker is not content-hashed, so version its URL with this bundle's
  // own hash: a stale cached worker never outlives its protocol.
  const bundleHash = (import.meta.url.match(/-([A-Za-z0-9_-]+)\.js$/) || [])[1] || "";
  const version = bundleHash ? `?v=${bundleHash}` : "";
  const workerUrl = `${base}/wasm/cadWorker.js${version}`;

  // The upstream CSP only allows blob: workers, so load the script as a static
  // asset and instantiate it from a Blob instead of a same-origin URL.
  const response = await fetch(workerUrl);
  if (!response.ok) throw new Error(`Unable to load CAD worker (${response.status})`);
  const blobUrl = URL.createObjectURL(new Blob([await response.text()], { type: "text/javascript" }));
  const worker = new Worker(blobUrl);
  URL.revokeObjectURL(blobUrl);

  worker.onmessage = (e) => {
    const { id, type, error, text, kind, ...rest } = e.data;
    const req = pendingRequests.get(id);
    if (!req) return;

    if (type === "log") {
      if (req.onProgress) req.onProgress(text, kind);
      return;
    }

    pendingRequests.delete(id);
    if (type === "error" || error) {
      req.reject(new Error(error || "CAD processing error"));
    } else {
      req.resolve(rest);
    }
  };

  worker.onerror = (e) => {
    console.error("[CAD Worker Error]", e);
    for (const [id, req] of pendingRequests.entries()) {
      req.reject(new Error(e.message || "CAD Web Worker crashed"));
      pendingRequests.delete(id);
    }
    // The worker is dead; drop it so the next request spins up a fresh one.
    worker.terminate();
    workerPromise = null;
  };

  worker.postMessage({
    type: "init",
    openswZipUrl: `${base}/wasm/opensw.zip`,
    goOpenSwUrl: `${base}/wasm/opensw.wasm${version}`,
    pyodideUrl: "https://cdn.jsdelivr.net/pyodide/v0.29.5/full/",
    occtUrl: "https://cdn.jsdelivr.net/npm/occt-import-js@0.0.23/dist/",
  });

  return worker;
}

function getWorker(): Promise<Worker> {
  if (!workerPromise) {
    workerPromise = createWorker().catch((error) => {
      workerPromise = null;
      throw error;
    });
  }
  return workerPromise;
}

export function isCad3DFile(filename: string): boolean {
  if (!filename) return false;
  return /\.(step|stp|x_t|x_b|sldprt|sldasm)$/i.test(filename);
}

export function isCadDrawingFile(filename: string): boolean {
  if (!filename) return false;
  return /\.slddrw$/i.test(filename);
}

export function isCadFile(filename: string): boolean {
  return isCad3DFile(filename) || isCadDrawingFile(filename);
}

export interface CadConversionTarget {
  format: string;
  label: string;
  description: string;
  icon: string;
}

export function getCadConversionTargets(filename: string): CadConversionTarget[] {
  const ext = (filename.split(".").pop() || "").toLowerCase();
  switch (ext) {
    case "sldprt":
      return [
        { format: "step", label: "STEP (.STEP)", description: "Standard CAD exchange solid", icon: "view_in_ar" },
        { format: "x_t", label: "Parasolid (.x_t)", description: "Parasolid text transmit file", icon: "category" },
      ];
    case "x_t":
    case "x_b":
      return [
        { format: "step", label: "STEP (.STEP)", description: "Standard STEP B-Rep solid", icon: "view_in_ar" },
      ];
    case "sldasm":
      return [
        { format: "stl", label: "STL (.stl)", description: "3D mesh in millimeters", icon: "view_in_ar" },
        { format: "csv", label: "Bill of Materials (.csv)", description: "Component and parts list", icon: "table_chart" },
      ];
    case "slddrw":
      return [
        { format: "svg", label: "SVG Vector (.svg)", description: "Current sheet as scalable vector", icon: "image" },
        { format: "dxf", label: "AutoCAD DXF (.dxf)", description: "AutoCAD R12 DXF drawing", icon: "architecture" },
      ];
    default:
      return [];
  }
}

export interface CadMeshResult {
  faces: Array<{ nom: string; corps: number; triangles: number; couleur?: number[] }>;
  corps: number;
  rayon: number;
  sommets: ArrayBuffer;
  centre?: number[];
}

export interface CadStoredMass {
  centerM: number[];
  volumeM3: number;
  areaM2: number;
  massKg: number;
  inertiaKgM2?: number[];
}

export interface CadStoredProperties {
  available: boolean;
  configuration?: string;
  material?: string;
  materialBase?: string;
  mass?: CadStoredMass;
}

export interface Cad3DResult {
  mesh: CadMeshResult;
  stepBytes?: Uint8Array;
  stlBytes?: Uint8Array;
  composants?: Array<Record<string, unknown>>;
  bom?: CadBomRow[];
  bomCsv?: string;
  properties?: CadStoredProperties[];
  displayMesh?: boolean;
}

export interface CadBomRow {
  file: string;
  configuration: string;
  quantity: number;
  instances: string[];
}

export interface DrawingSheet {
  nom: string;
  active: boolean;
  svg: string;
  elements: number;
  vues: Array<{ nom: string; echelle: number; aretes: number }>;
}

export interface CadDrawingResult {
  plan: {
    feuilles: DrawingSheet[];
    non_lus?: string[];
  };
  dxfBytes?: Uint8Array;
}

export function loadCadProperties(
  dataBuffer: ArrayBuffer,
  filename: string,
  onProgress?: (msg: string, kind?: string) => void,
): Promise<{ properties: CadStoredProperties[] }> {
  const id = ++messageIdCounter;
  const copy = dataBuffer.slice(0);
  return getWorker().then((worker) => new Promise((resolve, reject) => {
    pendingRequests.set(id, { resolve: resolve as (value: unknown) => void, reject, onProgress });
    worker.postMessage({ id, action: "properties", filename, data: copy }, [copy]);
  }));
}

function requestCad3D(
  dataBuffer: ArrayBuffer,
  filename: string,
  onProgress?: (msg: string, kind?: string) => void,
  brep = false
): Promise<Cad3DResult> {
  const id = ++messageIdCounter;
  const copy = dataBuffer.slice(0);

  return getWorker().then((worker) => new Promise((resolve, reject) => {
    pendingRequests.set(id, { resolve: resolve as (value: unknown) => void, reject, onProgress });
    worker.postMessage(
      {
        id,
        action: "preview",
        filename,
        brep,
        data: copy,
      },
      [copy]
    );
  }));
}

export function loadCad3D(
  dataBuffer: ArrayBuffer,
  filename: string,
  onProgress?: (msg: string, kind?: string) => void
): Promise<Cad3DResult> {
  return requestCad3D(dataBuffer, filename, onProgress);
}

export function loadCadBRep(
  dataBuffer: ArrayBuffer,
  filename: string,
  onProgress?: (msg: string, kind?: string) => void
): Promise<Cad3DResult> {
  return requestCad3D(dataBuffer, filename, onProgress, true);
}

export function loadCadDrawing(
  dataBuffer: ArrayBuffer,
  filename: string,
  onProgress?: (msg: string, kind?: string) => void
): Promise<CadDrawingResult> {
  const id = ++messageIdCounter;
  const copy = dataBuffer.slice(0);

  return getWorker().then((worker) => new Promise((resolve, reject) => {
    pendingRequests.set(id, { resolve: resolve as (value: unknown) => void, reject, onProgress });
    worker.postMessage(
      {
        id,
        action: "preview",
        filename,
        data: copy,
      },
      [copy]
    );
  }));
}

export function convertCadFile(
  dataBuffer: ArrayBuffer,
  filename: string,
  targetFormat: string,
  onProgress?: (msg: string, kind?: string) => void
): Promise<{ filename: string; data: Uint8Array }> {
  const id = ++messageIdCounter;
  const copy = dataBuffer.slice(0);

  return getWorker().then((worker) => new Promise((resolve, reject) => {
    pendingRequests.set(id, { resolve: resolve as (value: unknown) => void, reject, onProgress });
    worker.postMessage(
      {
        id,
        action: "convert",
        filename,
        targetFormat,
        data: copy,
      },
      [copy]
    );
  }));
}
