import { afterEach, describe, expect, it, vi } from "vitest";

class WorkerMock {
  messages = [];
  onmessage = null;
  onerror = null;

  constructor() {
    WorkerMock.instance = this;
  }

  postMessage(message) {
    this.messages.push(message);
    if (message.action === "preview") {
      queueMicrotask(() => {
        this.onmessage?.({
          data: {
            id: message.id,
            type: "preview3d",
            mesh: { faces: [], corps: 0, rayon: 1, sommets: new ArrayBuffer(0) },
          },
        });
      });
    }
  }
}

describe("CAD preview modes", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.resetModules();
  });

  it("uses the fast default and marks explicit B-rep loads", async () => {
    vi.stubGlobal("Worker", WorkerMock);
    const { loadCad3D, loadCadBRep } = await import("./cadService.ts");
    const bytes = new ArrayBuffer(0);

    await loadCad3D(bytes, "part.sldprt");
    await loadCadBRep(bytes, "part.sldprt");

    const requests = WorkerMock.instance.messages.filter((message) => message.action === "preview");
    expect(requests.map((message) => message.brep)).toEqual([false, true]);
  });
});
