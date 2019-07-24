package datastructure

import (
	"fmt"
	"testing"
)
import "runtime"
import "unsafe"
import "errors"
import "time"

func TestWeakmap(t *testing.T) {
	generateA(6)
	fmt.Println("")

	debugWeakMap(6)
	fmt.Println("")

	runtime.GC()
	fmt.Println("GC runned.")

	time.Sleep(1 * time.Second)

	debugWeakMap(6)
}

func generateA(n int) {

	for i := 0; i < n; i++ {
		a := NewA()
		WMap.Add(a)
		fmt.Println("Added to WM A with id =", a.Id)
	}

}

func debugWeakMap(n int) {
	for i := 0; i < n; i++ {
		if WMap.Has(i) {
			fmt.Println("Has id =", i)
		} else {
			fmt.Println("Hasn't id =", i)
		}
	}
}

var aId int

func NewA() *A {
	aId++
	return &A{aId}
}

var WMap = &WeakMap{weakMap: make(map[int]uintptr)}

type WeakMap struct {
	weakMap map[int]uintptr
}

type A struct {
	Id int
}

func (w *WeakMap) Add(a *A) {
	runtime.SetFinalizer(a, finalizer)

	w.weakMap[a.Id] = uintptr(unsafe.Pointer(a))
}

func (w *WeakMap) Get(id int) (*A, error) {
	if !w.Has(id) {
		return nil, errors.New("")
	}

	a := (*A)(unsafe.Pointer(w.weakMap[id]))

	return a, nil
}

func (w *WeakMap) Has(id int) bool {
	_, ok := w.weakMap[id]
	return ok
}

func (w *WeakMap) Remove(a *A) {
	delete(w.weakMap, a.Id)
}

func finalizer(a *A) {
	fmt.Println(a.Id)
	WMap.Remove(a)
}
