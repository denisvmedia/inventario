package errkit

import (
	"fmt"
)

const badKey = "!BADKEY"

func ToFields(fields []any) Fields {
	if len(fields) == 0 {
		return nil
	}

	if len(fields) == 1 {
		if fm, ok := fields[0].(Fields); ok {
			fs := make(Fields, len(fm))
			for k, v := range fm {
				fs[k] = v
			}
			return fs
		}
	}

	if len(fields)%2 != 0 {
		fields = append(fields[:len(fields)-1], badKey, fields[len(fields)-1])
	}

	fs := make(Fields, len(fields)/2)

	for i := 0; i < len(fields); i += 2 {
		f := fields[i]
		fstr, ok := f.(string)
		if !ok {
			fstr = fmt.Sprintf("%v(%v)", badKey, f)
		}

		fs[fstr] = fields[i+1]
	}

	return fs
}
