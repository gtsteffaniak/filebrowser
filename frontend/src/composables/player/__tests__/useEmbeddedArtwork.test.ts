import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { type EmbeddedMetadata, useEmbeddedArtwork } from "../useEmbeddedArtwork.ts";

type Meta = EmbeddedMetadata & { albumArt?: string };

describe("useEmbeddedArtwork", () => {
  let created: string[];
  let revoke: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    created = [];
    revoke = vi.fn();
    vi.stubGlobal(
      "URL",
      Object.assign(URL, {
        createObjectURL: vi.fn(() => {
          const url = `blob:art-${created.length}`;
          created.push(url);
          return url;
        }),
        revokeObjectURL: revoke,
      }),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  function setup(initial: Meta) {
    let meta = initial;
    let name = "song";
    const artwork = useEmbeddedArtwork({ getMetadata: () => meta, getFileName: () => name });
    return {
      ...artwork,
      next: (nextMeta: Meta, nextName = name) => {
        meta = nextMeta;
        name = nextName;
      },
    };
  }

  it("falls back to the file name", () => {
    const art = setup({});
    art.loadMediaMetadata();
    expect(art.metadata.value?.title).toBe("song");
    expect(art.albumArtUrl.value).toBeNull();
  });

  it("replaces metadata and cover together and revokes the old cover", () => {
    const art = setup({ title: "One", albumArt: btoa("one") });
    art.loadMediaMetadata();
    art.next({ title: "Two", albumArt: btoa("two") });
    art.loadMediaMetadata();
    expect(art.metadata.value?.title).toBe("Two");
    expect(art.albumArtUrl.value).toBe("blob:art-1");
    expect(revoke).toHaveBeenCalledWith("blob:art-0");
  });

  it("drops the previous cover when the next file has none", () => {
    const art = setup({ title: "One", albumArt: btoa("one") });
    art.loadMediaMetadata();
    art.next({ title: "Two" });
    art.loadMediaMetadata();
    expect(art.albumArtUrl.value).toBeNull();
    expect(revoke).toHaveBeenCalledWith("blob:art-0");
  });

  it("clears and revokes on cleanup", () => {
    const art = setup({ title: "One", albumArt: btoa("one") });
    art.loadMediaMetadata();
    art.cleanupAlbumArt();
    expect(art.albumArtUrl.value).toBeNull();
    expect(art.metadata.value).toBeNull();
    expect(revoke).toHaveBeenCalledWith("blob:art-0");
  });
});
