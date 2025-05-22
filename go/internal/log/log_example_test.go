package log_test

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/denisvmedia/inventario/internal/log"
)

var _ logrus.Formatter = (*simpleFormatter)(nil)

type simpleFormatter struct {
}

func (*simpleFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	return []byte(strings.ToUpper(entry.Level.String()) + " " + entry.Message), nil
}

func ExampleTracef() {
	logrus.SetLevel(logrus.TraceLevel)
	logrus.SetOutput(os.Stdout)
	logrus.SetFormatter(&simpleFormatter{})

	log.Tracef("Hello %s", "World")
	// Output:
	// TRACE Hello World
}
