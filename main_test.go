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
	cursor, _ := tbl.GetCursor(stmt.row.id)
	if err := cursor.SetRow(stmt.row); err != nil {
		t.Errorf("error insert row: %v", err)
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
	if err := os.Remove(testdb); err != nil {
		t.Log("optional os remove error:", err)
	}
	table, err := NewTable[Row](testdb)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		table.file.Close()
		if err := os.Remove(testdb); err != nil {
			t.Log("os.remove error:", err)
		}
	}()
	var stmt Statement
	for i := 1; i <= 5; i++ {
		buf := fmt.Sprintf("insert %d user-%d email-%d;", i, i, i)
		insert1Row(t, table, &stmt, buf)
	}
}
