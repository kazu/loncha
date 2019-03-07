package base_template

type EqInt map[string]int
type EqString map[string]string

type Eq map[string]interface{}

type Op map[string]interface{}

// sample
type GEN_ELEMENT struct {
	ID   int
	Name string
}

type CondFunc func(int) bool
