import { ref } from "vue";

export interface EmbeddedMetadata {
  title?: string;
  artist?: string;
  album?: string;
  year?: string | number;
}

interface EmbeddedArtworkOptions {
  /** `req.metadata` as returned by the backend; `albumArt` is base64 encoded. */
  getMetadata: () => (EmbeddedMetadata & { albumArt?: string }) | undefined;
  getFileName: () => string;
}

/** Title/artist/album and the embedded cover art of the current file, shared by the audio and video player */
export function useEmbeddedArtwork({ getMetadata, getFileName }: EmbeddedArtworkOptions) {
  const metadata = ref<EmbeddedMetadata | null>(null);
  const albumArtUrl = ref<string | null>(null);

  function revokeAlbumArt(url: string | null): void {
    if (url?.startsWith("blob:")) URL.revokeObjectURL(url);
  }

  function decodeAlbumArt(albumArt: string): string | null {
    try {
      const bytes = Uint8Array.from(atob(albumArt), (c) => c.charCodeAt(0));
      return URL.createObjectURL(new Blob([bytes], { type: "image/jpeg" }));
    } catch (error) {
      console.error("Failed to decode album art:", error);
      return null;
    }
  }

  /** Replaces metadata and cover in one step (no empty state in between) and revokes the previous cover. */
  function loadMediaMetadata(): void {
    const meta = getMetadata();
    const previousUrl = albumArtUrl.value;
    metadata.value = {
      title: meta?.title || getFileName(), // Fallback to filename
      artist: meta?.artist,
      album: meta?.album,
      year: meta?.year,
    };
    albumArtUrl.value = meta?.albumArt ? decodeAlbumArt(meta.albumArt) : null;
    revokeAlbumArt(previousUrl);
  }

  function cleanupAlbumArt(): void {
    revokeAlbumArt(albumArtUrl.value);
    albumArtUrl.value = null;
    metadata.value = null;
  }

  return { metadata, albumArtUrl, loadMediaMetadata, cleanupAlbumArt };
}
