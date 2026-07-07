export function highlightQuery(text: string, query: string): string {
  const safe = text.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]!))
  const esc = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  return safe.replace(new RegExp(esc, 'gi'), '<mark>$&</mark>')
}

export function fuzzyScore(query: string, str: string): number {
  const q = query.toLowerCase()
  const s = str.toLowerCase()
  if (s === q) return 10000
  if (s.startsWith(q)) return 9000 - s.length
  if (s.includes(q)) return 8000 - s.length
  let qi = 0, score = 0, consecutive = 0
  for (let i = 0; i < s.length && qi < q.length; i++) {
    if (s[i] === q[qi]) {
      qi++; consecutive++
      score += consecutive * 5
      if (i === 0 || '/-_'.includes(s[i - 1])) score += 15
    } else { consecutive = 0 }
  }
  return qi === q.length ? score : -1
}

export function fuzzyHighlight(query: string, str: string): string {
  const safe = str.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]!))
  const q = query.toLowerCase(), s = str.toLowerCase()
  if (s.includes(q)) {
    const esc = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    return safe.replace(new RegExp(esc, 'gi'), '<mark>$&</mark>')
  }
  let result = '', qi = 0
  for (let i = 0; i < safe.length; i++) {
    if (qi < q.length && s[i] === q[qi]) { result += `<mark>${safe[i]}</mark>`; qi++ }
    else result += safe[i]
  }
  return result
}
