// Client-side matching that follows the server's search rule
// (registry.SplitSearchTerms and GET /search?type=locations): white space
// separates terms, double quotes keep a phrase together, and every term has
// to be found, ignoring case, in at least one field.

export function splitSearchTerms(query: string): string[] {
  const terms: string[] = []
  const seen = new Set<string>()
  const add = (raw: string) => {
    const term = raw.trim()
    if (!term) return
    const key = foldCase(term)
    if (seen.has(key)) return
    seen.add(key)
    terms.push(term)
  }
  let current = ""
  let quoted = false
  for (const char of query) {
    if (char === '"') {
      add(current)
      current = ""
      quoted = !quoted
    } else if (/\p{Cc}/u.test(char) || (!quoted && /\s/u.test(char))) {
      add(current)
      current = ""
    } else {
      current += char
    }
  }
  add(current)
  return terms
}

// Lower case, with the final sigma folded to σ as Unicode case folding
// does, so "ΟΔΌΣ" finds "Οδός" in the middle of a name too.
export function foldCase(text: string): string {
  return text.toLowerCase().replaceAll("ς", "σ")
}

// No terms match everything.
export function matchesEveryTerm(
  terms: readonly string[],
  fields: ReadonlyArray<string | null | undefined>
): boolean {
  const folded = fields.map((field) => foldCase(field ?? ""))
  return terms.every((term) => {
    const needle = foldCase(term)
    return folded.some((field) => field.includes(needle))
  })
}
