import type Plyr from "plyr";
import { transcodeLog } from "@/player/transcoding/transcodeDebug.ts";

export interface VideoLoadingIndicator {
  (): void;
  expectPlayback?: () => void;
}

export type LoadingMedia = Pick<HTMLMediaElement, "currentSrc" | "src" | "networkState" | "readyState" | "paused">;

/** The part of Plyr that the indicator reads and listens to */
export interface LoadingPlayer {
  media: LoadingMedia;
  on: (event: keyof Plyr.PlyrEventMap, callback: () => void) => void;
  off: (event: keyof Plyr.PlyrEventMap, callback: () => void) => void;
}

/** Show a loading indicator while the native video element is fetching or buffering */
export function enableVideoLoadingIndicator(
  player: LoadingPlayer,
  onLoadingChange: (loading: boolean) => void,
): VideoLoadingIndicator {
  const media = player?.media;
  if (!media || typeof onLoadingChange !== "function") {
    return () => {};
  }

  let waiting = false;
  let seeking = false;
  let loading = false;
  let playbackExpected = false;

  const isLoadingOrBuffering = () => (
    waiting
    || seeking
    || loading
    || (
      media.networkState === HTMLMediaElement.NETWORK_LOADING
      && media.readyState < HTMLMediaElement.HAVE_FUTURE_DATA
    )
  );

  let lastShow = false;
  const sync = () => {
    const hasSource = Boolean(media.currentSrc || media.src);
    const show = playbackExpected && hasSource && isLoadingOrBuffering();
    if (show !== lastShow) {
      lastShow = show;
      transcodeLog("loading overlay changed", {
        show,
        playbackExpected,
        waiting,
        seeking,
        loading,
        hasSource,
        networkState: media.networkState,
        readyState: media.readyState,
        paused: media.paused,
        currentSrc: media.currentSrc,
      });
    }
    onLoadingChange(show);
  };

  const onPlay = () => {
    playbackExpected = true;
    sync();
  };

  const onPause = () => {
    playbackExpected = false;
    sync();
  };

  const onWaiting = () => {
    waiting = true;
    sync();
  };
  const onPlaying = () => {
    waiting = false;
    loading = false;
    sync();
  };
  const onLoadStart = () => {
    loading = true;
    sync();
  };
  const onCanPlay = () => {
    loading = false;
    sync();
  };
  const onSeeking = () => {
    seeking = true;
    sync();
  };
  const onSeeked = () => {
    seeking = false;
    sync();
  };
  const onError = () => {
    waiting = false;
    seeking = false;
    loading = false;
    playbackExpected = false;
    sync();
  };
  const onEmptied = () => {
    waiting = false;
    seeking = false;
    loading = false;
    playbackExpected = false;
    sync();
  };

  const handlers: Array<[keyof Plyr.PlyrEventMap, () => void]> = [
    ["play", onPlay],
    ["pause", onPause],
    ["loadstart", onLoadStart],
    ["waiting", onWaiting],
    ["playing", onPlaying],
    ["canplay", onCanPlay],
    ["seeking", onSeeking],
    ["seeked", onSeeked],
    ["error", onError],
    ["emptied", onEmptied],
  ];

  for (const [evt, fn] of handlers) {
    player.on(evt, fn);
  }

  return Object.assign(() => {
    for (const [evt, fn] of handlers) {
      player.off(evt, fn);
    }
    onLoadingChange(false);
  }, {
    expectPlayback: () => {
      playbackExpected = true;
      sync();
    },
  });
}
