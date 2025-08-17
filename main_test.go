package main

import (
	"fmt"
	"testing"
)

func populateDb(t *testing.T) {
	instr := []string{
		"insert 1 mashingan hello-world;",
		"insert 2 rdruffy thousand-sunny;",
		"insert 5  rahshingan madoushi;",
	}
	insertDb(t, instr...)
}

func insert1Row(t *testing.T, stmt *Statement, insert string) {
	pk := prepareResult(stmt, insert)
	if pk != PrepareSuccess {
		t.Errorf("failed to read instruction buffer: %s\n", insert)
		return
	}
	n, err := executeInsert(stmt, &pages)
	if n <= 0 {
		t.Error("no insert, expected insert 1 row")
		return
	}
	if err != nil {
		t.Error("expecting no error, got:", err)
		return
	}
}

func insertDb(t *testing.T, insert ...string) {
	var stmt Statement
	for _, inst := range insert {
		insert1Row(t, &stmt, inst)
	}
}

func TestInsert(t *testing.T) {
	clear(pages)
	populateDb(t)

	clear(pages)
	pages = nil
	var stmt Statement
	for i := range 1400 {
		buf := fmt.Sprintf("insert %d user-%d email-%d;", i, i, i)
		insert1Row(t, &stmt, buf)
	}
	t.Log("total pages:", len(pages))
	if len(pages) != 100 {
		t.Error("expected db has 100 pages, got:", len(pages))
	}
}
