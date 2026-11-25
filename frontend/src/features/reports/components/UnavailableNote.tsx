// UnavailableNote is the small line in a report cell whose value could not
// be loaded. It stays off paper, so a printed report shows the cell empty.
export function UnavailableNote({ text }: { text: string }) {
  return (
    <p
      className="print-hide mt-0.5 text-xs text-muted-foreground print:hidden"
      data-testid="report-value-unavailable"
    >
      {text}
    </p>
  )
}
