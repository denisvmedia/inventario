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
//     the user removed that tag after a scan suggested it;
//   - the title replaces only the default title, and never on a file whose
//     suggested title the user changed, even back to the file name.
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
    if (s.tag && !classified && !rejectedTagsOf(entry).includes(s.tag)) {
      const base = settled(entry)
      updated = {
        ...base,
        tags: [...base.tags, s.tag],
        suggestion: { ...base.suggestion, tag: s.tag },
      }
    }
    if (s.title && hasDefaultTitle(entry) && entry.suggestion?.rejectedTitle === undefined) {
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

// withTags is the staged file with the tags the user set. Removing the tag a
// scan suggested rejects it: no later scan suggests it for the file again,
// and adding it back by hand makes it the user's tag, shown without the
// suggestion's mark.
export function withTags(entry: PendingFile, tags: string[]): PendingFile {
  return settled({ ...entry, tags })
}

// withTitle is the staged file with the title the user typed. Changing the
// title a scan suggested rejects it: no later scan sets a title on the file,
// also when the user goes back to the file name.
export function withTitle(entry: PendingFile, title: string): PendingFile {
  const suggested = entry.suggestion?.title
  if (suggested === undefined || title === suggested) return { ...entry, title }
  const suggestion = { ...entry.suggestion, rejectedTitle: suggested }
  delete suggestion.title
  return { ...entry, title, suggestion }
}

// rejectedTagsOf lists the suggested tags the user removed from the file,
// with the suggested tag the file no longer has.
function rejectedTagsOf(entry: PendingFile): readonly string[] {
  return settled(entry).suggestion?.rejectedTags ?? []
}

// settled records as rejected a suggested tag the file no longer has.
// Returns the entry itself when there is nothing to record.
function settled(entry: PendingFile): PendingFile {
  const tag = entry.suggestion?.tag
  if (tag === undefined || entry.tags.includes(tag)) return entry
  const rejectedTags = entry.suggestion?.rejectedTags ?? []
  const suggestion = {
    ...entry.suggestion,
    rejectedTags: rejectedTags.includes(tag) ? rejectedTags : [...rejectedTags, tag],
  }
  delete suggestion.tag
  return { ...entry, suggestion }
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
