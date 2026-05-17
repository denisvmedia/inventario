// TanStack Query mutation for the feedback submission slice (#1387).
import { useMutation } from "@tanstack/react-query"

import { submitFeedback, type SubmitFeedbackInput } from "./api"

// useSubmitFeedback is a single-shot mutation — there is no list query
// to invalidate. The FE surfaces success/error through the returned
// mutation state; the SettingsPage row renders a toast in onSuccess /
// onError of the mutation call site.
export function useSubmitFeedback() {
  return useMutation({
    mutationFn: (input: SubmitFeedbackInput) => submitFeedback(input),
  })
}
