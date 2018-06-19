package mysqldump_loader

import (
	"io"
)

type insertion struct {
	ignore  bool
	r       io.Reader
	replace bool
	table   string
}
