package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"unsafe"

	"github.com/mashingan/bitfield"
	"golang.org/x/exp/constraints"
)

func main() {
	scn := bufio.NewScanner(os.Stdin)
	isString := false
	buffer := ""
	prompt := "> "
	var (
		stmt Statement
	)
	table, err := NewTable[Cell[Row]]("from-scratch.db")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := table.flushPages(); err != nil {
			log.Println(err)
		}
		table.Close()
	}()
	clearPrompt := func() {
		buffer = ""
		prompt = "> "
	}
	for {
		fmt.Print(prompt)
		if !scn.Scan() {
			break
		}
		line := scn.Text()
		if !isString && strings.HasSuffix(line, ";") {
			line = strings.TrimRight(line, ";")
			buffer += line
			fmt.Println("buffer:", buffer)
			pk := prepareResult(&stmt, buffer)
			if pk != PrepareSuccess {
				fmt.Println("unknown statement query")
			} else {
				fmt.Println("select statement:", statementMap[stmt.kind])
				table.Execute(&stmt)
			}
			clear(stmt.row.username[:])
			clear(stmt.row.email[:])
			clearPrompt()
			continue
		}
		if buffer == "" && strings.HasPrefix(line, ".") {
			_, cont := handleMetaCommand(line, table)
			if !cont {
				break
			}
			clearPrompt()
			continue
		}
		buffer += line + " "
		prompt = ".  "
	}
}

func handleMetaCommand[R any](cmd string, tbl *Table[R]) (MetaCommand, bool) {
	rcmd := MetaCommandUnrecognized
	cont := true
	switch cmd {
	case ".quit", ".q":
		rcmd = MetaCommandSuccess
		cont = false
	case ".pages", ".p":
		rcmd = MetaCommandSuccess
		for _, page := range tbl.pages {
			lp := len(page)
			fmt.Println("total pages length:", lp)
			rows := binary.LittleEndian.Uint16(page[4:6])
			for i := 0; i < int(rows); i++ {
				row := i
				idpos := 6 + (i * int(tbl.rowSize))
				unamepos := idpos + 4
				emailpos := unamepos + 32
				log.Println("idpos:", idpos)
				log.Println("unamepos:", unamepos)
				log.Println("emailpos:", emailpos)
				if unamepos >= lp || emailpos >= lp || emailpos+255 >= lp {
					log.Println("any endpoint more than pages. ignore!")
					break
				}
				id := binary.LittleEndian.Uint32(page[idpos:unamepos])
				fmt.Printf("row %d: (id: %d, username: %q, email: %q)\n",
					row, id, page[unamepos:emailpos], page[emailpos:emailpos+255])
			}

		}
	}
	return rcmd, cont
}

func prepareResult(stmt *Statement, bfr string) PrepareKind {
	prepare := PrepareUnknown
	stmtKind := StatementSelect
	bfr = strings.ToLower(bfr)
	if strings.HasPrefix(bfr, "select") {
		prepare = PrepareSuccess
	} else if strings.HasPrefix(bfr, "insert") {
		stmtKind = StatementInsert
		prepare = PrepareSuccess
		var email, uname string
		n, err := fmt.Sscanf(bfr, "insert %d %s %s", &stmt.row.id,
			&uname, &email)

		// buname := unsafe.StringData(uname)
		// stmt.row.username = *(*[32]byte)(unsafe.Pointer(&unsafe.Slice(buname, 32)[0]))
		// stmt.row.email = *(*[255]byte)(unsafe.Pointer(unsafe.StringData(email)))
		copy(stmt.row.username[:], []byte(uname))
		copy(stmt.row.email[:], []byte(email))
		if err != nil {
			log.Println(err)
		}
		if n != 3 {
			log.Println("err: supplied command is not exactly 3, got:", n)
		}
	}
	stmt.kind = stmtKind
	return prepare
}

func (tbl *Table[R]) insertRow(stmt *Statement) (int, error) {
	nullUname := bytes.IndexByte(stmt.row.username[:], '\x00')
	nullEmail := bytes.IndexByte(stmt.row.email[:], '\x00')
	fmt.Printf("insert exec row id: %d, username: %s, email: %s\n", stmt.row.id,
		stmt.row.username[:nullUname], stmt.row.email[:nullEmail])
	thetable[stmt.row.id] = stmt.row
	page, pg := tbl.GetPage()
	id := make([]byte, 4)
	binary.LittleEndian.PutUint32(id, stmt.row.id)
	idpos := 6 + uint32(pg.rows*uint16(tbl.rowSize))
	idoff := unsafe.Sizeof(stmt.row.id)
	binary.LittleEndian.PutUint32(page[idpos:idpos+uint32(idoff)], stmt.row.id)
	unamepos := idpos + uint32(unsafe.Offsetof(stmt.row.username))
	copy(page[unamepos:unamepos+uint32(len(stmt.row.username))], stmt.row.username[:])
	emailpos := idpos + uint32(unsafe.Offsetof(stmt.row.email))
	copy(page[emailpos:emailpos+uint32(len(stmt.row.email))], stmt.row.email[:])
	pg.length += uint16(tbl.rowSize)
	binary.LittleEndian.PutUint16(page[2:4], pg.length)
	pg.rows++
	binary.LittleEndian.PutUint16(page[4:6], pg.rows)
	tbl.rows++
	return 1, nil
}

func parseCell[R Row](page Page, offset uint32) Cell[R] {
	keyend := offset + 4
	unamepos := keyend + 4
	emailpos := unamepos + 32
	emailoff := emailpos + 255
	cell := Cell[R]{
		key: binary.LittleEndian.Uint32(page[offset:keyend]),
	}
	row := Row{
		id: binary.LittleEndian.Uint32(page[keyend:unamepos]),
	}
	copy(row.username[:], page[unamepos:emailpos])
	copy(row.email[:], page[emailpos:emailoff])
	cell.row = R(row)
	return cell
}

func (tbl *Table[R]) selectRow(_ *Statement) []Row {
	rows := []Row{}
	cursor, found := tbl.CursorStart()
	if !found {
		return rows
	}

	i := 0
	for {
		row, found := cursor.Row()
		if !found {
			log.Println("not found at iterate:", i)
			break
		}
		log.Println("cursor rownum:", cursor.rowNum)
		if !cursor.Next() {
			break
		}
		rows = append(rows, row)
		i++
	}
	return rows
}

func (tbl *Table[R]) Execute(stmt *Statement) {
	switch stmt.kind {
	case StatementInsert:
		// tbl.insertRow(stmt)
		cursor, existed := tbl.GetCursor(stmt.row.id)
		if existed {
			log.Printf("id %d is already existed\n", stmt.row.id)
			return
		}
		if err := cursor.SetRow(stmt.row); err != nil {
			log.Println("error insert row:", err)
		}
	case StatementSelect:
		for _, row := range tbl.selectRow(stmt) {
			nullUname := bytes.IndexByte(row.username[:], '\x00')
			nullEmail := bytes.IndexByte(row.email[:], '\x00')
			fmt.Printf("(%d, %s, %s)\n", row.id,
				row.username[:nullUname], row.email[:nullEmail])

		}
	}
}

func sizeOf[Uint constraints.Unsigned](nums ...Uint) Uint {
	var n Uint
	if len(nums) > 0 {
		n = nums[0]
	}
	return Uint(unsafe.Sizeof(n))
}

func newPage(pages *[][]byte) ([]byte, PageHeader) {
	page := make([]byte, pageSize)
	*pages = append(*pages, page)

	page[0] = uint8(bitfield.New(nkIsLeaf).Value())
	pho := pageHeaderOffsets()
	binary.LittleEndian.PutUint32(page[pho.parentStart:pho.parentEnd], 0)
	binary.LittleEndian.PutUint16(page[pho.parentEnd:pho.lengthEnd], uint16(pho.rowsEnd))
	binary.LittleEndian.PutUint16(page[pho.lengthEnd:pho.rowsEnd], 0)
	return page, PageHeader{page[0], 0, uint16(pho.rowsEnd), 0}
}

func readPageHeader(page []byte) PageHeader {
	pho := pageHeaderOffsets()
	return PageHeader{
		nodeType: page[0],
		parent:   binary.LittleEndian.Uint32(page[pho.parentStart:pho.parentEnd]),
		length:   binary.LittleEndian.Uint16(page[pho.parentEnd:pho.lengthEnd]),
		rows:     binary.LittleEndian.Uint16(page[pho.lengthEnd:pho.rowsEnd]),
	}
}

type (
	MetaCommand   byte
	StatementKind byte
	PrepareKind   byte
)

const pageSize = 4096

var (
	szu8  = sizeOf[uint8]()
	szu16 = sizeOf[uint16]()
	szu32 = sizeOf[uint32]()
)

const (
	MetaCommandSuccess MetaCommand = iota
	MetaCommandUnrecognized
)

const (
	StatementSelect StatementKind = iota
	StatementInsert
)

const (
	PrepareSuccess PrepareKind = iota
	PrepareUnknown
)

type (
	Statement struct {
		kind StatementKind
		row  Row
	}

	Cell[R any] struct {
		key uint32
		row R
	}

	Row struct {
		id       uint32
		username [32]byte
		email    [255]byte
	}

	PageHeader struct {
		nodeType     byte // bitfield value of Bitfield[NodeKind]
		parent       uint32
		length, rows uint16
	}

	PageHeaderOffset struct {
		parentStart, parentEnd, lengthEnd, rowsEnd int
	}

	Page         []byte
	Table[R any] struct {
		name    string
		file    *os.File
		rows    uint32
		rowSize uint32
		pages   [][]byte
	}

	Cursor[R any] struct {
		table      *Table[R]
		rowNum     uint32
		pageNum    uint32
		pageOffset uint16
		existed    bool
		endOfTable bool
	}
)

func (tbl *Table[R]) fetchDBFile() error {
	if tbl.file == nil {
		return nil
	}
	pagesz := pageSize
	fstat, err := tbl.file.Stat()
	if err != nil {
		return fmt.Errorf("fetchDBFile: %w", err)
	}
	tbl.file.Seek(0, io.SeekStart)
	rowsStart := uint16(szu8) + uint16(szu32) + szu16
	rowsEnd := rowsStart + szu16
	for i := 0; i*int(pagesz) < int(fstat.Size()); i++ {
		log.Println("fetch db page:", i+1)
		page := make([]byte, pagesz)
		_, err := tbl.file.Read(page)
		if err != nil {
			log.Printf("error reading page %d: %v\n", i, err)
			continue
		}
		rows := uint32(binary.LittleEndian.Uint16(page[rowsStart:rowsEnd]))
		log.Printf("page: %d, rows: %d", i, rows)
		tbl.rows += rows
		tbl.pages = append(tbl.pages, page)
	}
	return nil
}

func NewTable[R any](name string) (*Table[R], error) {
	var row R
	tbl := Table[R]{
		name:    name,
		rowSize: uint32(unsafe.Sizeof(row)),
	}
	var err error
	tbl.file, err = os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("cannot open db file: %w", err)
	}
	tbl.file.Seek(0, io.SeekStart)
	if err := tbl.fetchDBFile(); err != nil {
		return nil, fmt.Errorf("cannot read db file: %w", err)
	}
	tbl.GetPage()
	return &tbl, nil

}

func (t *Table[R]) GetPage() (Page, PageHeader) {
	var (
		page Page
		pg   PageHeader
	)
	if len(t.pages) < 1 {
		page, pg = newPage(&t.pages)
	} else {
		page = t.pages[len(t.pages)-1]
		pg = readPageHeader(page)
	}
	if pg.length+uint16(t.rowSize) >= pageSize {
		page, pg = newPage(&t.pages)
	}
	return page, pg
}

func (t *Table[R]) GetCursor(id uint32) (*Cursor[R], bool) {
	page, _ := t.GetPage()
	minIndex := uint32(0)
	maxIndex := (pageSize - uint32(pageHeaderSize)) / t.rowSize

	if id >= maxIndex {
		return nil, false
	}

	var cursor *Cursor[R]
	for minIndex != maxIndex {
		idx := (maxIndex + minIndex) / 2
		cursor, _ = t.CursorAt(idx)
		keyend := cursor.pageOffset + 4
		key := binary.LittleEndian.Uint32(page[cursor.pageOffset:keyend])
		if key == 0 {
			return cursor, false
		}
		if key == idx {
			return cursor, true
		}
		if key > idx {
			minIndex = idx
		} else {
			maxIndex = idx
		}
	}
	isCursorEmpty := true
	return cursor, isCursorEmpty
}

func (t *Table[R]) flushPages() error {
	t.file.Seek(0, io.SeekStart)
	for _, page := range t.pages {
		if _, err := t.file.Write(page); err != nil {
			return err
		}
	}
	return nil
}

func (t *Table[R]) Close() error {
	if t.file == nil {
		return nil
	}
	return t.file.Close()
}

func (t *Table[R]) CursorAt(rownum uint32) (*Cursor[R], bool) {
	atEnd := false
	cursor := &Cursor[R]{
		table:      t,
		rowNum:     rownum,
		endOfTable: atEnd,
	}
	pgsz := uint32(pageHeaderSize)
	rowsPerPage := (pageSize - pgsz) / t.rowSize
	var pagePos uint32
	if atEnd {
		pagePos = uint32(len(t.pages)) - 1
	} else {
		pagePos = (rownum / rowsPerPage)
	}
	if pagePos >= uint32(len(t.pages)) {
		return cursor, false
	}
	cursor.pageNum = pagePos
	cursor.pageOffset = uint16(pgsz + (rownum%rowsPerPage)*t.rowSize)
	return cursor, true

}

func (t *Table[R]) CursorStart() (*Cursor[R], bool) {
	return t.CursorAt(0)
}

func (t *Table[R]) CursorEnd() (*Cursor[R], bool) {
	cursor, found := t.CursorAt(t.rows)
	cursor.endOfTable = true
	return cursor, found
}

func (c *Cursor[R]) Row() (Row, bool) {
	if c.rowNum > c.table.rows {
		return Row{}, false
	}
	cell := parseCell(c.table.pages[c.pageNum], uint32(c.pageOffset))
	return cell.row, true
}

func (c *Cursor[R]) Next() bool {
	if c.rowNum >= c.table.rows || c.endOfTable {
		return false
	}
	cc, _ := c.table.CursorAt(c.rowNum + 1)
	c.endOfTable = cc.endOfTable
	c.pageNum = cc.pageNum
	c.pageOffset = cc.pageOffset
	c.rowNum++
	return true
}

func (c *Cursor[R]) SetRow(row Row) error {
	nullUname := bytes.IndexByte(row.username[:], '\x00')
	nullEmail := bytes.IndexByte(row.email[:], '\x00')
	fmt.Printf("insert exec row id: %d, username: %s, email: %s\n", row.id,
		row.username[:nullUname], row.email[:nullEmail])
	thetable[row.id] = row
	page, pg := c.table.GetPage()
	log.Printf("cursor then: %#v\n", c)
	if c.pageOffset >= pageSize {
		cc, _ := c.table.CursorEnd()
		c.endOfTable = cc.endOfTable
		c.pageNum = cc.pageNum
		c.pageOffset = cc.pageOffset
	}
	log.Printf("cursor now: %#v\n", c)
	keypos := c.pageOffset + 4
	unamepos := keypos + 4
	emailpos := unamepos + 32
	emailoff := emailpos + 255
	binary.LittleEndian.PutUint32(page[c.pageOffset:keypos], row.id)
	binary.LittleEndian.PutUint32(page[keypos:unamepos],
		row.id)
	copy(page[unamepos:emailpos], row.username[:])
	copy(page[emailpos:emailoff], row.email[:])
	c.table.rows++
	pg.length += uint16(c.table.rowSize)
	pho := pageHeaderOffsets()
	binary.LittleEndian.PutUint16(page[pho.parentEnd:pho.lengthEnd], pg.length)
	pg.rows++
	binary.LittleEndian.PutUint16(page[pho.lengthEnd:pho.rowsEnd], pg.rows)
	return nil
}

var (
	statementMap = map[StatementKind]string{
		StatementSelect: "SELECT",
		StatementInsert: "INSERT",
	}
	thetable       = map[uint32]Row{}
	pageHeaderSize = uint32(szu8) + szu32 + 2*uint32(szu16)
)

type NodeKind byte

const (
	nkIsRoot NodeKind = iota
	nkIsLeaf
)

func (nk NodeKind) Enums() []NodeKind {
	return []NodeKind{nkIsRoot, nkIsLeaf}
}

var nodeKinds = []string{"nkIsRoot", "nkIsLeaf"}

func (nk NodeKind) String() string {
	return nodeKinds[nk]
}

type Node struct {
	kind   NodeKind
	isRoot bool
	parent *Node
}

func pageHeaderOffsets() PageHeaderOffset {
	pho := PageHeaderOffset{}
	pho.parentStart = 1
	pho.parentEnd = pho.parentStart + int(szu32)
	pho.lengthEnd = pho.parentEnd + int(szu16)
	pho.rowsEnd = pho.lengthEnd + int(szu16)
	return pho
}
