// Per-file suggestions of the AI scan for the Add item form (#1626). The scan
// returns, for the files it was given, a document type and a readable title.
// The form shows the type as a file tag the user can remove and puts the
// title into the file's title while the file still has its default title.
//
// Batch intake applies the same suggestions on the server
// (go/services/intake_file_suggestions.go); the mapping from type to tag and
// the confidence floor are the same here.
import type { FileTagPill } from "@/features/files/constants"

import { capFileTitle, defaultFileTitle, fileSignature, type PendingFile } from "./draft"
import type { ScanDocumentType, ScanFileGuess } from "./scanApi"

// The lowest confidence a suggestion is used at, the floor below which the
// review leaves a field unticked too.
export const FILE_SUGGESTION_MIN_CONFIDENCE = 0.3

// The file tag for each document type. A receipt is a proof of purchase like
// an invoice and there is no receipt tag, so it gets the invoice tag. A file
// of another kind gets no tag.
const DOCUMENT_TYPE_TAGS: Partial<Record<ScanDocumentType, FileTagPill["id"]>> = {
  invoice: "invoice",
  receipt: "invoice",
  manual: "manual",
  warranty: "warranty",
  photo: "photo",
}

// The tags that say what kind of document a file is. A file with one of them
// is classified already, and gets no other.
export const DOCUMENT_KIND_TAGS: readonly string[] = ["invoice", "manual", "warranty", "photo"]

// A suggestion for one file the scan was sent: the tag of its document type
// and its title, each present only when the scan was sure enough.
export interface FileSuggestion {
  file: File
  tag?: string
  title?: string
}

// fileSuggestionsFor pairs the scan's file suggestions with the files the
// request sent, in the order it sent them. A guess's index names the file at
// that position of `sent`, also when the server skipped an earlier part (an
// empty file), so the pairing never goes by the position of a file in the
// form's list, which leaves out a file it already holds.
export function fileSuggestionsFor(
  guesses: readonly ScanFileGuess[],
  sent: readonly File[]
): FileSuggestion[] {
  const out: FileSuggestion[] = []
  for (const guess of guesses) {
    const file = sent[guess.index]
    if (!file) continue
    const suggestion: FileSuggestion = { file }
    if (guess.documentType && guess.documentType.confidence >= FILE_SUGGESTION_MIN_CONFIDENCE) {
      suggestion.tag = DOCUMENT_TYPE_TAGS[guess.documentType.value]
    }
    if (guess.title && guess.title.confidence >= FILE_SUGGESTION_MIN_CONFIDENCE) {
      const title = capFileTitle(guess.title.value.trim())
      if (title !== "") suggestion.title = title
    }
    if (suggestion.tag || suggestion.title) out.push(suggestion)
  }
  return out
}

// hasDefaultTitle reports whether a staged file still has the title it was
// staged with: none typed, or the file name without the extension.
export function hasDefaultTitle(entry: PendingFile): boolean {
  const title = entry.title?.trim() ?? ""
  return title === "" || title === defaultFileTitle(entry.file)
}

// applyFileSuggestions puts the suggestions on the staged files they are
// for, found by the file itself. What the user set on a file stays:
//
//   - the tag is added to a file that has none of DOCUMENT_KIND_TAGS, unless
//     the user removed the same suggestion from it before;
//   - the title replaces only the default title.
//
// So a second scan adds nothing to a file the first one classified or named.
// Returns the same array when nothing changes.
export function applyFileSuggestions(
  entries: PendingFile[],
  suggestions: readonly FileSuggestion[]
): PendingFile[] {
  if (suggestions.length === 0) return entries
  const bySignature = new Map<string, FileSuggestion>()
  for (const s of suggestions) {
    const sig = fileSignature(s.file)
    if (!bySignature.has(sig)) bySignature.set(sig, s)
  }
  let changed = false
  const next = entries.map((entry) => {
    const s = bySignature.get(fileSignature(entry.file))
    if (!s) return entry
    let updated = entry
    const classified = entry.tags.some((tag) => DOCUMENT_KIND_TAGS.includes(tag.toLowerCase()))
    if (s.tag && !classified && entry.suggestion?.tag !== s.tag) {
      updated = {
        ...updated,
        tags: [...updated.tags, s.tag],
        suggestion: { ...updated.suggestion, tag: s.tag },
      }
    }
    if (s.title && hasDefaultTitle(entry)) {
      updated = {
        ...updated,
        title: s.title,
        suggestion: { ...updated.suggestion, title: s.title },
      }
    }
    if (updated !== entry) changed = true
    return updated
  })
  return changed ? next : entries
}

// suggestedTagOf is the tag on the file that a scan suggested and the user
// kept, if any.
export function suggestedTagOf(entry: PendingFile): string | undefined {
  const tag = entry.suggestion?.tag
  return tag && entry.tags.includes(tag) ? tag : undefined
}

// hasSuggestedTitle reports whether the file's title is the one a scan
// suggested.
export function hasSuggestedTitle(entry: PendingFile): boolean {
  const title = entry.suggestion?.title
  return !!title && entry.title === title
}
