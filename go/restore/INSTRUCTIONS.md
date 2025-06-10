# Importing XML Backups

## Current State

We have import in xml format (see [xml export schema definition](../export/schemadef/export.xsd)). It contains
database entries as well as binary data encoded in base64. Binary data is sourced from files attached to commodities.
There are three types of files: images, invoices, and manuals.

## Goal

We need to support importing xml backups we made previously. Note, xml file can be extremely large, thus, you will have
to process the incoming stream with the file data on-the-fly following the instructions.

---

## Deferred Import Workflow with Metadata Extraction

Given the possibility of massive XML files and the need for safety and transparency, the import process follows a two-phase approach:  
**1. Upload & Metadata Extraction, 2. User Review & Final Import.**

### 1. Upload Phase and Metadata Extraction

- **Upload Handling:**  
  The uploaded XML backup is saved as-is to a temporary or staging location (disk, S3, etc.).

- **Streaming Parse for Metadata:**  
  During the upload, the system parses the XML stream (using a token-based approach) to extract high-level metadata without fully materializing all entities or files.

- **Metadata Storage:**  
  Extracted metadata is stored in an `Uploads` table (or similar structure) in the database. Examples of metadata:
  - Entity counts by type (commodities, users, files, etc.)
  - List of files/blobs referenced (filenames, sizes, types)
  - IDs/keys of entities in the backup (for later diff/merge/preview)
  - Backup creation time, version, user info
  - Any warnings or detected inconsistencies

- **File Manifest (Optional):**  
  Binary blobs (base64 data) can be pre-extracted to temporary files during this phase, with their paths and metadata recorded.

### 2. User Review & Import Decision

- **Preview & Validation:**  
  The user is presented with a summary of the backup contents (entities, files, potential conflicts, etc.) before any database changes are made.

- **Import Strategy Selection:**  
  The user chooses their desired import strategy (see below) with clear explanations and previews of each option's effect.

- **Import Execution:**  
  When confirmed, the system processes the staged XML file using the selected strategy. Metadata guides the process for efficiency and safety. Upon completion, temp files are cleaned up and the `Uploads` entry is updated.

---

## Database Restore Strategies

When restoring from a backup, the database may have diverged (e.g., changes were made since the backup was taken). Multiple restore strategies are supported:

- **Full Replace (Destructive Restore):**
  - Wipe the current database and restore everything from the backup.
  - *Use with caution: all changes since the backup will be lost.*

- **Merge (Additive Import):**
  - Only add data from the backup that is missing in the current DB (matched by primary key or unique fields).

- **Merge (Update Existing, Keep Unmatched):**
  - For each record in the backup, create if missing, update if exists (by unique key), leave other records untouched.

- **Merge (Replace + Preserve Extras):**
  - For each entity, replace matching records (by key) with backup versions; keep any DB records not in backup.

- **Abort on Divergence:**
  - Detect if the DB state has diverged and abort restore unless the user forces it.

- **User Choice:**
  - The user is presented with these strategies at restore time, with previews and plain-language explanations.

---

## Instructions for Streaming XML with Embedded Base64 in Go

This guide outlines how to process a giant XML file in Go where some sections contain base64-encoded binary data.
The goal is to:

* Stream the XML file without loading it into memory
* Detect and handle base64-encoded binary sections
* Decode them in chunks and write the output to separate files

### 1. Stream the XML File

* Open the XML file using `os.Open()`.
* Create an XML decoder with `xml.NewDecoder(file)` to avoid full in-memory loading.

### 2. Token-based Parsing

* Loop over the tokens using `decoder.Token()`.
* For each `xml.StartElement`, check if it matches a tag like `<data>`.
* Optionally extract attributes (e.g., filename).

### 3. Handling Base64 Data

* Once the desired tag is found, call a function like `decodeBase64ToFile()`.
* Inside this function:

  * Create a file with `os.Create()`.
  * Use `io.Pipe` to connect the XML reading stream to the base64 decoder.
  * Start a goroutine to read tokens, writing only `xml.CharData` to the pipe.
  * Exit when the closing tag (e.g., `</data>`) is reached.

### 4. Decoding in Chunks

* Wrap the pipe reader with `base64.NewDecoder()`.
* Use a loop with a 32KB buffer to read and decode.
* Write decoded chunks directly to the output file.

### 5. Continue Processing

* After processing one `<data>`, resume parsing tokens.
* Repeat the process for each binary section found.

### 6. Error Handling

* Handle errors at all I/O and decoding steps.
* Ensure all files and pipes are properly closed.

---

### Example Code

```go
package main

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"os"
)

func main() {
	file, err := os.Open("dump.xml")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	decoder := xml.NewDecoder(file)

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "FileData" {
				var filename string
				for _, attr := range t.Attr {
					if attr.Name.Local == "filename" {
						filename = attr.Value
					}
				}
				if filename == "" {
					filename = "output.dat"
				}
				if err := decodeBase64ToFile(decoder, filename); err != nil {
					panic(err)
				}
			}
		}
	}
}

func decodeBase64ToFile(decoder *xml.Decoder, filename string) error {
	outFile, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer outFile.Close()

	pipeReader, pipeWriter := io.Pipe()

	go func() {
		defer pipeWriter.Close()
		for {
			tok, err := decoder.Token()
			if err != nil {
				pipeWriter.CloseWithError(err)
				return
			}
			switch token := tok.(type) {
			case xml.CharData:
				if _, err := pipeWriter.Write([]byte(token)); err != nil {
					pipeWriter.CloseWithError(err)
					return
				}
			case xml.EndElement:
				if token.Name.Local == "FileData" {
					return
				}
			default:
				pipeWriter.CloseWithError(fmt.Errorf("unexpected token in base64 content"))
				return
			}
		}
	}()

	b64Decoder := base64.NewDecoder(base64.StdEncoding, pipeReader)
	buffer := make([]byte, 32*1024)
	for {
		n, err := b64Decoder.Read(buffer)
		if n > 0 {
			if _, err := outFile.Write(buffer[:n]); err != nil {
				return err
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}
```

---

## UI/UX Guidelines

- **Clear Restore Options:**  
  Present restore strategies with plain language explanations and warnings about data loss for destructive options.

- **Preview Changes:**  
  Show a summary of what will happen: records to be added, updated, deleted, or left untouched. Highlight potential conflicts or duplicates.

- **Progress & Feedback:**  
  Display progress for each step (parsing, extracting, validating, importing). Indicate current action and estimated time.

- **Error Handling:**  
  Clearly display errors, reasons for aborting, and options to retry or roll back.

- **Confirmation & Undo:**  
  Require explicit confirmation for destructive actions. Offer undo/rollback if possible.

- **Help & Documentation:**  
  Provide inline help/tooltips for each option and link to full documentation.

---

## Summary

- Use deferred two-step import: save backup and extract metadata first, then let user choose and finalize the import.
- Support multiple restore strategies with clear UI and validation.
- Use efficient streaming and chunked decoding for XML with embedded base64 data in Go.
- Prioritize transparency, safety, and user control throughout the process.
