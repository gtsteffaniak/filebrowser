// src/utils/lyrics.ts

// the supported formats are: elrc, lrc, srt, vtt, or embedded (in that exact order) coming from the backend.
// and only .elrc will have `words` for word by word highligh.

export interface LyricWord {
  text: string;
  timestamp: number;
}

export interface LyricLine {
  text: string;
  timestamp: number;
  words?: LyricWord[];
}

export interface LyricsMeta {
  title?: string;
  artist?: string;
}

export type LyricLines = LyricLine[] & { lrcMeta?: LyricsMeta };

const INLINE_TAG_RE = /<[^>]*>/g;

// Lines that have brackets like [ti: Title] - unlike the timestamps that are like 
// [00:12.34] (starting with digits) are treated as lyric metadata.
const META_LINE_RE = /^\[[a-zA-Z]+:[^\]]*\]$/;

function isMetadataTag(line: string): boolean {
  return META_LINE_RE.test(line);
}

function parseTimestamp(str: string): number | null {
  const cleaned = str.trim().replace(",", ".");
  const parts = cleaned.split(":");
  const secParts = parts.pop()!.split(".");
  const sec = Number(secParts[0]);
  const frac = Number((secParts[1] || "0").padEnd(3, "0").slice(0, 3));
  const min = parts.length ? Number(parts.pop()) : 0;
  const hour = parts.length ? Number(parts.pop()) : 0;
  if ([hour, min, sec, frac].some((n) => Number.isNaN(n))) return null;
  return (hour * 3600 + min * 60 + sec) * 1000 + frac;
}

function matchLrcTags(line: string): { timestamps: number[]; text: string } | null {
  let rest = line;
  const timestamps: number[] = [];
  while (rest.startsWith("[")) {
    const end = rest.indexOf("]");
    if (end === -1) break;
    const timestamp = parseTimestamp(rest.slice(1, end));
    if (timestamp === null) break;
    timestamps.push(timestamp);
    rest = rest.slice(end + 1);
  }
  if (!timestamps.length) return null;
  return { timestamps, text: rest };
}

function splitWordTags(text: string): { text: string; timestamp: number | null }[] {
  const segments = text.split("<");
  const words: { text: string; timestamp: number | null }[] = [];
  const leading = segments.shift();
  if (leading?.trim()) {
    words.push({ text: leading, timestamp: null });
  }
  for (const seg of segments) {
    const end = seg.indexOf(">");
    if (end === -1) continue;
    const timestamp = parseTimestamp(seg.slice(0, end));
    if (timestamp === null) continue;
    words.push({ text: seg.slice(end + 1), timestamp });
  }
  return words;
}

function parseLrc(raw: string): LyricLine[] {
  const lines: LyricLine[] = [];
  for (const rawLine of raw.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line) continue;
    if (isMetadataTag(line)) continue;
    const tag = matchLrcTags(line);
    if (!tag) {
      lines.push({ text: line.replace(INLINE_TAG_RE, "").trim(), timestamp: 0 });
      continue;
    }
    const text = tag.text.replace(INLINE_TAG_RE, "").trim();
    for (const timestamp of tag.timestamps) {
      lines.push({ text, timestamp });
    }
  }
  return lines;
}

function parseElrc(raw: string): LyricLine[] {
  const lines: LyricLine[] = [];
  for (const rawLine of raw.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line) continue;
    if (isMetadataTag(line)) continue;
    const tag = matchLrcTags(line);
    if (!tag) {
      lines.push({ text: line.replace(INLINE_TAG_RE, "").trim(), timestamp: 0, words: [] });
      continue;
    }
    const rawWords = splitWordTags(tag.text).map((word) => ({ ...word, text: word.text.trim() }));
    const text = rawWords.map((word) => word.text).filter(Boolean).join(" ");
    const words: LyricWord[] = rawWords.filter(
      (word): word is { text: string; timestamp: number } => word.timestamp !== null && !!word.text
    );
    for (const timestamp of tag.timestamps) {
      lines.push({ text, timestamp, words });
    }
  }
  return lines;
}

function parseBlock(raw: string): LyricLine[] {
  const lines: LyricLine[] = [];
  for (const block of raw.split(/\r?\n\r?\n+/)) {
    const blockLines = block.split(/\r?\n/).map((l) => l.trim()).filter(Boolean);
    const timeLine = blockLines.find((l) => l.includes("-->"));
    if (!timeLine) continue;
    const timestamp = parseTimestamp(timeLine.split("-->")[0]);
    if (timestamp === null) continue;
    const text = blockLines
      .slice(blockLines.indexOf(timeLine) + 1)
      .join(" ")
      .replace(INLINE_TAG_RE, "")
      .trim();
    if (!text) continue;
    lines.push({ text, timestamp });
  }
  return lines;
}

// Extracts [ti:...] / [ar:...] metadata tags in lyrics
// Only lrc/elrc can have this from what I know
// we can also add them manually with the editor if using sidecar files since is just text with brackets.
function extractMeta(raw: string, format: string): LyricsMeta {
  const lrcMeta: LyricsMeta = {};
  if (format !== "lrc" && format !== "elrc") return lrcMeta;
  for (const rawLine of raw.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line || !isMetadataTag(line)) continue;
    const end = line.indexOf(":");
    const key = line.slice(1, end).trim().toLowerCase();
    const value = line.slice(end + 1, -1).trim();
    if (!value) continue;
    if (key === "ti") lrcMeta.title = value;
    else if (key === "ar") lrcMeta.artist = value;
  }
  return lrcMeta;
}

export function parseLyrics(raw: string, format: string = "lrc"): LyricLines {
  if (!raw) return [];
  let lines: LyricLines;
  switch (format) {
    case "srt":
    case "vtt":
      lines = parseBlock(raw);
      break;
    case "elrc":
      lines = parseElrc(raw);
      break;
    default:
      lines = parseLrc(raw);
  }
  Object.defineProperty(lines, "lrcMeta", {
    value: extractMeta(raw, format),
    enumerable: false,
    configurable: true,
    writable: true,
  });
  return lines;
}
