package assert

import (
	"github.com/denisvmedia/inventario/internal/log"
)

func NoError(err error) {
	if err != nil {
		log.WithError(err).Error("unexpected error")
		panic(err)
	}
}
