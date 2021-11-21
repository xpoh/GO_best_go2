package main

import (
	"testing"
)

func TestGetFileList(t *testing.T) {
	a := app{}
	if a.InitApp() != nil {
		panic("Error init dup_cleaner")
	}
	defer a.CloseApp()

	list, err := a.getFileList("./")
	if err != nil {
		t.Error("err!=nil")
	}
	if len(list) != 3 {
		t.Error("len(list)!=3")
	}
	if list[0].fs.Name() != "dup_cleaner.go" {
		t.Error("list[0].fs.Name()!=dup_cleaner.go")
	}
	if list[1].fs.Name() != "dup_cleaner_test.go" {
		t.Error("list[1].fs.Name()!=dup_cleaner_test.go")
	}
}
