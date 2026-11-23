import type Plyr from "plyr";
import { type Mock, vi } from "vitest";
import { type App, createApp, defineComponent, h, nextTick } from "vue";
import type { useMediaPlayer as realUseMediaPlayer } from "../useMediaPlayer.ts";

// --- Shared helpers for player tests ---

/** Lets the pending awaits, timers and renders of a swap finish. */
export async function settle(): Promise<void> {
  for (let i = 0; i < 5; i++) {
    await nextTick();
    await new Promise<void>((resolve) => {
      setTimeout(resolve, 0);
    });
  }
}

export interface FakeMediaSession {
  metadata: Partial<MediaMetadataInit> | null;
  setActionHandler: Mock;
  playbackState: string;
}

/** Replaces `navigator.mediaSession`; undo it with `removeMediaSession` (and `vi.unstubAllGlobals`). */
export function installMediaSession(): FakeMediaSession {
  const session: FakeMediaSession = { metadata: null, setActionHandler: vi.fn(), playbackState: "none" };
  vi.stubGlobal(
    "MediaMetadata",
    class FakeMetadata {
      constructor(init: MediaMetadataInit) {
        Object.assign(this, init);
      }
    },
  );
  Object.defineProperty(navigator, "mediaSession", { value: session, configurable: true });
  return session;
}

export function removeMediaSession(): void {
  Reflect.deleteProperty(navigator, "mediaSession");
}

/** Component props without the readonly flag, so a test can change them like a parent would. */
export type Writable<T> = { -readonly [K in keyof T]: T[K] };

type Options = Parameters<typeof realUseMediaPlayer>[0];
type Handlers = { play?: () => void; pause?: () => void; timeupdate?: () => void };

export class FakePlayer {
  speed = 1;
  playing = false;
  duration = 0;
  currentTime = 0;
  elements = { container: document.createElement("div") };
  fullscreen: { active: boolean; enter: Mock } = { active: false, enter: vi.fn() };
  on: Mock = vi.fn();
  off: Mock = vi.fn();
  destroy: Mock = vi.fn();
  pause: Mock = vi.fn();
  playMock: Mock = vi.fn(() => Promise.resolve());
  play: Mock = this.playMock;
  constructor(readonly media: HTMLMediaElement) {
    fakePlayers.push(this);
  }
}

/** Every player created since the last `fakePlayers.length = 0`. */
export const fakePlayers: FakePlayer[] = [];
export const reapplyPlaybackRate: Mock = vi.fn();
const stopMediaElement: Mock = vi.fn();
const stopIfRouteLeftThisFile: Mock = vi.fn();

export const resolveMountedFileKey = (source?: string, path?: string) => ({
  source: source || "src",
  path: path ? (path.startsWith("/") ? path : `/${path}`) : "",
});
export const stripPlaybackTimeFromUrl = async (): Promise<void> => {};

export function useMediaPlayer(options: Options) {
  const lifecycle = {
    mountedFile: null as ReturnType<typeof resolveMountedFileKey> | null,
    tornDown: false,
    unmounted: false,
    mountInProgress: false,
  };
  let fake: FakePlayer | null = null;
  let nativePlay: (() => Promise<void> | void) | null = null;

  return {
    lifecycle,
    createPlayer: () => {
      const media = options.getMediaElement();
      if (!media) return null;
      fake = new FakePlayer(media);
      options.player.value = fake as unknown as Plyr;
      return options.player.value;
    },
    setupSharedEvents: (extra: Handlers = {}) => {
      for (const event of ["play", "pause", "timeupdate"] as const) {
        const handler = extra[event];
        if (handler) fake?.on(event, handler);
      }
    },
    onPlayerEvent: (event: string, handler: () => void) => fake?.on(event, handler),
    overridePlay: (handler: (native: () => Promise<void> | void) => Promise<void> | void) => {
      if (!fake) return;
      nativePlay = fake.play.bind(fake);
      fake.play = vi.fn(() => handler(() => nativePlay?.()));
    },
    getNativePlay: () => nativePlay,
    restoreNativePlay: () => {
      if (fake && nativePlay) fake.play = vi.fn(nativePlay);
      nativePlay = null;
    },
    getActiveMediaElement: () => fake?.media ?? options.getMediaElement() ?? null,
    stopMediaElement,
    releaseFile: () => options.onReleaseFile?.(),
    resumeFile: () => {},
    getReleasedFile: () => null,
    setMountedFile: (source?: string, path?: string) => {
      lifecycle.mountedFile = resolveMountedFileKey(source, path);
    },
    reapplyPlaybackRate,
    destroyPlayer: () => {
      fake?.destroy();
      fake = null;
      options.player.value = null;
    },
    stopIfRouteLeftThisFile,
  };
}

/** Runs a composable inside a mounted component so lifecycle hooks work. */
export function withSetup<T>(setup: () => T): { result: T; app: App } {
  let result!: T;
  const app = createApp(
    defineComponent({
      setup() {
        result = setup();
        return () => h("div");
      },
    }),
  );
  app.mount(document.createElement("div"));
  return { result, app };
}
