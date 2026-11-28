/** Extensions supported by {@link convertToVTT} (lowercase, with leading dot). */
const SUBTITLE_FORMAT_EXTENSIONS = [
  '.vtt', '.srt', '.sbv', '.lrc', '.ass', '.ssa', '.smi', '.sub'
]

/**
 * Resolves the format extension for subtitle conversion.
 * Filenames like `Movie.en.srt` or `Show.fr-cc.srt` must use `.srt`, not `.en.srt`
 * (see {@link getFileExtension} in files.js, which uses the first dot segment).
 */
export function getSubtitleFormatExtension(filename: string | null | undefined): string {
  if (typeof filename !== 'string') {
    return ''
  }
  const lower = filename.toLowerCase()
  for (const ext of SUBTITLE_FORMAT_EXTENSIONS) {
    if (lower.endsWith(ext)) {
      return ext
    }
  }
  const lastDot = filename.lastIndexOf('.')
  if (lastDot === -1) {
    return ''
  }
  return filename.slice(lastDot).toLowerCase()
}

/**
 * Strips ASS/SSA override blocks from dialogue text (e.g. {\fad(100,100)\blur3}).
 * Used only when converting .ass / .ssa to VTT. Other formats keep their styling.
 */
export function stripAssOverrideTags (text: string): string {
  if (typeof text !== 'string') return text
  return text
    .replace(/\{[^}]*\}/g, '')
    .replace(/\s+/g, ' ')
    .trim()
}

/** For each index, the first later entry with a strictly greater value */
function nextLaterBoundaries (starts: number[]): (number | undefined)[] {
  const result: (number | undefined)[] = new Array(starts.length)
  const stack: number[] = []
  for (let i = starts.length - 1; i >= 0; i--) {
    const current = starts[i] as number
    while (stack.length && (starts[stack.at(-1) as number] as number) <= current) stack.pop()
    const top = stack.at(-1)
    result[i] = top === undefined ? undefined : starts[top]
    stack.push(i)
  }
  return result
}

/** Translates ASS line-break (\N) and non-breaking space (\h) escapes. */
function decodeAssEscapes (text: string): string {
  return text.replace(/(?:\\N)+/g, '\n').replace(/\\n/g, ' ').replace(/\\h/g, '\u00a0')
}

const LRC_TIMESTAMP_RE = /^\[(\d+):(\d{1,2}(?:\.\d+)?)\]/
const LRC_FALLBACK_DURATION_MS = 3000

function lrcToVttCues (lines: string[]): string {
  const entries: { start: number; dialogue: string }[] = []
  for (const line of lines) {
    let rest = line.trim()
    const starts: number[] = []
    let match = LRC_TIMESTAMP_RE.exec(rest)
    while (match) {
      starts.push(Number(match[1]) * 60000 + Math.round(Number.parseFloat(match[2] ?? '0') * 1000))
      rest = rest.slice(match[0].length)
      match = LRC_TIMESTAMP_RE.exec(rest)
    }
    const dialogue = rest.trim()
    for (const start of starts) {
      entries.push({ start, dialogue })
    }
  }
  entries.sort((a, b) => a.start - b.start)
  const nextStarts = nextLaterBoundaries(entries.map(entry => entry.start))
  return entries
    .map((entry, index) => {
      if (!entry.dialogue) return ''
      const end = nextStarts[index] ?? entry.start + LRC_FALLBACK_DURATION_MS
      return `${formatMillisecondsToVTT(entry.start)} --> ${formatMillisecondsToVTT(end)}\n ${entry.dialogue}`
    })
    .filter(cue => cue !== '')
    .join('\n\n')
}

export function convertToVTT (ext: string, text: string): string {
  if (ext === '.vtt') {
    return text
  }
  let vttContent = 'WEBVTT\n\n'
  const lrcLines: string[] = text.split('\n').filter(line => line.trim() !== '') // Filter out empty lines

  switch (ext.toLowerCase()) {
    case '.srt':
      // Convert SRT to VTT
      vttContent += text
        .replace(/\r\n|\r/g, '\n') // Normalize newlines
        .replace(
          /\d+\n(\d{2}):(\d{2}):(\d{2}),(\d{3}) --> (\d{2}):(\d{2}):(\d{2}),(\d{3})/g,
          (_match, h1, m1, s1, ms1, h2, m2, s2, ms2) => {
            return `${Number.parseInt(h1, 10)}:${m1}:${s1}.${ms1} --> ${Number.parseInt(h2, 10)}:${m2}:${s2}.${ms2}`
          }
        ) // Fix timestamps (remove leading zeros)
        .replace(/\n\n+/g, '\n\n') // Ensure proper spacing
        .replace(/(\d+:\d{2}:\d{2}\.\d{3})\n([^\n])/g, '$1\n$2') // Prevent extra blank lines
        .trim()
      break

    case '.sbv':
      // Convert SBV to VTT
      vttContent += text
        .replace(
          /(\d{1,2}:\d{2}:\d{2}\.\d{3}),(\d{1,2}:\d{2}:\d{2}\.\d{3})/g,
          '$1 --> $2'
        ) // Convert comma to -->
        .replace(/\n\n+/g, '\n\n')
        .trim()
      break

    case '.lrc':
      vttContent += lrcToVttCues(lrcLines)
      break

    case '.ass':
    case '.ssa':
      // Convert ASS/SSA to VTT
      vttContent += text
        .split('\n')
        .filter(line => line.startsWith('Dialogue:')) // Keep only dialogue lines
        .map(line => {
          const parts = line.split(',')
          const startTime = formatAssTime((parts[1] ?? '').trim())
          const endTime = formatAssTime((parts[2] ?? '').trim())
          const dialogue = decodeAssEscapes(stripAssOverrideTags(parts.slice(9).join(',').trim()))
          return `${startTime} --> ${endTime}\n ${dialogue}` // Add leading space before dialogue
        })
        .join('\n\n')
        .trim()
      break
    case '.smi':
      // Convert SAMI to VTT
      vttContent += samiToVttCues(text)
      break
    case '.sub':
      // Convert SUB to VTT
      vttContent += text
        .replace(/\r\n|\r/g, '\n') // Normalize newlines
        .split('\n\n') // Split by empty lines (each caption block)
        .map(block => {
          const lines = block.split('\n').filter(line => line.trim() !== ''); // Remove empty lines
          if (lines.length < 2) return ''; // Ensure each block has a timestamp and text

          const [time = '', ...dialogue] = lines; // First line is time, rest is dialogue
          let [start, end] = time.split(','); // Split start and end time

          if (!start || !end || dialogue.length === 0) return ''; // Skip invalid blocks

          start = formatSubTime(start.trim());
          end = formatSubTime(end.trim());

          return `${start} --> ${end}\n${dialogue.join(' ')}`; // Join dialogue lines into one block
        })
        .filter(block => block !== '') // Remove empty blocks
        .join('\n\n')
        .trim();
      break;


    default:
      throw new Error('Unsupported subtitle format.')
  }

  return vttContent
}

function samiToVttCues (text: string): string {
  const normalized = text.replace(/\r\n|\r/g, '\n')
  const syncs = [...normalized.matchAll(/<SYNC\s+Start\s*=\s*["']?(\d+)["']?[^>]*>/gi)]
  const nextStarts = nextLaterBoundaries(syncs.map(sync => Number.parseInt(sync[1] ?? '0', 10)))
  return syncs
    .map((sync, index) => {
      const start = Number.parseInt(sync[1] ?? '0', 10)
      const bodyStart = (sync.index ?? 0) + sync[0].length
      const bodyEnd = syncs[index + 1]?.index ?? normalized.length
      const body = normalized.slice(bodyStart, bodyEnd).replace(/<\/SYNC>[\s\S]*$/i, '')
      const dialogue = body
        .replace(/<\/?(?:BODY|SAMI)[^>]*>[\s\S]*$/i, '')
        .replace(/<br\s*\/?>/gi, '\n')
        .replace(/<[^>]+>/g, '')
        .replace(/&nbsp;/gi, ' ')
        .split('\n')
        .map(line => line.trim())
        .filter(Boolean)
        .join('\n')
      if (!dialogue) return ''
      const end = nextStarts[index] ?? start + 3000
      return `${formatMillisecondsToVTT(start)} --> ${formatMillisecondsToVTT(end)}\n${dialogue}`
    })
    .filter(cue => cue !== '')
    .join('\n\n')
}

// Helper function to fix ASS/SSA timestamps
function formatAssTime (time: string): string {
  const parts = time.split(':')
  const hours = parts.length === 3 ? Number.parseInt(parts[0] ?? '', 10) : 0
  const minutes = parts.length === 3 ? parts[1] : parts[0]
  const seconds = (parts.at(-1) ?? '').replace('.', ':')

  // Ensure proper milliseconds formatting (000)
  const [sec, ms = '000'] = seconds.split(':')
  return `${hours}:${minutes}:${sec}.${ms.padEnd(3, '0')}`
}

// Helper function to ensure proper SUB timestamp format (with 3-digit milliseconds)
function formatSubTime(time: string): string {
  const parts = time.split(':');
  if (parts.length < 3) return '00:00:00.000'; // Fallback for invalid input

  const [hours, minutes, secMs = ''] = parts;
  const seconds = secMs.split('.')[0];
  let ms = secMs.split('.')[1] || '000';

  ms = ms.padEnd(3, '0'); // Ensure milliseconds are 3 digits

  return `${hours}:${minutes}:${seconds}.${ms}`;
}


// Helper function to convert milliseconds to HH:MM:SS.MMM format
function formatMillisecondsToVTT (ms: number): string {
  const totalSeconds = Math.floor(ms / 1000)
  const milliseconds = (ms % 1000).toString().padStart(3, '0')
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60
  return `${hours}:${minutes.toString().padStart(2, '0')}:${seconds
    .toString()
    .padStart(2, '0')}.${milliseconds}`
}
