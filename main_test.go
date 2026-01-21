package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"testing"
)

func populateDb[R any](t *testing.T, tbl *Table[R]) {
	instr := []string{
		"insert 1 mashingan hello-world;",
		"insert 2 rdruffy thousand-sunny;",
		"insert 5  rahshingan madoushi;",
		"insert 10  cell dragon;",
	}
	insertDb(t, tbl, instr...)
}

func insert1Row[R any](t *testing.T, tbl *Table[R], insert string) {
	var stmt Statement
	pk := prepareResult(&stmt, insert)
	if pk != PrepareSuccess {
		t.Errorf("failed to read instruction buffer: %s\n", insert)
		return
	}
	cursor, existed, err := tbl.GetCursor(stmt.row.id)
	t.Log("cursor:", cursor)
	if err != nil {
		t.Error(err)
		return
	}
	if cursor != nil {
		page := cursor.table.pages[cursor.pageNum]
		key := binary.LittleEndian.Uint32(page[cursor.pageOffset : cursor.pageOffset+4])
		t.Logf("key: %d, id: %d", key, stmt.row.id)
	}
	if existed {
		t.Errorf("id %d is already existed\n", stmt.row.id)
		return
	}
	if err := cursor.SetRow(stmt.row); err != nil {
		t.Errorf("error insert row: %v", err)
	}
}

func insertDb[R any](t *testing.T, tbl *Table[R], insert ...string) {
	for _, inst := range insert {
		insert1Row(t, tbl, inst)
	}
}

func TestInsert(t *testing.T) {
	const testdb = "test.db"
	if err := os.Remove(testdb); err != nil {
		t.Log("optional os remove error:", err)
	}
	table, err := NewTable[Cell[Row]](testdb)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		table.file.Close()
		if err := os.Remove(testdb); err != nil {
			t.Log("os.remove error:", err)
		}
	}()
	for i := range 5 {
		buf := fmt.Sprintf("insert %d user-%d email-%d;", i, i, i)
		insert1Row(t, table, buf)
	}
}

func TestSelect(t *testing.T) {
	const testdb = "test.db"
	if err := os.Remove(testdb); err != nil {
		t.Log("optional os remove error:", err)
	}
	table, err := NewTable[Cell[Row]](testdb)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		table.file.Close()
		if err := os.Remove(testdb); err != nil {
			t.Log("os.remove error:", err)
		}
	}()
	populateDb(t, table)
	for _, row := range table.selectRow(nil) {
		t.Logf("(%d, %s, %s)\n", row.id,
			row.username, row.email)
	}

	query := "select 5"
	var stmt Statement
	pk := prepareResult(&stmt, query)
	if pk != PrepareSuccess {
		t.Fatal("expected query select")
	}
	cursor, found, err := table.GetCursor(stmt.row.id)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !found {
		t.Fatal("expected to find cursor")
	}
	row, found := cursor.Row()
	if !found {
		t.Fatal("expected to find row")
	}
	if row.id != stmt.row.id {
		t.Fatalf("expected to find row id %d, got %d", stmt.row.id, row.id)
	}
}

func TestInsertErrorMaxIndex(t *testing.T) {
	const testdb = "test.db"
	if err := os.Remove(testdb); err != nil {
		t.Log("optional os remove error:", err)
	}
	table, err := NewTable[Cell[Row]](testdb)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		table.file.Close()
		if err := os.Remove(testdb); err != nil {
			t.Log("os.remove error:", err)
		}
	}()
	buf := fmt.Sprintf("insert %d user-%d email-%d;", 13, 13, 13)
	insert1Row(t, table, buf)
}
