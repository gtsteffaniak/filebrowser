import type { Locator, Page } from "@playwright/test";
import { expect } from "../test-setup.ts";

export const MINIMAL_PLAYLIST = `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:2
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:VOD
#EXT-X-MAP:URI="init.mp4"
#EXTINF:2.0,
seg/00000.m4s
#EXT-X-ENDLIST
`;

// 64x64 VP9 fMP4, 2s: hls must be able to buffer it or the HLS attach fails and the session is stopped
const INIT_SEGMENT = Buffer.from("AAAAHGZ0eXBpc281AAACAGlzbzVpc282bXA0MQAAAw9tb292AAAAbG12aGQAAAAAAAAAAAAAAAAAAAPoAAAAAAABAAABAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAACEXRyYWsAAABcdGtoZAAAAAMAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAEAAAAAAQAAAAEAAAAAAACRlZHRzAAAAHGVsc3QAAAAAAAAAAQAAAAAAAAAAAAEAAAAAAYltZGlhAAAAIG1kaGQAAAAAAAAAAAAAAAAAACgAAAAAAFXEAAAAAAAtaGRscgAAAAAAAAAAdmlkZQAAAAAAAAAAAAAAAFZpZGVvSGFuZGxlcgAAAAE0bWluZgAAABR2bWhkAAAAAQAAAAAAAAAAAAAAJGRpbmYAAAAcZHJlZgAAAAAAAAABAAAADHVybCAAAAABAAAA9HN0YmwAAACoc3RzZAAAAAAAAAABAAAAmHZwMDkAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAQABAAEgAAABIAAAAAAAAAAEYTGF2YzYwLjMxLjEwMiBsaWJ2cHgtdnA5AAAAAAAAAAAY//8AAAAUdnBjQwEAAAAACoICAgIAAAAAAApmaWVsAQAAAAAQcGFzcAAAAAEAAAABAAAAFGJ0cnQAAAAAAABOIAAATiAAAAAQc3R0cwAAAAAAAAAAAAAAEHN0c2MAAAAAAAAAAAAAABRzdHN6AAAAAAAAAAAAAAAAAAAAEHN0Y28AAAAAAAAAAAAAAChtdmV4AAAAIHRyZXgAAAAAAAAAAQAAAAEAAAAAAAAAAAAAAAAAAABidWR0YQAAAFptZXRhAAAAAAAAACFoZGxyAAAAAAAAAABtZGlyYXBwbAAAAAAAAAAAAAAAAC1pbHN0AAAAJal0b28AAAAdZGF0YQAAAAEAAAAATGF2ZjYwLjE2LjEwMA==", "base64");
const MEDIA_SEGMENT = Buffer.from("AAAAGHN0eXBtc2RoAAAAAG1zZGhtc2l4AAAANHNpZHgBAAAAAAAAAQAAKAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAfwAAFAAgAAAAAAAALhtb29mAAAAEG1maGQAAAAAAAAAAQAAAKB0cmFmAAAAHHRmaGQAAgA4AAAAAQAABAAAAAAfAQEAAAAAABR0ZmR0AQAAAAAAAAAAAAAAAAAAaHRydW4AAAIFAAAAFAAAAMACAAAAAAAAHwAAAA8AAAAPAAAADwAAAA8AAAAPAAAADwAAAA8AAAAPAAAADwAAAA8AAAAPAAAADwAAAA8AAAAPAAAADwAAAA8AAAAPAAAADwAAAA8AAAFEbWRhdIJJg0IAA/AD9gA4JBwYQgAAMGAAAGc///9ZrxE7swCGAECSnABJQAADIAAAVHCGAECSnABKwAADIAAAVHCGAECSnABJwAADIAAAVHCGAECSnABIoAADIAAAVHCGAECSnABHgAADIAAAVHCGAECSnABG4AADIAAAVHCGAECSnABGQAADIAAAVHCGAECSnABFwAADIAAAVHCGAECSnABFQAADIAAAVHCGAMCSnABBwAADIAAAVHCGAECSnABE4AADIAAAVHCGAECSnABEYAADIAAAVHCGAECSnABEAAADIAAAVHCGAECSnABDoAADIAAAVHCGAECSnABDQAADIAAAVHCGAECSnABDAAADIAAAVHCGAECSnABCwAADIAAAVHCGAECSnABCgAADIAAAVHCGAECSnABCQAADIAAAVHA=", "base64");

/**
 * Force ffmpeg availability in the static app-config and advertise transcoding via
 * the live media settings API, so the plyr mode menu appears even when the CI image
 * lacks ffmpeg. Transcode state is read only from the live API.
 *
 * Hook getElementById so the patch runs when constants.js reads `#app-config`.
 * Only rewrite the JSON text — do not replace `window.globalVars` with a new
 * object, or the module-scoped binding used by isTranscodeAvailable stays stale.
 */
export async function enableTranscodeUi(page: Page) {
  await page.addInitScript(() => {
    const patchAppConfig = (el: HTMLElement | null) => {
      if (el?.textContent && !el.dataset.transcodeUiPatched) {
        try {
          const cfg = JSON.parse(el.textContent);
          cfg.ffmpegAvailable = true;
          el.textContent = JSON.stringify(cfg);
          el.dataset.transcodeUiPatched = "1";
        } catch (_) {
          // Leave the page config alone if it is not JSON.
        }
      }
      return el;
    };
    const originalGetElementById = Document.prototype.getElementById;
    Document.prototype.getElementById = function patchedGetElementById(this: Document, id: string) {
      const el = originalGetElementById.call(this, id);
      return id === "app-config" ? patchAppConfig(el) : el;
    };
    const originalQuerySelector = Document.prototype.querySelector;
    Document.prototype.querySelector = function patchedQuerySelector(this: Document, selector: string) {
      const el = originalQuerySelector.call(this, selector) as HTMLElement | null;
      return selector === "#app-config" ? patchAppConfig(el) : el;
    } as typeof Document.prototype.querySelector;
  });

  await page.route("**/api/settings/media", async (route) => {
    if (route.request().method() !== "GET") {
      return route.continue();
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      // Matches the PUBLIC UserMedia payload from GET /api/settings/media.
      body: JSON.stringify({
        transcode: {
          enabled: true,
          maxConcurrentPerUser: 1,
          enabledProfiles: ["quality", "balanced", "datasaver"],
        },
        imagePreview: {},
      }),
    });
  });
}

type SessionStart = {
  profile?: string;
  replaceSessionId?: string;
  source?: string;
  path?: string;
};

export async function mockTranscodeSessions(
  page: Page,
  opts: { playlistNotReadyFirst?: boolean } = {},
) {
  let sessionCounter = 0;
  const sessions = new Map<string, { id: string; profile: string; source: string; path: string; state: string }>();
  const starts: SessionStart[] = [];
  let playlistHits = 0;
  // Track the not-ready response per session so a new session's first playlist
  // poll still exercises the retry path instead of inheriting prior state.
  const playlistNotReadySeen = new Set<string>();

  await page.route("**/api/media/transcode/sessions**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const method = request.method();

    if (method === "POST" && /\/sessions\/?$/.test(path)) {
      const body = request.postDataJSON() as SessionStart;
      starts.push(body);
      if (body.replaceSessionId) {
        sessions.delete(body.replaceSessionId);
      }
      // Mirror backend same-file handover when replaceSessionId is omitted.
      for (const [id, sess] of sessions.entries()) {
        if (sess.source === body.source && sess.path === body.path) {
          sessions.delete(id);
        }
      }
      sessionCounter += 1;
      const id = `sess-${sessionCounter}`;
      const profile = body.profile || "quality";
      sessions.set(id, {
        id,
        profile,
        source: body.source || "",
        path: body.path || "",
        state: "starting",
      });
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          id,
          source: body.source,
          path: body.path,
          profile,
          state: "starting",
          startedAt: Date.now() / 1000,
          heartbeatIntervalSec: 12,
          deliveryBaseUrl: `/api/media/transcode/sessions/${id}`,
          snapshot: {
            enabled: true,
            canStart: true,
            sessions: [...sessions.values()],
          },
        }),
      });
      return;
    }

    if (method === "GET" && /\/sessions\/?$/.test(path)) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          enabled: true,
          canStart: sessions.size === 0,
          userLimit: 1,
          userActive: sessions.size,
          sessions: [...sessions.values()],
        }),
      });
      return;
    }

    const playlistMatch = path.match(/\/sessions\/([^/]+)\/index\.m3u8$/);
    if (method === "GET" && playlistMatch) {
      playlistHits += 1;
      const playlistSessionId = playlistMatch[1] || "";
      if (opts.playlistNotReadyFirst && !playlistNotReadySeen.has(playlistSessionId)) {
        playlistNotReadySeen.add(playlistSessionId);
        await route.fulfill({
          status: 503,
          headers: { "Retry-After": "1" },
          contentType: "application/json",
          body: JSON.stringify({ message: "transcode playlist not ready" }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: "application/vnd.apple.mpegurl",
        body: MINIMAL_PLAYLIST,
      });
      return;
    }

    const segmentMatch = path.match(/\/sessions\/([^/]+)\/(init\.mp4|seg\/[^/]+)$/);
    if (method === "GET" && segmentMatch) {
      const isInit = segmentMatch[2] === "init.mp4";
      await route.fulfill({
        status: 200,
        contentType: isInit ? "video/mp4" : "video/iso.segment",
        body: isInit ? INIT_SEGMENT : MEDIA_SEGMENT,
      });
      return;
    }

    if (method === "DELETE") {
      const id = path.split("/").pop() || "";
      sessions.delete(id);
      await route.fulfill({ status: 204, body: "" });
      return;
    }

    if (method === "PATCH" && path.endsWith("/heartbeat")) {
      await route.fulfill({ status: 204, body: "" });
      return;
    }

    await route.continue();
  });

  return { starts, sessions: () => sessions, playlistHits: () => playlistHits };
}

export function nativeVideo(page: Page) {
  return page.locator(".media-player video").first();
}

/** Preview transcode-offer screen (not the in-player settings menu). */
function previewTranscodeOfferButton(page: Page) {
  return page.locator(".preview-notice button[aria-label='Transcode']");
}

/**
 * Wait until Preview.vue finishes media enrichment and mounts the async media player
 * chunk (not the WMV transcode-offer screen).
 */
export async function waitForNativePlayerMounted(page: Page) {
  await expect(page.locator(".av-preview-loading")).toHaveCount(0, { timeout: 30_000 });
  await expect.poll(async () => {
    if (await previewTranscodeOfferButton(page).count()) {
      return "offer";
    }
    if (await page.locator(".media-player").count()) {
      return "player";
    }
    return "pending";
  }, { timeout: 30_000, intervals: [100, 250, 500, 1000] }).toBe("player");
  await expect(nativeVideo(page)).toBeAttached({ timeout: 15_000 });
}

async function clickPlayControl(page: Page) {
  const video = nativeVideo(page);
  await video.click({ force: true });
  const playBtn = page.locator(
    ".media-player .plyr__control--overlaid, .media-player button.plyr__control[data-plyr='play']",
  ).first();
  if (await playBtn.count()) {
    await domClick(playBtn);
  } else {
    await page.keyboard.press("Space");
  }
  await video.evaluate((el: HTMLVideoElement) => {
    void el.play().catch(() => {});
  });
}

function videoIsActivelyPlaying() {
  return (el: HTMLVideoElement) => {
    if (el.paused || el.ended) {
      return false;
    }
    return el.currentTime > 0.05 || el.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA;
  };
}

export async function waitForNativeMediaStream(page: Page) {
  return page.waitForResponse(
    (res) =>
      res.request().method() === "GET"
      && res.url().includes("/api/media/stream")
      && res.ok(),
    { timeout: 15_000 },
  );
}

/**
 * Trusted play gesture so attachVideoStreamAndPlay runs and GET /api/media/stream fires.
 */
export async function startNativeVideoPlayback(page: Page) {
  await waitForNativePlayerMounted(page);
  const video = nativeVideo(page);
  const hasStreamSrc = () =>
    video.evaluate((el: HTMLVideoElement) => (el.getAttribute("src") || "").includes("media/stream"));
  const isPlaying = () => video.evaluate(videoIsActivelyPlaying());

  if (!(await hasStreamSrc())) {
    const streamWait = waitForNativeMediaStream(page);

    if (!(await isPlaying())) {
      await clickPlayControl(page);
    }

    await streamWait;
  } else if (!(await isPlaying())) {
    await clickPlayControl(page);
  }

  const playbackStartTime = await video.evaluate((el: HTMLVideoElement) => el.currentTime);
  await expect.poll(() => video.evaluate(
    (el: HTMLVideoElement, startTime: number) =>
      !el.paused && !el.ended && el.currentTime > startTime + 0.05,
    playbackStartTime,
  ), {
    timeout: 15_000,
    intervals: [100, 250, 500],
  }).toBe(true);
}

/** Top-level settings gear only — Plyr also stamps data-plyr="settings" on menu back buttons. */
export function settingsButton(page: Page) {
  return page.locator(".media-player button[data-plyr='settings']:not([role='menuitem'])");
}

export function transcodeMenuButton(page: Page) {
  return page.locator(".media-player button[data-plyr-menu='transcodeMode']");
}

/** DOM click — Plyr controls are often off-screen / covered in the CI viewport. */
export async function domClick(locator: Locator) {
  await locator.evaluate((el: HTMLElement) => el.click());
}

export async function openTranscodeModeMenu(page: Page) {
  const settings = settingsButton(page);
  await expect(settings).toBeAttached({ timeout: 15_000 });
  await domClick(settings);
  const transcodeBtn = transcodeMenuButton(page);
  await expect(transcodeBtn).toBeAttached({ timeout: 10_000 });
  // ensurePlaybackModeApplied should have cleared hidden when transcode is available.
  await expect(transcodeBtn).not.toHaveAttribute("hidden", { timeout: 10_000 });
  await domClick(transcodeBtn);
}

export async function selectTranscodeMode(page: Page, mode: string) {
  await openTranscodeModeMenu(page);
  const option = page.locator(`.media-player button[data-plyr='transcode-mode'][value='${mode}']`);
  await expect(option).toBeAttached({ timeout: 10_000 });
  await domClick(option);
}
