import { describe, expect, it } from "vitest";
import { hasSiblingFile, resolveAssetFilePath } from "./threeAssetUrl.js";

const MODEL = "/3D Prints/Snap/model.stl";
const TEXTURES_DIR = { name: "textures", type: "directory" };

describe("resolveAssetFilePath", () => {
  it("passes through blob, data and external URLs (returns null)", () => {
    expect(resolveAssetFilePath("blob:http://x/abc", { modelPath: MODEL })).toBeNull();
    expect(resolveAssetFilePath("data:image/png;base64,AAAA", { modelPath: MODEL })).toBeNull();
    expect(resolveAssetFilePath("https://cdn.example.com/a.png", { modelPath: MODEL })).toBeNull();
    expect(resolveAssetFilePath("", { modelPath: MODEL })).toBeNull();
  });

  it("resolves relative paths against the model directory", () => {
    expect(resolveAssetFilePath("textures/albedo.png", { modelPath: MODEL })).toBe(
      "/3D Prints/Snap/textures/albedo.png",
    );
    expect(resolveAssetFilePath("../shared/tex.png", { modelPath: MODEL })).toBe(
      "/3D Prints/shared/tex.png",
    );
  });

  it("decodes percent-encoded filenames exactly once", () => {
    expect(resolveAssetFilePath("Wood%20Diffuse.png", { modelPath: MODEL })).toBe(
      "/3D Prints/Snap/textures/Wood Diffuse.png",
    );
    expect(resolveAssetFilePath("textures/Wood%20Diffuse.png", { modelPath: MODEL })).toBe(
      "/3D Prints/Snap/textures/Wood Diffuse.png",
    );
  });

  it("uses the sibling file for bare names when it exists beside the model", () => {
    const items = [{ name: "diffuse.png", type: "file" }];
    expect(resolveAssetFilePath("diffuse.png", { modelPath: MODEL, parentDirItems: items })).toBe(
      "/3D Prints/Snap/diffuse.png",
    );
  });

  it("uses a case-insensitive textures directory when listing shows one", () => {
    const items = [{ name: "Textures", type: "directory" }];
    expect(resolveAssetFilePath("diffuse.png", { modelPath: MODEL, parentDirItems: items })).toBe(
      "/3D Prints/Snap/Textures/diffuse.png",
    );
  });

  it("does not invent a textures directory when the listing shows none", () => {
    const items = [{ name: "other.stl", type: "file" }];
    expect(resolveAssetFilePath("diffuse.png", { modelPath: MODEL, parentDirItems: items })).toBe(
      "/3D Prints/Snap/diffuse.png",
    );
  });

  it("falls back to textures/ when the listing is unknown", () => {
    expect(resolveAssetFilePath("diffuse.png", { modelPath: MODEL })).toBe(
      "/3D Prints/Snap/textures/diffuse.png",
    );
  });

  it("keeps only the filename for Windows absolute paths", () => {
    const items = [TEXTURES_DIR];
    expect(
      resolveAssetFilePath("C:\\Users\\art\\diffuse.png", { modelPath: MODEL, parentDirItems: items }),
    ).toBe("/3D Prints/Snap/textures/diffuse.png");
    expect(
      resolveAssetFilePath("C:/Users/art/diffuse.png", { modelPath: MODEL, parentDirItems: items }),
    ).toBe("/3D Prints/Snap/textures/diffuse.png");
  });

  it("normalizes Windows backslashes in relative paths", () => {
    expect(resolveAssetFilePath("textures\\albedo.png", { modelPath: MODEL })).toBe(
      "/3D Prints/Snap/textures/albedo.png",
    );
  });

  it("keeps parser-expanded source-root-relative references absolute", () => {
    // three.js expands an external buffer to "/models/mesh.bin" for a model at /models/ship.glb.
    // It must not be prefixed with the model directory a second time.
    expect(resolveAssetFilePath("/models/mesh.bin", { modelPath: "/models/ship.glb" })).toBe(
      "/models/mesh.bin",
    );
  });

  it("keeps a bare mesh.bin model-relative", () => {
    expect(resolveAssetFilePath("mesh.bin", { modelPath: "/models/ship.glb" })).toBe(
      "/models/mesh.bin",
    );
  });

  it("still resolves genuinely relative references against the model directory", () => {
    expect(resolveAssetFilePath("models/mesh.bin", { modelPath: "/work/ship.glb" })).toBe(
      "/work/models/mesh.bin",
    );
  });

  it("resolves a bare glTF buffer beside the model when the listing is missing", () => {
    expect(resolveAssetFilePath("ship.bin", { modelPath: "/models/ship.gltf" })).toBe(
      "/models/ship.bin",
    );
  });

  it("resolves a bare glTF buffer beside the model even when a textures folder exists", () => {
    const items = [TEXTURES_DIR];
    expect(
      resolveAssetFilePath("ship.bin", { modelPath: "/models/ship.gltf", parentDirItems: items }),
    ).toBe("/models/ship.bin");
  });

  it("reads the file parameter from FileBrowser API URLs", () => {
    expect(
      resolveAssetFilePath("/api/resources/view?source=default&file=%2F3D%20Prints%2Ftex.png&viewToken=abc", {
        modelPath: MODEL,
      }),
    ).toBe("/3D Prints/tex.png");
    expect(resolveAssetFilePath("/api/resources/view?source=default", { modelPath: MODEL })).toBeNull();
  });
});

describe("hasSiblingFile", () => {
  it("returns true when the listing is unknown so the request is still attempted", () => {
    expect(hasSiblingFile(undefined, "model.mtl")).toBe(true);
  });

  it("matches sibling names case-insensitively", () => {
    expect(hasSiblingFile([{ name: "Model.MTL", type: "file" }], "model.mtl")).toBe(true);
  });

  it("returns false when no matching file exists", () => {
    expect(hasSiblingFile([{ name: "other.obj", type: "file" }], "model.mtl")).toBe(false);
  });

  it("ignores directories with the same name", () => {
    expect(hasSiblingFile([{ name: "model.mtl", type: "directory" }], "model.mtl")).toBe(false);
  });
});
