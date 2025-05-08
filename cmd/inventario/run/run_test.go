package run_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-extras/go-kit/must"
)

func TestRunCommand(t *testing.T) {
	fmt.Println(filepath.ToSlash(filepath.Join(must.Must(os.Getwd()), "uploads")))
}
