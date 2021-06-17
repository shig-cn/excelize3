// Copyright 2016 - 2020 The excelize Authors. All rights reserved. Use of
// this source code is governed by a BSD-style license that can be found in
// the LICENSE file.
//
// Package excelize providing a set of functions that allow you to write to
// and read from XLSX files. Support reads and writes XLSX file generated by
// Microsoft Excel™ 2007 and later. Support save file without losing original
// charts of XLSX. This library needs Go version 1.10 or later.

package excelize

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// StreamWriter defined the type of stream writer.
type StreamWriter struct {
	File       *File
	Sheet      string
	SheetID    int
	worksheet  *xlsxWorksheet
	rawData    bufferedWriter
	tableParts string
}

// NewStreamWriter return stream writer struct by given worksheet name for
// generate new worksheet with large amounts of data. Note that after set
// rows, you must call the 'Flush' method to end the streaming writing
// process and ensure that the order of line numbers is ascending. For
// example, set data for worksheet of size 102400 rows x 50 columns with
// numbers and style:
//
//    file := excelize.NewFile()
//    streamWriter, err := file.NewStreamWriter("Sheet1")
//    if err != nil {
//        fmt.Println(err)
//    }
//    styleID, err := file.NewStyle(`{"font":{"color":"#777777"}}`)
//    if err != nil {
//        fmt.Println(err)
//    }
//    if err := streamWriter.SetRow("A1", []interface{}{excelize.Cell{StyleID: styleID, Value: "Data"}}); err != nil {
//        fmt.Println(err)
//    }
//    for rowID := 2; rowID <= 102400; rowID++ {
//        row := make([]interface{}, 50)
//        for colID := 0; colID < 50; colID++ {
//            row[colID] = rand.Intn(640000)
//        }
//        cell, _ := excelize.CoordinatesToCellName(1, rowID)
//        if err := streamWriter.SetRow(cell, row); err != nil {
//            fmt.Println(err)
//        }
//    }
//    if err := streamWriter.Flush(); err != nil {
//        fmt.Println(err)
//    }
//    if err := file.SaveAs("Book1.xlsx"); err != nil {
//        fmt.Println(err)
//    }
//
func (f *File) NewStreamWriter(sheet string) (*StreamWriter, error) {
	sheetID := f.GetSheetIndex(sheet)
	if sheetID == 0 {
		return nil, fmt.Errorf("sheet %s is not exist", sheet)
	}
	sw := &StreamWriter{
		File:    f,
		Sheet:   sheet,
		SheetID: sheetID,
	}
	var err error
	sw.worksheet, err = f.workSheetReader(sheet)
	if err != nil {
		return nil, err
	}
	sw.rawData.WriteString(XMLHeader + `<worksheet` + templateNamespaceIDMap)
	bulkAppendFields(&sw.rawData, sw.worksheet, 1, 5)
	sw.rawData.WriteString(`<sheetData>`)
	return sw, err
}

// AddTable creates an Excel table for the StreamWriter using the given
// coordinate area and format set. For example, create a table of A1:D5:
//
//    err := sw.AddTable("A1", "D5", ``)
//
// Create a table of F2:H6 with format set:
//
//    err := sw.AddTable("F2", "H6", `{"table_name":"table","table_style":"TableStyleMedium2","show_first_column":true,"show_last_column":true,"show_row_stripes":false,"show_column_stripes":true}`)
//
// Note that the table must be at least two lines including the header. The
// header cells must contain strings and must be unique.
//
// Currently only one table is allowed for a StreamWriter. AddTable must be
// called after the rows are written but before Flush.
//
// See File.AddTable for details on the table format.
func (sw *StreamWriter) AddTable(hcell, vcell, format string) error {
	formatSet, err := parseFormatTableSet(format)
	if err != nil {
		return err
	}

	coordinates, err := areaRangeToCoordinates(hcell, vcell)
	if err != nil {
		return err
	}
	_ = sortCoordinates(coordinates)

	// Correct the minimum number of rows, the table at least two lines.
	if coordinates[1] == coordinates[3] {
		coordinates[3]++
	}

	// Correct table reference coordinate area, such correct C1:B3 to B1:C3.
	ref, err := sw.File.coordinatesToAreaRef(coordinates)
	if err != nil {
		return err
	}

	// create table columns using the first row
	tableHeaders, err := sw.getRowValues(coordinates[1], coordinates[0], coordinates[2])
	if err != nil {
		return err
	}
	tableColumn := make([]*xlsxTableColumn, len(tableHeaders))
	for i, name := range tableHeaders {
		tableColumn[i] = &xlsxTableColumn{
			ID:   i + 1,
			Name: name,
		}
	}

	tableID := sw.File.countTables() + 1

	name := formatSet.TableName
	if name == "" {
		name = "Table" + strconv.Itoa(tableID)
	}

	table := xlsxTable{
		XMLNS:       NameSpaceSpreadSheet,
		ID:          tableID,
		Name:        name,
		DisplayName: name,
		Ref:         ref,
		AutoFilter: &xlsxAutoFilter{
			Ref: ref,
		},
		TableColumns: &xlsxTableColumns{
			Count:       len(tableColumn),
			TableColumn: tableColumn,
		},
		TableStyleInfo: &xlsxTableStyleInfo{
			Name:              formatSet.TableStyle,
			ShowFirstColumn:   formatSet.ShowFirstColumn,
			ShowLastColumn:    formatSet.ShowLastColumn,
			ShowRowStripes:    formatSet.ShowRowStripes,
			ShowColumnStripes: formatSet.ShowColumnStripes,
		},
	}

	sheetRelationshipsTableXML := "../tables/table" + strconv.Itoa(tableID) + ".xml"
	tableXML := strings.Replace(sheetRelationshipsTableXML, "..", "xl", -1)

	// Add first table for given sheet.
	sheetPath, _ := sw.File.sheetMap[trimSheetName(sw.Sheet)]
	sheetRels := "xl/worksheets/_rels/" + strings.TrimPrefix(sheetPath, "xl/worksheets/") + ".rels"
	rID := sw.File.addRels(sheetRels, SourceRelationshipTable, sheetRelationshipsTableXML, "")

	sw.tableParts = fmt.Sprintf(`<tableParts count="1"><tablePart r:id="rId%d"></tablePart></tableParts>`, rID)

	sw.File.addContentTypePart(tableID, "table")

	b, _ := xml.Marshal(table)
	sw.File.saveFileList(tableXML, b)
	return nil
}

// Extract values from a row in the StreamWriter.
func (sw *StreamWriter) getRowValues(hrow, hcol, vcol int) (res []string, err error) {
	res = make([]string, vcol-hcol+1)

	r, err := sw.rawData.Reader()
	if err != nil {
		return nil, err
	}

	dec := sw.File.xmlNewDecoder(r)
	for {
		token, err := dec.Token()
		if err == io.EOF {
			return res, nil
		}
		if err != nil {
			return nil, err
		}
		startElement, ok := getRowElement(token, hrow)
		if !ok {
			continue
		}
		// decode cells
		var row xlsxRow
		if err := dec.DecodeElement(&row, &startElement); err != nil {
			return nil, err
		}
		for _, c := range row.C {
			col, _, err := CellNameToCoordinates(c.R)
			if err != nil {
				return nil, err
			}
			if col < hcol || col > vcol {
				continue
			}
			res[col-hcol] = c.V
		}
		return res, nil
	}
}

// Check if the token is an XLSX row with the matching row number.
func getRowElement(token xml.Token, hrow int) (startElement xml.StartElement, ok bool) {
	startElement, ok = token.(xml.StartElement)
	if !ok {
		return
	}
	ok = startElement.Name.Local == "row"
	if !ok {
		return
	}
	ok = false
	for _, attr := range startElement.Attr {
		if attr.Name.Local != "r" {
			continue
		}
		row, _ := strconv.Atoi(attr.Value)
		if row == hrow {
			ok = true
			return
		}
	}
	return
}

// Cell can be used directly in StreamWriter.SetRow to specify a style and
// a value.
type Cell struct {
	StyleID int
	Value   interface{}
}

// SetRow writes an array to stream rows by giving a worksheet name, starting
// coordinate and a pointer to an array of values. Note that you must call the
// 'Flush' method to end the streaming writing process.
//
// As a special case, if Cell is used as a value, then the Cell.StyleID will be
// applied to that cell.
func (sw *StreamWriter) SetRow(axis string, values []interface{}) error {
	col, row, err := CellNameToCoordinates(axis)
	if err != nil {
		return err
	}

	fmt.Fprintf(&sw.rawData, `<row r="%d">`, row)
	for i, val := range values {
		axis, err := CoordinatesToCellName(col+i, row)
		if err != nil {
			return err
		}
		c := xlsxC{R: axis}
		if v, ok := val.(Cell); ok {
			c.S = v.StyleID
			val = v.Value
		} else if v, ok := val.(*Cell); ok && v != nil {
			c.S = v.StyleID
			val = v.Value
		}
		if err = setCellValFunc(&c, val); err != nil {
			sw.rawData.WriteString(`</row>`)
			return err
		}
		writeCell(&sw.rawData, c)
	}
	sw.rawData.WriteString(`</row>`)
	return sw.rawData.Sync()
}

// setCellValFunc provides a function to set value of a cell.
func setCellValFunc(c *xlsxC, val interface{}) (err error) {
	switch val := val.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		err = setCellIntFunc(c, val)
	case float32:
		c.T, c.V = setCellFloat(float64(val), -1, 32)
	case float64:
		c.T, c.V = setCellFloat(val, -1, 64)
	case string:
		c.T, c.V, c.XMLSpace = setCellStr(val)
	case []byte:
		c.T, c.V, c.XMLSpace = setCellStr(string(val))
	case time.Duration:
		c.T, c.V = setCellDuration(val)
	case time.Time:
		c.T, c.V, _, err = setCellTime(val)
	case bool:
		c.T, c.V = setCellBool(val)
	case nil:
		c.T, c.V, c.XMLSpace = setCellStr("")
	default:
		c.T, c.V, c.XMLSpace = setCellStr(fmt.Sprint(val))
	}
	return err
}

// setCellIntFunc is a wrapper of SetCellInt.
func setCellIntFunc(c *xlsxC, val interface{}) (err error) {
	switch val := val.(type) {
	case int:
		c.T, c.V = setCellInt(val)
	case int8:
		c.T, c.V = setCellInt(int(val))
	case int16:
		c.T, c.V = setCellInt(int(val))
	case int32:
		c.T, c.V = setCellInt(int(val))
	case int64:
		c.T, c.V = setCellInt(int(val))
	case uint:
		c.T, c.V = setCellInt(int(val))
	case uint8:
		c.T, c.V = setCellInt(int(val))
	case uint16:
		c.T, c.V = setCellInt(int(val))
	case uint32:
		c.T, c.V = setCellInt(int(val))
	case uint64:
		c.T, c.V = setCellInt(int(val))
	default:
	}
	return
}

func writeCell(buf *bufferedWriter, c xlsxC) {
	buf.WriteString(`<c`)
	if c.XMLSpace.Value != "" {
		fmt.Fprintf(buf, ` xml:%s="%s"`, c.XMLSpace.Name.Local, c.XMLSpace.Value)
	}
	fmt.Fprintf(buf, ` r="%s"`, c.R)
	if c.S != 0 {
		fmt.Fprintf(buf, ` s="%d"`, c.S)
	}
	if c.T != "" {
		fmt.Fprintf(buf, ` t="%s"`, c.T)
	}
	buf.WriteString(`>`)
	if c.V != "" {
		buf.WriteString(`<v>`)
		xml.EscapeText(buf, []byte(c.V))
		buf.WriteString(`</v>`)
	}
	buf.WriteString(`</c>`)
}

// Flush ending the streaming writing process.
func (sw *StreamWriter) Flush() error {
	sw.rawData.WriteString(`</sheetData>`)
	bulkAppendFields(&sw.rawData, sw.worksheet, 7, 37)
	sw.rawData.WriteString(sw.tableParts)
	bulkAppendFields(&sw.rawData, sw.worksheet, 39, 39)
	sw.rawData.WriteString(`</worksheet>`)
	if err := sw.rawData.Flush(); err != nil {
		return err
	}

	sheetXML := fmt.Sprintf("xl/worksheets/sheet%d.xml", sw.SheetID)
	delete(sw.File.Sheet, sheetXML)
	delete(sw.File.checked, sheetXML)

	defer sw.rawData.Close()
	b, err := sw.rawData.Bytes()
	if err != nil {
		return err
	}
	sw.File.XLSX[sheetXML] = b
	return nil
}

// bulkAppendFields bulk-appends fields in a worksheet by specified field
// names order range.
func bulkAppendFields(w io.Writer, ws *xlsxWorksheet, from, to int) {
	s := reflect.ValueOf(ws).Elem()
	enc := xml.NewEncoder(w)
	for i := 0; i < s.NumField(); i++ {
		if from <= i && i <= to {
			enc.Encode(s.Field(i).Interface())
		}
	}
}

// bufferedWriter uses a temp file to store an extended buffer. Writes are
// always made to an in-memory buffer, which will always succeed. The buffer
// is written to the temp file with Sync, which may return an error.
// Therefore, Sync should be periodically called and the error checked.
type bufferedWriter struct {
	tmp *os.File
	buf bytes.Buffer
}

// Write to the in-memory buffer. The err is always nil.
func (bw *bufferedWriter) Write(p []byte) (n int, err error) {
	return bw.buf.Write(p)
}

// WriteString wites to the in-memory buffer. The err is always nil.
func (bw *bufferedWriter) WriteString(p string) (n int, err error) {
	return bw.buf.WriteString(p)
}

// Reader provides read-access to the underlying buffer/file.
func (bw *bufferedWriter) Reader() (io.Reader, error) {
	if bw.tmp == nil {
		return bytes.NewReader(bw.buf.Bytes()), nil
	}
	if err := bw.Flush(); err != nil {
		return nil, err
	}
	fi, err := bw.tmp.Stat()
	if err != nil {
		return nil, err
	}
	// os.File.ReadAt does not affect the cursor position and is safe to use here
	return io.NewSectionReader(bw.tmp, 0, fi.Size()), nil
}

// Bytes returns the entire content of the bufferedWriter. If a temp file is
// used, Bytes will efficiently allocate a buffer to prevent re-allocations.
func (bw *bufferedWriter) Bytes() ([]byte, error) {
	if bw.tmp == nil {
		return bw.buf.Bytes(), nil
	}

	if err := bw.Flush(); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if fi, err := bw.tmp.Stat(); err == nil {
		if size := fi.Size() + bytes.MinRead; size > bytes.MinRead {
			if int64(int(size)) == size {
				buf.Grow(int(size))
			} else {
				return nil, bytes.ErrTooLarge
			}
		}
	}

	if _, err := bw.tmp.Seek(0, 0); err != nil {
		return nil, err
	}

	_, err := buf.ReadFrom(bw.tmp)
	return buf.Bytes(), err
}

// Sync will write the in-memory buffer to a temp file, if the in-memory
// buffer has grown large enough. Any error will be returned.
func (bw *bufferedWriter) Sync() (err error) {
	// Try to use local storage
	const chunk = 1 << 24
	if bw.buf.Len() < chunk {
		return nil
	}
	if bw.tmp == nil {
		bw.tmp, err = ioutil.TempFile(os.TempDir(), "excelize-")
		if err != nil {
			// can not use local storage
			return nil
		}
	}
	return bw.Flush()
}

// Flush the entire in-memory buffer to the temp file, if a temp file is being
// used.
func (bw *bufferedWriter) Flush() error {
	if bw.tmp == nil {
		return nil
	}
	_, err := bw.buf.WriteTo(bw.tmp)
	if err != nil {
		return err
	}
	bw.buf.Reset()
	return nil
}

// Close the underlying temp file and reset the in-memory buffer.
func (bw *bufferedWriter) Close() error {
	bw.buf.Reset()
	if bw.tmp == nil {
		return nil
	}
	defer os.Remove(bw.tmp.Name())
	return bw.tmp.Close()
}
