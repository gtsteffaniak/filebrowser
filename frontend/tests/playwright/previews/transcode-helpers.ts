import type { Locator, Page } from "@playwright/test";
import { expect } from "../test-setup.ts";

export const MINIMAL_PLAYLIST = `#EXTM3U
#EXT-X-VERSION:6
#EXT-X-TARGETDURATION:2
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:2.0,
seg/00000.m4s
#EXT-X-ENDLIST
`;

/**
 * Force the static app-config to advertise
 * transcoding so the plyr mode menu appears even when the CI image lacks ffmpeg.
 *
 * Hook getElementById so the patch runs when constants.js reads `#app-config`.
 * Only rewrite the JSON text — do not replace `window.globalVars` with a new
 * object, or the module-scoped binding used by isTranscodeAvailable stays stale.
 */
export async function enableTranscodeUi(page: Page) {
  await page.addInitScript(() => {
    const original = Document.prototype.getElementById;
    Document.prototype.getElementById = function patchedGetElementById(id) {
      const el = original.call(this, id);
      if (id === "app-config" && el && el.textContent && !el.dataset.transcodeUiPatched) {
        try {
          const cfg = JSON.parse(el.textContent);
          cfg.mediaAvailable = true;
          cfg.transcodeEnabled = true;
          if (!Array.isArray(cfg.transcodeProfiles) || cfg.transcodeProfiles.length === 0) {
            cfg.transcodeProfiles = ["quality", "balanced", "datasaver"];
          }
          el.textContent = JSON.stringify(cfg);
          el.dataset.transcodeUiPatched = "1";
        } catch (_) {
          // Leave the page config alone if it is not JSON.
        }
      }
      return el;
    };
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
      for (const [id, sess] of [...sessions.entries()]) {
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

    const segmentMatch = path.match(/\/sessions\/([^/]+)\/(?:init\.mp4|seg\/[^/]+)$/);
    if (method === "GET" && segmentMatch) {
      await route.fulfill({
        status: 200,
        contentType: "video/iso.segment",
        body: Buffer.from([0, 0, 0, 0]),
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

/** Top-level settings gear only — Plyr also stamps data-plyr="settings" on menu back buttons. */
export function plyrSettingsButton(page: Page) {
  return page.locator(".plyr-viewer button[data-plyr='settings']:not([role='menuitem'])");
}

export function plyrTranscodeMenuButton(page: Page) {
  return page.locator(".plyr-viewer button[data-plyr-menu='transcodeMode']");
}

/** DOM click — Plyr controls are often off-screen / covered in the CI viewport. */
export async function domClick(locator: Locator) {
  await locator.evaluate((el: HTMLElement) => el.click());
}

export async function openTranscodeModeMenu(page: Page) {
  const settings = plyrSettingsButton(page);
  await expect(settings).toBeAttached({ timeout: 15_000 });
  await domClick(settings);
  const transcodeBtn = plyrTranscodeMenuButton(page);
  await expect(transcodeBtn).toBeAttached({ timeout: 10_000 });
  // ensurePlaybackModeApplied should have cleared hidden when transcode is available.
  await expect(transcodeBtn).not.toHaveAttribute("hidden", { timeout: 10_000 });
  await domClick(transcodeBtn);
}

export async function selectTranscodeMode(page: Page, mode: string) {
  await openTranscodeModeMenu(page);
  const option = page.locator(`.plyr-viewer button[data-plyr='transcode-mode'][value='${mode}']`);
  await expect(option).toBeAttached({ timeout: 10_000 });
  await domClick(option);
}
