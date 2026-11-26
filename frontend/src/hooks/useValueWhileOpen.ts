import { useState } from "react"

// useValueWhileOpen returns `value` while `open` is true and, after `open`
// turns false, the last value it had while open. Radix keeps a dialog's
// content on screen for the close animation, and the parent often clears the
// record the dialog shows in the same update that closes it; the dialog reads
// the record through this hook so it does not switch to an empty create form
// while it slides away. The held value is adjusted during render, React's
// pattern for state that follows a prop.
export function useValueWhileOpen<T>(open: boolean, value: T): T {
  const [held, setHeld] = useState(value)
  if (open && !Object.is(held, value)) setHeld(value)
  return open ? value : held
}
