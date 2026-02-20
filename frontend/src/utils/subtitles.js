/**
 * Strips ASS/SSA override blocks from dialogue text (e.g. {\fad(100,100)\blur3}).
 * Used only when converting .ass / .ssa to VTT. Other formats keep their styling.
 */
export function stripAssOverrideTags (text) {
  if (typeof text !== 'string') return text
  return text
    .replace(/\{[^}]*\}/g, '')
    .replace(/\s+/g, ' ')
    .trim()
}

export function convertToVTT (ext, text) {
  if (ext == '.vtt') {
    return text
  }
  let vttContent = 'WEBVTT\n\n'
  let lrcLines = text.split('\n').filter(line => line.trim() !== '') // Filter out empty lines

  switch (ext.toLowerCase()) {
    case '.srt':
      // Convert SRT to VTT
      vttContent += text
        .replace(/\r\n|\r/g, '\n') // Normalize newlines
        .replace(
          /\d+\n(\d{2}):(\d{2}):(\d{2}),(\d{3}) --> (\d{2}):(\d{2}):(\d{2}),(\d{3})/g,
          (match, h1, m1, s1, ms1, h2, m2, s2, ms2) => {
            return `${parseInt(h1)}:${m1}:${s1}.${ms1} --> ${parseInt(
              h2
            )}:${m2}:${s2}.${ms2}`
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
      // Convert LRC to VTT
      lrcLines = lrcLines
        .map((line, index) => {
          if (line.startsWith('[')) {
            let [time, dialogue] = line.split(']') // Remove square brackets
            if (!time || !dialogue) return '' // Skip invalid lines

            time = time.slice(1) // Remove opening square bracket

            // Format time to HH:MM:SS.MMM
            let startTime = formatLrcTime(time)
            let endTime =
              index + 1 < lrcLines.length
                ? formatLrcTime(lrcLines[index + 1].split(']')[0].slice(1))
                : startTime // Use next line's timestamp for end time

            return `${startTime} --> ${endTime}\n ${dialogue.trim()}` // Add leading space before dialogue
          } else {
            return '' // Skip lines that don't start with '['
          }
        })
        .filter(line => line !== '')
        .join('\n\n')
        .trim() // Remove empty lines

      vttContent += lrcLines
      break

    case '.ass':
    case '.ssa':
      // Convert ASS/SSA to VTT
      vttContent += text
        .split('\n')
        .filter(line => line.startsWith('Dialogue:')) // Keep only dialogue lines
        .map(line => {
          let parts = line.split(',')
          let startTime = formatAssTime(parts[1].trim())
          let endTime = formatAssTime(parts[2].trim())
          let dialogue = stripAssOverrideTags(parts.slice(9).join(',').trim())
          return `${startTime} --> ${endTime}\n ${dialogue}` // Add leading space before dialogue
        })
        .join('\n\n')
        .trim()
      break
    case '.smi':
      // Convert SAMI to VTT
      vttContent += text
        .replace(/\r\n|\r/g, '\n') // Normalize newlines
        .match(/<SYNC Start=\d+>.*?<\/SYNC>/gs) // Match all <SYNC> blocks
        ?.map(syncBlock => {
          let startTime = syncBlock.match(/<SYNC Start=(\d+)>/)[1] // Extract start time in ms
          let dialogueMatch = syncBlock.match(/<P[^>]*>(.*?)<\/P>/) // Extract text inside <P>
          let dialogue = dialogueMatch
            ? dialogueMatch[1].replace(/<[^>]+>/g, '').trim()
            : '' // Remove HTML tags

          if (!dialogue) return '' // Skip empty captions
          let startFormatted = formatMillisecondsToVTT(startTime)

          // Use next start time as the end time if available
          let nextSyncMatch = text.match(
            new RegExp(`<SYNC Start=${parseInt(startTime) + 1}>`)
          )
          let endTime = nextSyncMatch
            ? nextSyncMatch[1]
            : parseInt(startTime) + 3000 // Default to +3s

          let endFormatted = formatMillisecondsToVTT(endTime)
          return `${startFormatted} --> ${endFormatted}\n${dialogue}`
        })
        .filter(syncBlock => syncBlock !== '')
        .join('\n\n')
        .trim()
      break
    case '.sub':
      // Convert SUB to VTT
      vttContent += text
        .replace(/\r\n|\r/g, '\n') // Normalize newlines
        .split('\n\n') // Split by empty lines (each caption block)
        .map(block => {
          let lines = block.split('\n').filter(line => line.trim() !== ''); // Remove empty lines
          if (lines.length < 2) return ''; // Ensure each block has a timestamp and text
    
          let [time, ...dialogue] = lines; // First line is time, rest is dialogue
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

// Helper function to format LRC time (e.g., 00:00.000 -> 00:00:00.000)
function formatLrcTime (time) {
  let [minutes, seconds] = time.split(':')
  let [sec, ms] = seconds.split('.')
  ms = ms ? ms.padEnd(3, '0') : '000' // Ensure milliseconds are 3 digits

  // Return in the correct format
  return `00:${minutes}:${sec}.${ms}`
}

// Helper function to fix ASS/SSA timestamps
function formatAssTime (time) {
  let parts = time.split(':')
  let hours = parts.length === 3 ? parseInt(parts[0]) : 0
  let minutes = parts.length === 3 ? parts[1] : parts[0]
  let seconds = parts[parts.length - 1].replace('.', ':')

  // Ensure proper milliseconds formatting (000)
  const [sec, ms = '000'] = seconds.split(':')
  return `${hours}:${minutes}:${sec}.${ms.padEnd(3, '0')}`
}

// Helper function to ensure proper SUB timestamp format (with 3-digit milliseconds)
function formatSubTime(time) {
  let parts = time.split(':');
  if (parts.length < 3) return '00:00:00.000'; // Fallback for invalid input

  let [hours, minutes, secMs] = parts;
  let [seconds, ms = '000'] = secMs.split('.');

  ms = ms.padEnd(3, '0'); // Ensure milliseconds are 3 digits

  return `${hours}:${minutes}:${seconds}.${ms}`;
}


// Helper function to convert milliseconds to HH:MM:SS.MMM format
function formatMillisecondsToVTT (ms) {
  let totalSeconds = Math.floor(ms / 1000)
  let milliseconds = (ms % 1000).toString().padStart(3, '0')
  let hours = Math.floor(totalSeconds / 3600)
  let minutes = Math.floor((totalSeconds % 3600) / 60)
  let seconds = totalSeconds % 60
  return `${hours}:${minutes.toString().padStart(2, '0')}:${seconds
    .toString()
    .padStart(2, '0')}.${milliseconds}`
}
