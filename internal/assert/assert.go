package assert

import (
	"github.com/denisvmedia/inventario/internal/log"
)

func NoError(err error) {
	if err != nil {
		log.WithError(err).Fatal("unexpected error")
		panic(err)
	}
}
