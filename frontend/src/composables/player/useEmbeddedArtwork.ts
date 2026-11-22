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

  function loadMediaMetadata(): void {
    const meta = getMetadata();
    metadata.value = {
      title: meta?.title || getFileName(), // Fallback to filename
      artist: meta?.artist,
      album: meta?.album,
      year: meta?.year,
    };
    if (!meta?.albumArt) return;
    try {
      const bytes = Uint8Array.from(atob(meta.albumArt), (c) => c.charCodeAt(0));
      albumArtUrl.value = URL.createObjectURL(new Blob([bytes], { type: "image/jpeg" }));
    } catch (error) {
      console.error("Failed to decode album art:", error);
      albumArtUrl.value = null;
    }
  }

  function cleanupAlbumArt(): void {
    if (albumArtUrl.value?.startsWith("blob:")) {
      URL.revokeObjectURL(albumArtUrl.value);
    }
    albumArtUrl.value = null;
    metadata.value = null;
  }

  return { metadata, albumArtUrl, loadMediaMetadata, cleanupAlbumArt };
}
