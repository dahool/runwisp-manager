package tomledit

import "errors"

// Kind identifies an indexed TOML construct.
type Kind uint8

const (
	KindKeyValue Kind = iota + 1
	KindTable
	KindArrayTable
	KindComment
)

// Node is an indexed source construct. Spans use byte offsets and are half-open.
type Node struct {
	Kind   Kind
	Path   []string
	Span   [2]int
	Key    [2]int
	Value  [2]int
	Header [2]int
}

var (
	ErrUnsupportedLayout = errors.New("tomledit: unsupported layout")
	ErrSemanticMismatch  = errors.New("tomledit: semantic mismatch")
	ErrNotFound          = errors.New("tomledit: path not found")
)

// Op is a TOML edit operation.
type Op interface{ isOp() }

type SetValue struct {
	Path  []string
	Value any
}

func (SetValue) isOp() {}

type AddKey struct {
	Table         []string
	Key           string
	Value         any
	Before, After string
}

func (AddKey) isOp() {}

type DeleteKey struct{ Path []string }

func (DeleteKey) isOp() {}

type KV struct {
	Key   string
	Value any
}
type AddTable struct {
	Path     []string
	Array    bool
	Body     []KV
	Position int
}

func (AddTable) isOp() {}

type DeleteTable struct {
	Path  []string
	Index int
}

func (DeleteTable) isOp() {}

type RenameTable struct{ From, To []string }

func (RenameTable) isOp() {}

type ReorderArrayTable struct {
	Path     []string
	From, To int
}

func (ReorderArrayTable) isOp() {}
