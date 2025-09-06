package input

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// InputField represents a field that can collect user input
type InputField interface {
	Prompt(ctx context.Context) (any, error)
}

// Reader handles input reading with proper writer output
type Reader struct {
	input   io.Reader
	output  io.Writer
	scanner *bufio.Scanner
}

// NewReader creates a new Reader with the specified input and output
func NewReader(input io.Reader, output io.Writer) *Reader {
	return &Reader{
		input:   input,
		output:  output,
		scanner: bufio.NewScanner(input),
	}
}

// NewDefaultReader creates a Reader with stdin/stdout
func NewDefaultReader() *Reader {
	return NewReader(os.Stdin, os.Stdout)
}

// readParamFromStdin reads a parameter from stdin
func (r *Reader) readParamFromStdin(response *string, prompt string) error {
	fmt.Fprint(r.output, prompt)

	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		return fmt.Errorf("failed to read input")
	}

	*response = strings.TrimSpace(r.scanner.Text())
	return nil
}

// readPasswordFromStdin reads a password with hidden input
func (r *Reader) readPasswordFromStdin(prompt string) (string, error) {
	fmt.Fprint(r.output, prompt)

	// Try to get file descriptor for terminal input
	var fd int
	if f, ok := r.input.(*os.File); ok {
		fd = int(f.Fd())
	} else {
		fd = int(syscall.Stdin)
	}

	bytePassword, err := term.ReadPassword(fd)
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}

	fmt.Fprintln(r.output) // Add newline after password input
	return string(bytePassword), nil
}

// readIntFromStdin reads an integer from stdin
func (r *Reader) readIntFromStdin(response *int, prompt string) error {
	var strResponse string
	err := r.readParamFromStdin(&strResponse, prompt)
	if err != nil {
		return err
	}

	if strResponse == "" {
		return NewAnswerError("") // Empty response, special error to indicate empty
	}

	parsed, err := strconv.Atoi(strResponse)
	if err != nil {
		return NewAnswerError("Please enter a valid number")
	}

	*response = parsed
	return nil
}
