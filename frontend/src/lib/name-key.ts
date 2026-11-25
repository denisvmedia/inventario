// The characters Go's unicode.IsSpace accepts, which the server splits a
// name on. JavaScript's \s differs: it lacks U+0085 and has U+FEFF.
const SERVER_SPACE = /[\t\n\v\f\r \u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]+/gu

// foldChar folds one code point the way golang.org/x/text/cases.Fold does.
// Lowercasing, uppercasing and lowercasing again yields the full case
// folding for nearly every character (ß, ẞ and the ligatures expand,
// final sigma becomes σ). The exceptions are spelled out: the dotless ı has
// no folding, and x/text folds the Cherokee small letters to capitals.
function foldChar(c: string): string {
  const cp = c.codePointAt(0) ?? 0
  if (cp === 0x131) return c
  if (cp >= 0xab70 && cp <= 0xabbf) return String.fromCodePoint(cp - 0xab70 + 0x13a0)
  if (cp >= 0x13f8 && cp <= 0x13fd) return String.fromCodePoint(cp - 8)
  return c.toLowerCase().toUpperCase().toLowerCase()
}

// nameKey compares names the way the server's uniqueness rules do
// (models.SellerNameKey, models.ItemTypeNameKey): NFC, full case folding,
// white space trimmed and collapsed. Checked against the server for every
// code point: the only differences are letters newer than the browser's
// Unicode tables, which the browser leaves unfolded. For those the server's
// own check still applies.
export function nameKey(name: string): string {
  const folded = Array.from(name.normalize("NFC"), foldChar).join("").normalize("NFC")
  return folded.replace(SERVER_SPACE, " ").replace(/^ | $/g, "")
}
