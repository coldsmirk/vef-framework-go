package unusedrecv

type T struct{ v int }

func (t *T) Named() string { return "named" } // want `receiver "t" is unused`

func (_ *T) Blank() string { return "blank" } // want `receiver "_" is unused`

func (*T) Omitted() string { return "omitted" }

func (t T) Value() string { return "value" } // want `receiver "t" is unused`

func (t *T) Used() int { return t.v }

// Shadowed rebinds the receiver's name, which resolution by type information
// must see through: the receiver itself is still unused.
func (t *T) Shadowed() string { // want `receiver "t" is unused`
	if t := "shadow"; t != "" {
		return t
	}

	return ""
}

func Plain() string { return "plain" }
