import { readFileSync } from "node:fs";
import path from "node:path";
import { runInNewContext } from "node:vm";
import { expect, it, vi } from "vitest";

const workerSource = readFileSync(path.resolve("public/wasm/cadWorker.js"), "utf8");

it("uses the Go display mesh for a default SLDPRT preview without loading Python", async () => {
  const messages = [];
  const importedScripts = [];
  const self = {};
  const context = {
    self,
    performance: { now: () => 1 },
    importScripts: (url) => {
      importedScripts.push(url);
      if (url.endsWith("wasm_exec.js")) {
        context.Go = class {
          importObject = {};

          run() {
            context.openswGoPart = () => ({
              ok: true,
              vue: {
                faces: [],
                corps: 1,
                rayon: 1,
                sommets: new Float32Array(0),
              },
            });
            self.__openswGoResolve();
          }
        };
      }
    },
    fetch: vi.fn(async () => ({
      ok: true,
      arrayBuffer: async () => new ArrayBuffer(0),
    })),
    WebAssembly: { instantiate: async () => ({ instance: {} }) },
    postMessage: (message) => messages.push(message),
  };
  runInNewContext(workerSource, context);

  await self.onmessage({
    data: { type: "init", goOpenSwUrl: "/public/static/wasm/opensw.wasm" },
  });
  await self.onmessage({
    data: {
      id: 1,
      action: "preview",
      filename: "part.sldprt",
      data: new ArrayBuffer(0),
    },
  });

  expect(importedScripts.some((url) => url.includes("pyodide"))).toBe(false);
  expect(messages.some((message) => message.type === "preview3d" && message.displayMesh)).toBe(true);
});

it("builds an assembly BOM grouped by part and configuration", async () => {
  const messages = [];
  const self = {};
  const components = [
    { nom: "bolt-1", fichier: "Bolt.SLDPRT", configuration: "Default" },
    { nom: "bolt-2", fichier: "Bolt.SLDPRT", configuration: "Default", cache: true },
    { nom: "bolt-3", fichier: "Bolt.SLDPRT", configuration: "Fine" },
    { nom: "bolt-4", fichier: "Bolt.SLDPRT", configuration: "Default", horsNomenclature: true },
    { nom: "plate-1", fichier: "Plate.SLDPRT", supprime: true },
  ];
  const context = {
    self,
    performance: { now: () => 1 },
    importScripts: (url) => {
      if (url.endsWith("wasm_exec.js")) {
        context.Go = class {
          importObject = {};

          run() {
            context.openswGoAssembly = () => ({
              ok: true,
              vue: {
                faces: [],
                corps: 1,
                rayon: 1,
                sommets: new Float32Array(0),
              },
              composants: components,
            });
            self.__openswGoResolve();
          }
        };
      }
    },
    fetch: vi.fn(async () => ({
      ok: true,
      arrayBuffer: async () => new ArrayBuffer(0),
    })),
    WebAssembly: { instantiate: async () => ({ instance: {} }) },
    postMessage: (message) => messages.push(message),
  };
  runInNewContext(workerSource, context);

  await self.onmessage({
    data: { type: "init", goOpenSwUrl: "/public/static/wasm/opensw.wasm" },
  });
  await self.onmessage({
    data: {
      id: 2,
      action: "preview",
      filename: "assembly.sldasm",
      data: new ArrayBuffer(0),
    },
  });

  const preview = messages.find((message) => message.type === "preview3d");
  expect(preview.bom).toEqual([
    { file: "Bolt.SLDPRT", configuration: "Default", quantity: 2, instances: ["bolt-1", "bolt-2"] },
    { file: "Bolt.SLDPRT", configuration: "Fine", quantity: 1, instances: ["bolt-3"] },
  ]);
  expect(preview.bomCsv).toContain('"Bolt.SLDPRT","Default",2');
});
