// The server counts a text length limit in characters (Unicode code points,
// Go runes). zod's string .max() counts code points too, so a schema can use
// it as is. String.length, String.slice and the HTML maxLength attribute count
// UTF-16 units instead: an emoji counts twice, and a cut can split one in half.

// capCodePoints keeps at most max code points of value. Use it to cap text
// the way the server counts, in place of value.slice(0, max) or maxLength.
export function capCodePoints(value: string, max: number): string {
  const chars = Array.from(value)
  return chars.length <= max ? value : chars.slice(0, max).join("")
}
