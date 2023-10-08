package errkit

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
		panic("errkit: invalid fields (must be odd count)")
	}

	fs := make(Fields, len(fields)/2)

	for i := 0; i < len(fields); i += 2 {
		f := fields[i]
		fstr, ok := f.(string)
		if !ok {
			panic("errkit: invalid field key (must be string)")
		}

		fs[fstr] = fields[i+1]
	}

	return fs
}
