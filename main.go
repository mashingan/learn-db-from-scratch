package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"strings"
	"unsafe"
)

func main() {
	scn := bufio.NewScanner(os.Stdin)
	isString := false
	buffer := ""
	prompt := "> "
	var stmt Statement
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
				execute(&stmt)
			}
			clear(stmt.row.username[:])
			clear(stmt.row.email[:])
			clearPrompt()
			continue
		}
		if buffer == "" && strings.HasPrefix(line, ".") {
			_, cont := handleMetaCommand(line)
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

func handleMetaCommand(cmd string) (MetaCommand, bool) {
	rcmd := MetaCommandUnrecognized
	cont := true
	switch cmd {
	case ".quit", ".q":
		rcmd = MetaCommandSuccess
		cont = false
	case ".pages", ".p":
		rcmd = MetaCommandSuccess
		for _, page := range pages {
			lp := len(page)
			fmt.Println("total pages length:", lp)
			rows := binary.LittleEndian.Uint16(page[4:6])
			for i := 0; i < int(rows); i++ {
				row := i
				idpos := 6 + (i * int(rowSize))
				unamepos := idpos + 4
				emailpos := unamepos + 32
				log.Println("idpos:", idpos)
				log.Println("unamepos:", unamepos)
				log.Println("emailpos:", emailpos)
				if unamepos >= lp || emailpos >= lp || emailpos+255 >= lp {
					log.Println("any endpoint more than pages. ignore!")
					continue
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

		buname := unsafe.StringData(uname)
		stmt.row.username = *(*[32]byte)(unsafe.Pointer(&unsafe.Slice(buname, 32)[0]))
		stmt.row.email = *(*[255]byte)(unsafe.Pointer(unsafe.StringData(email)))
		// copy(stmt.row.username[:], []byte(uname))
		// copy(stmt.row.email[:], []byte(email))
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

func executeInsert(stmt *Statement, pages *[][]byte) (int, error) {
	nullUname := bytes.IndexByte(stmt.row.username[:], '\x00')
	nullEmail := bytes.IndexByte(stmt.row.email[:], '\x00')
	fmt.Printf("insert exec row id: %d, username: %s, email: %s\n", stmt.row.id,
		stmt.row.username[:nullUname], stmt.row.email[:nullEmail])
	thetable[stmt.row.id] = stmt.row
	page := []byte{}
	var pg PageHeader
	if len(*pages) < 1 {
		page, pg = newPage(pages)
	} else {
		page = (*pages)[len(*pages)-1]
		pg = readPageHeader(page)
	}
	if pg.length+uint16(rowSize) >= pg.size {
		page, pg = newPage(pages)
	}
	id := make([]byte, 4)
	binary.LittleEndian.PutUint32(id, stmt.row.id)
	idpos := 6 + uint32(pg.rows*uint16(rowSize))
	idoff := unsafe.Sizeof(stmt.row.id)
	binary.LittleEndian.PutUint32(page[idpos:idpos+uint32(idoff)], stmt.row.id)
	unamepos := idpos + uint32(unsafe.Offsetof(stmt.row.username))
	copy(page[unamepos:unamepos+uint32(len(stmt.row.username))], stmt.row.username[:])
	emailpos := idpos + uint32(unsafe.Offsetof(stmt.row.email))
	copy(page[emailpos:emailpos+uint32(len(stmt.row.email))], stmt.row.email[:])
	pg.length += uint16(rowSize)
	binary.LittleEndian.PutUint16(page[2:4], pg.length)
	pg.rows++
	binary.LittleEndian.PutUint16(page[4:6], pg.rows)
	return 1, nil
}

func executeSelect(_ *Statement, pages [][]byte) []Row {
	rows := []Row{}
	for _, page := range pages {
		pg := readPageHeader(page)
		for i := 0; i < int(pg.rows); i++ {
			idpos := 6 + int(rowSize)*i
			unamepos := idpos + 4
			emailpos := unamepos + 32
			emailsz := 255
			row := Row{
				id: binary.LittleEndian.Uint32(page[idpos:unamepos]),
			}
			copy(row.username[:], page[unamepos:emailpos])
			copy(row.email[:], page[emailpos:emailpos+emailsz])
			rows = append(rows, row)
		}
	}
	return rows
}

func execute(stmt *Statement) {
	switch stmt.kind {
	case StatementInsert:
		executeInsert(stmt, &pages)
	case StatementSelect:
		for _, row := range executeSelect(stmt, pages) {
			nullUname := bytes.IndexByte(row.username[:], '\x00')
			nullEmail := bytes.IndexByte(row.email[:], '\x00')
			fmt.Printf("(%d, %s, %s)\n", row.id,
				row.username[:nullUname], row.email[:nullEmail])
		}
	}
}

func newPage(pages *[][]byte) ([]byte, PageHeader) {
	page := make([]byte, pageSize)
	*pages = append(*pages, page)
	// size, length, rows
	var u16 uint16
	sz := uint16(unsafe.Sizeof(u16))
	binary.LittleEndian.PutUint16(page[:sz], pageSize)
	binary.LittleEndian.PutUint16(page[sz:sz+2], sz*3)
	binary.LittleEndian.PutUint16(page[sz+2:sz+4], 0)
	return page, PageHeader{pageSize, sz * 3, 0}
}

func readPageHeader(page []byte) PageHeader {
	var u16 uint16
	sz := uint16(unsafe.Sizeof(u16))
	return PageHeader{
		size:   binary.LittleEndian.Uint16(page[:sz]),
		length: binary.LittleEndian.Uint16(page[sz : sz*2]),
		rows:   binary.LittleEndian.Uint16(page[sz*2 : sz*3]),
	}
}

type (
	MetaCommand   byte
	StatementKind byte
	PrepareKind   byte
)

const pageSize = 4096

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

	Row struct {
		id       uint32
		username [32]byte
		email    [255]byte
	}

	PageHeader struct {
		size, length, rows uint16
	}
)

var (
	statementMap = map[StatementKind]string{
		StatementSelect: "SELECT",
		StatementInsert: "INSERT",
	}
	thetable = map[uint32]Row{}
	pages    = [][]byte{}
	rowSize  = unsafe.Sizeof(Row{})
)
