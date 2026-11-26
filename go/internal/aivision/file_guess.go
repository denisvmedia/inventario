package aivision

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// FileLabel is the text that precedes the file at index in the request a
// provider sends, so the model can tell which file a FileGuess is about.
func FileLabel(index int) string {
	return fmt.Sprintf("File %d", index)
}

// FillDocumentType sets the scan-level document type from Files when the
// result has none: the type of the first document among the files in
// request order, or "other" when they are photos and other files only. The
// fields map is copied first, since a provider may share it with Items[0].
func (r *ScanResult) FillDocumentType() {
	if r == nil {
		return
	}
	if _, ok := r.Fields[FieldNameDocumentType]; ok {
		return
	}
	guess, ok := documentTypeFromFiles(r.Files)
	if !ok {
		return
	}
	fields := maps.Clone(r.Fields)
	if fields == nil {
		fields = map[string]FieldGuess{}
	}
	fields[FieldNameDocumentType] = guess
	r.Fields = fields
}

func documentTypeFromFiles(files []FileGuess) (FieldGuess, bool) {
	var fallback *FieldGuess
	for _, f := range files {
		if f.DocumentType == nil {
			continue
		}
		docType, _ := f.DocumentType.Value.(string)
		switch docType {
		case "":
		case DocumentTypePhoto, DocumentTypeOther:
			if fallback == nil {
				fallback = &FieldGuess{Value: DocumentTypeOther, Confidence: f.DocumentType.Confidence}
			}
		default:
			return FieldGuess{Value: docType, Confidence: f.DocumentType.Confidence}, true
		}
	}
	if fallback == nil {
		return FieldGuess{}, false
	}
	return *fallback, true
}

// rawFileGuess mirrors one entry of the files list as the model sends it.
type rawFileGuess struct {
	Index        json.RawMessage `json:"index"`
	DocumentType *rawFieldGuess  `json:"document_type"`
	Title        *rawFieldGuess  `json:"title"`
}

// decodeFileGuesses checks the model's file suggestions against the request
// and returns those it can use, ordered by index. An entry is dropped when it
// names no attached file (its index is missing, not a whole number, or not
// below fileCount), when it repeats the index of an earlier entry, or when it
// is left with neither a document type nor a title. A document type outside
// FileDocumentTypes and an empty title are left out of an entry that keeps
// the other. A title is cleaned by CleanFileTitle. Each problem becomes a
// "low_confidence" warning on FieldNameFiles; none fails the scan.
func decodeFileGuesses(raw json.RawMessage, fileCount int) ([]FileGuess, []Warning) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, []Warning{fileWarning("the file suggestions are not a list")}
	}
	var (
		out      []FileGuess
		warnings []Warning
		seen     = make(map[int]bool, len(entries))
	)
	for pos, entry := range entries {
		guess, problems := decodeFileGuess(entry, fileCount)
		usable := guess.Index >= 0 && (guess.DocumentType != nil || guess.Title != nil)
		if guess.Index >= 0 && seen[guess.Index] {
			problems = []string{fmt.Sprintf("file %d is suggested twice", guess.Index)}
			usable = false
		}
		if !usable {
			if len(problems) == 0 {
				problems = []string{"no document type or title"}
			}
			warnings = append(warnings, fileWarning(fmt.Sprintf("files[%d] dropped: %s", pos, strings.Join(problems, ", "))))
			continue
		}
		for _, p := range problems {
			warnings = append(warnings, fileWarning(fmt.Sprintf("files[%d]: %s left out", pos, p)))
		}
		seen[guess.Index] = true
		out = append(out, guess)
	}
	slices.SortFunc(out, func(a, b FileGuess) int { return cmp.Compare(a.Index, b.Index) })
	return out, warnings
}

// decodeFileGuess decodes one entry of the files list. An entry that names
// no attached file comes back with Index -1. problems describe what was left
// out.
func decodeFileGuess(raw json.RawMessage, fileCount int) (FileGuess, []string) {
	var r rawFileGuess
	if err := json.Unmarshal(raw, &r); err != nil {
		return FileGuess{Index: -1}, []string{"not an object"}
	}
	index, ok := decodeFileIndex(r.Index, fileCount)
	if !ok {
		return FileGuess{Index: -1}, []string{"no index of an attached file"}
	}
	guess := FileGuess{Index: index}
	var problems []string
	if r.DocumentType != nil {
		if v, ok := decodeEnum(r.DocumentType.Value, FileDocumentTypes); ok {
			guess.DocumentType = &FieldGuess{Value: v, Confidence: r.DocumentType.Confidence}
		} else {
			problems = append(problems, "unknown document type")
		}
	}
	if r.Title != nil {
		if v, ok := decodeFileTitle(r.Title.Value); ok {
			guess.Title = &FieldGuess{Value: v, Confidence: r.Title.Confidence}
		} else {
			problems = append(problems, "empty title")
		}
	}
	return guess, problems
}

func decodeFileIndex(raw json.RawMessage, fileCount int) (int, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, false
	}
	var n float64
	if json.Unmarshal(raw, &n) != nil || n != math.Trunc(n) || n < 0 || n >= float64(fileCount) {
		return 0, false
	}
	return int(n), true
}

// decodeEnum reads a string that must be one of values, ignoring case and
// surrounding spaces.
func decodeEnum(raw json.RawMessage, values []string) (string, bool) {
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return "", false
	}
	s = strings.ToLower(strings.TrimSpace(s))
	return s, slices.Contains(values, s)
}

func decodeFileTitle(raw json.RawMessage) (string, bool) {
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return "", false
	}
	s = CleanFileTitle(s)
	return s, s != ""
}

// CleanFileTitle returns s as a file title: format characters (bidi
// overrides, zero-width spaces and joiners) are removed, control characters
// and runs of whitespace become one space, the ends are trimmed, and the
// title is cut to MaxFileTitleRunes.
func CleanFileTitle(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case unicode.Is(unicode.Cf, r):
			return -1
		case unicode.IsControl(r):
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > MaxFileTitleRunes {
		s = strings.TrimSpace(string([]rune(s)[:MaxFileTitleRunes]))
	}
	return s
}

func fileWarning(detail string) Warning {
	return Warning{Code: "low_confidence", Field: FieldNameFiles, Detail: detail}
}
