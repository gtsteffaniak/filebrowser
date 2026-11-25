import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAudioVisualizer } from "../useAudioVisualizer.ts";

class FakeSource {
  connect = vi.fn();
}

class FakeAudioContext {
  static instances: FakeAudioContext[] = [];
  static wrappedElements = new WeakSet<object>();
  static at(index: number): FakeAudioContext {
    const instance = FakeAudioContext.instances[index];
    if (!instance) throw new Error(`no AudioContext #${index}`);
    return instance;
  }
  state = "running";
  destination = {};
  sources: FakeSource[] = [];
  close = vi.fn(() => {
    this.state = "closed";
    return Promise.resolve();
  });
  resume = vi.fn(() => {
    this.state = "running";
    return Promise.resolve();
  });
  constructor() {
    FakeAudioContext.instances.push(this);
  }
  createMediaElementSource(element: HTMLMediaElement) {
    if (FakeAudioContext.wrappedElements.has(element)) {
      throw new Error("HTMLMediaElement already connected previously to a different MediaElementSourceNode");
    }
    FakeAudioContext.wrappedElements.add(element);
    const source = new FakeSource();
    this.sources.push(source);
    return source;
  }
}

function setup(element: HTMLMediaElement | null = document.createElement("audio")) {
  let el = element;
  const visualizer = useAudioVisualizer({ getMediaElement: () => el });
  return { ...visualizer, setElement: (next: HTMLMediaElement | null) => (el = next) };
}

describe("useAudioVisualizer", () => {
  beforeEach(() => {
    FakeAudioContext.instances = [];
    FakeAudioContext.wrappedElements = new WeakSet();
    vi.stubGlobal("AudioContext", FakeAudioContext);
    vi.spyOn(window, "AudioContext", "get").mockReturnValue(FakeAudioContext as unknown as typeof AudioContext);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("builds the graph once and exposes the context and source", () => {
    const v = setup();
    v.setupAudioVisualizer();
    v.setupAudioVisualizer();
    expect(FakeAudioContext.instances).toHaveLength(1);
    expect(v.audioContext.value).toBe(FakeAudioContext.at(0));
    expect(v.audioSource.value).toBe(FakeAudioContext.at(0).sources[0]);
    expect(FakeAudioContext.at(0).sources[0]?.connect).toHaveBeenCalledWith(
      FakeAudioContext.at(0).destination,
    );
  });

  it("does nothing without a media element", () => {
    const v = setup(null);
    v.setupAudioVisualizer();
    expect(FakeAudioContext.instances).toHaveLength(0);
    expect(v.audioContext.value).toBeNull();
  });

  it("reuses the graph for the same element after a non-closing cleanup", () => {
    const v = setup();
    v.setupAudioVisualizer();
    v.cleanupAudioVisualizer();
    expect(v.audioContext.value).toBeNull();
    expect(v.audioSource.value).toBeNull();
    v.setupAudioVisualizer();
    expect(FakeAudioContext.instances).toHaveLength(1);
    expect(v.audioContext.value).toBe(FakeAudioContext.at(0));
  });

  it("closes the graph on cleanup with close and builds a new one for a fresh element", () => {
    const v = setup();
    v.setupAudioVisualizer();
    const first = FakeAudioContext.at(0);
    v.cleanupAudioVisualizer({ close: true });
    expect(first.close).toHaveBeenCalled();
    v.setElement(document.createElement("audio"));
    v.setupAudioVisualizer();
    expect(FakeAudioContext.instances).toHaveLength(2);
    expect(v.audioContext.value).toBe(FakeAudioContext.at(1));
    expect(v.audioSource.value).toBe(FakeAudioContext.at(1).sources[0]);
  });

  it("does not expose a graph when the same element is wrapped again after close", () => {
    const v = setup();
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    v.setupAudioVisualizer();
    v.cleanupAudioVisualizer({ close: true });
    v.setupAudioVisualizer();
    expect(v.audioContext.value).toBeNull();
    expect(v.audioSource.value).toBeNull();
    expect(FakeAudioContext.at(1).close).toHaveBeenCalled();
    expect(warn).toHaveBeenCalled();
  });

  it("replaces the graph when the element changed", () => {
    const v = setup();
    v.setupAudioVisualizer();
    const first = FakeAudioContext.at(0);
    v.cleanupAudioVisualizer();
    v.setElement(document.createElement("audio"));
    v.setupAudioVisualizer();
    expect(first.close).toHaveBeenCalled();
    expect(FakeAudioContext.instances).toHaveLength(2);
  });

  it("resumes only a suspended context", () => {
    const v = setup();
    v.setupAudioVisualizer();
    const ctx = FakeAudioContext.at(0);
    v.resumeAudioGraph();
    expect(ctx.resume).not.toHaveBeenCalled();
    ctx.state = "suspended";
    v.resumeAudioGraph();
    expect(ctx.resume).toHaveBeenCalledTimes(1);
  });

  it("closes a context it created when wiring the element fails", () => {
    const v = setup();
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.spyOn(FakeAudioContext.prototype, "createMediaElementSource").mockImplementation(() => {
      throw new Error("already wrapped");
    });
    v.setupAudioVisualizer();
    expect(FakeAudioContext.at(0).close).toHaveBeenCalled();
    expect(v.audioContext.value).toBeNull();
    expect(warn).toHaveBeenCalled();
  });
});
