package main

import (
	"fmt"
	"os"
	"testing"
)

func populateDb[R any](t *testing.T, tbl *Table[R]) {
	instr := []string{
		"insert 1 mashingan hello-world;",
		"insert 2 rdruffy thousand-sunny;",
		"insert 5  rahshingan madoushi;",
	}
	insertDb(t, tbl, instr...)
}

func insert1Row[R any](t *testing.T, tbl *Table[R], stmt *Statement, insert string) {
	pk := prepareResult(stmt, insert)
	if pk != PrepareSuccess {
		t.Errorf("failed to read instruction buffer: %s\n", insert)
		return
	}
	n, err := tbl.insertRow(stmt)
	if n <= 0 {
		t.Error("no insert, expected insert 1 row")
		return
	}
	if err != nil {
		t.Error("expecting no error, got:", err)
		return
	}
}

func insertDb[R any](t *testing.T, tbl *Table[R], insert ...string) {
	var stmt Statement
	for _, inst := range insert {
		insert1Row(t, tbl, &stmt, inst)
	}
}

func TestInsert(t *testing.T) {
	const testdb = "test.db"
	os.Remove(testdb)
	table, err := NewTable[Row](testdb)
	defer func() {
		if err := os.Remove(testdb); err != nil {
			t.Log("os.remove error:", err)
		}
	}()
	if err != nil {
		t.Fatal(err)
	}
	populateDb(t, table)

	table.pages = nil
	var stmt Statement
	for i := range 1400 {
		buf := fmt.Sprintf("insert %d user-%d email-%d;", i, i, i)
		insert1Row(t, table, &stmt, buf)
	}
	t.Log("total pages:", len(table.pages))
	if len(table.pages) != 100 {
		t.Error("expected db has 100 pages, got:", len(table.pages))
	}
}
