package typer

import (
	"fmt"
	"reflect"
	"testing"
)

type AA interface {
	AA()
}

type BB interface {
	BB()
}

type aa struct{}

func (a *aa) AA() {
	fmt.Printf("aa")
}

func (a *aa) BB() {
	fmt.Printf("bb")
}

type CC interface {
	AA
}

type cc struct {
	AA
}

var BBType = reflect.TypeOf((*BB)(nil)).Elem()

func TestType(t *testing.T) {
	a := &aa{}
	c := &cc{AA: a}

	if bb, ok := ConvertibleTo(c, BBType); ok {
		bb.(BB).BB()
	}
}
