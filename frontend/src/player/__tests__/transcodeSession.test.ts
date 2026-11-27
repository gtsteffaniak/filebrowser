import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { transcodeSession } from "@/composables/player/__tests__/mediaTestUtils.ts";

vi.mock("@/api/transcode.ts", () => ({
  startTranscodeSession: vi.fn(),
  heartbeatTranscodeSession: vi.fn(async () => {}),
  stopTranscodeSession: vi.fn(async () => {}),
}));

import { heartbeatTranscodeSession, startTranscodeSession, stopTranscodeSession } from "@/api/transcode.ts";
import { TranscodeSessionController } from "@/player/transcodeSession.ts";

describe("TranscodeSessionController", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.mocked(startTranscodeSession).mockResolvedValue(
      transcodeSession({
        id: "sess-1",
        heartbeatIntervalSec: 12,
        deliveryBaseUrl: "/api/media/transcode/sessions/sess-1",
      }),
    );
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it("starts session and sends heartbeat ticks", async () => {
    const ctrl = new TranscodeSessionController({
      source: "s",
      path: "/v.mp4",
      viewToken: "vt",
      profile: "quality",
      clientId: "c1",
    });
    await ctrl.start({});
    expect(startTranscodeSession).toHaveBeenCalledOnce();
    ctrl.updatePlayback({ currentTime: 12.5, paused: false });
    vi.advanceTimersByTime(12000);
    expect(heartbeatTranscodeSession).toHaveBeenCalled();
    await ctrl.stop();
    expect(stopTranscodeSession).toHaveBeenCalledWith("sess-1", { keepalive: false });
  });

  it("stops with keepalive on the unload path", async () => {
    const ctrl = new TranscodeSessionController({
      source: "s",
      path: "/v.mp4",
      viewToken: "vt",
      profile: "quality",
      clientId: "c1",
    });
    await ctrl.start({});
    await ctrl.stop({ keepalive: true });
    // keepalive lets the DELETE outlive the document; without it a tab close
    // cancels the request and the session is only reclaimed by the janitor.
    expect(stopTranscodeSession).toHaveBeenCalledWith("sess-1", { keepalive: true });
  });

  it("detach stops heartbeat without server stop", async () => {
    const ctrl = new TranscodeSessionController({
      source: "s",
      path: "/v.mp4",
      viewToken: "vt",
      profile: "quality",
    });
    await ctrl.start({});
    ctrl.detach();
    vi.advanceTimersByTime(30000);
    expect(heartbeatTranscodeSession).not.toHaveBeenCalled();
    expect(stopTranscodeSession).not.toHaveBeenCalled();
  });
});
