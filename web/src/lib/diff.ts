// Parse unified diff to extract old/new content for a specific file.
export function parseDiffForFile(fullDiff: string, filePath: string): { oldValue: string; newValue: string } | null {
  if (!fullDiff) return null

  const lines = fullDiff.split('\n')
  let inTargetFile = false
  let oldLines: string[] = []
  let newLines: string[] = []

  for (const line of lines) {
    if (line.startsWith('diff --git')) {
      const match = line.match(/^diff --git a\/(.+?) b\/(.+)$/)
      if (match) {
        const [, leftPath, rightPath] = match
        inTargetFile = leftPath === filePath || rightPath === filePath
      } else {
        inTargetFile = false
      }
      if (inTargetFile) {
        oldLines = []
        newLines = []
      }
      continue
    }

    if (!inTargetFile) continue

    if (line.startsWith('index ') || line.startsWith('--- ') || line.startsWith('+++ ') || line.startsWith('@@')) {
      continue
    }

    if (line.startsWith('-')) {
      oldLines.push(line.slice(1))
    } else if (line.startsWith('+')) {
      newLines.push(line.slice(1))
    } else if (line.startsWith(' ')) {
      oldLines.push(line.slice(1))
      newLines.push(line.slice(1))
    }
  }

  if (oldLines.length === 0 && newLines.length === 0) {
    return null
  }

  return {
    oldValue: oldLines.join('\n'),
    newValue: newLines.join('\n'),
  }
}
