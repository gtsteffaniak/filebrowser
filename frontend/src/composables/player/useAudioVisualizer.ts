import { shallowRef } from "vue";

interface AudioGraph {
  element: HTMLMediaElement;
  context: AudioContext;
  source: MediaElementAudioSourceNode;
}

interface AudioVisualizerOptions {
  /** The <audio> element the graph is built on. */
  getMediaElement: () => HTMLMediaElement | null | undefined;
}

type AudioContextConstructor = typeof AudioContext;

function closeMediaGraph(graph: AudioGraph | null): void {
  try {
    graph?.context.close().catch(() => {});
  } catch (_) {
    /* ignore */
  }
}

/**
 * Web Audio graph for the visualizer tab. An element can only be wrapped once, so the
 * graph outlives `cleanupAudioVisualizer()` and is only closed with `{ close: true }`.
 */
export function useAudioVisualizer({ getMediaElement }: AudioVisualizerOptions) {
  const audioContext = shallowRef<AudioContext | null>(null);
  const audioSource = shallowRef<MediaElementAudioSourceNode | null>(null);
  let audioGraphInitialized = false;
  let mediaGraph: AudioGraph | null = null;

  /** Called once, when the visualizer tab of the audio panel asks for the graph. */
  function setupAudioVisualizer(): void {
    if (audioGraphInitialized) return;
    const audio = getMediaElement();
    if (!audio) return;
    let newContext: AudioContext | null = null;
    try {
      let graph = mediaGraph;
      if (!(graph && graph.element === audio && graph.context.state !== "closed")) {
        if (graph) {
          closeMediaGraph(graph);
          mediaGraph = null;
        }
        const AudioContextCtor: AudioContextConstructor | undefined =
          window.AudioContext ||
          (window as unknown as { webkitAudioContext?: AudioContextConstructor }).webkitAudioContext;
        if (!AudioContextCtor) return;
        newContext = new AudioContextCtor({ latencyHint: "playback" });
        const source = newContext.createMediaElementSource(audio);
        source.connect(newContext.destination);
        graph = { element: audio, context: newContext, source };
        mediaGraph = graph;
        newContext = null; // owned by mediaGraph from here on
      }
      audioContext.value = graph.context;
      audioSource.value = graph.source;
      audioGraphInitialized = true;
    } catch (err) {
      console.warn("Audio visualizer creation failed:", err);
      if (newContext) {
        try {
          newContext.close();
        } catch (_) {
          /* ignore */
        }
      }
    }
  }

  /** Browsers suspend a context created before a user gesture, so resume when playback starts. */
  function resumeAudioGraph(): void {
    const context = mediaGraph?.context;
    if (context?.state !== "suspended") return;
    try {
      const resumed = context.resume();
      resumed?.catch((err: unknown) => console.warn("Audio graph resume failed:", err));
    } catch (err) {
      console.warn("Audio visualizer resume failed:", err);
    }
  }

  function cleanupAudioVisualizer({ close = false }: { close?: boolean } = {}): void {
    audioSource.value = null;
    audioContext.value = null;
    audioGraphInitialized = false;
    if (close) {
      closeMediaGraph(mediaGraph);
      mediaGraph = null;
    }
  }

  return { audioContext, audioSource, setupAudioVisualizer, resumeAudioGraph, cleanupAudioVisualizer };
}
