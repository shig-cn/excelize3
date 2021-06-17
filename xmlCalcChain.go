// Copyright 2016 - 2020 The excelize Authors. All rights reserved. Use of
// this source code is governed by a BSD-style license that can be found in
// the LICENSE file.
//
// Package excelize providing a set of functions that allow you to write to
// and read from XLSX files. Support reads and writes XLSX file generated by
// Microsoft Excel™ 2007 and later. Support save file without losing original
// charts of XLSX. This library needs Go version 1.10 or later.

package excelize

import "encoding/xml"

// xlsxCalcChain directly maps the calcChain element. This element represents the root of the calculation chain.
type xlsxCalcChain struct {
	XMLName xml.Name         `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main calcChain"`
	C       []xlsxCalcChainC `xml:"c"`
}

// xlsxCalcChainC directly maps the c element.
//
//     Attributes               | Attributes
//    --------------------------+----------------------------------------------------------
//     a (Array)                | A Boolean flag indicating whether the cell's formula
//                              | is an array formula. True if this cell's formula is
//                              | an array formula, false otherwise. If there is a
//                              | conflict between this attribute and the t attribute
//                              | of the f element (§18.3.1.40), the t attribute takes
//                              | precedence. The possible values for this attribute
//                              | are defined by the W3C XML Schema boolean datatype.
//                              |
//     i (Sheet Id)             | A sheet Id of a sheet the cell belongs to. If this is
//                              | omitted, it is assumed to be the same as the i value
//                              | of the previous cell.The possible values for this
//                              | attribute are defined by the W3C XML Schema int datatype.
//                              |
//     l (New Dependency Level) | A Boolean flag indicating that the cell's formula
//                              | starts a new dependency level. True if the formula
//                              | starts a new dependency level, false otherwise.
//                              | Starting a new dependency level means that all
//                              | concurrent calculations, and child calculations, shall
//                              | be completed - and the cells have new values - before
//                              | the calc chain can continue. In other words, this
//                              | dependency level might depend on levels that came before
//                              | it, and any later dependency levels might depend on
//                              | this level; but not later dependency levels can have
//                              | any calculations started until this dependency level
//                              | completes.The possible values for this attribute are
//                              | defined by the W3C XML Schema boolean datatype.
//                              |
//     r (Cell Reference)       | An A-1 style reference to a cell.The possible values
//                              | for this attribute are defined by the ST_CellRef
//                              | simple type (§18.18.7).
//                              |
//     s (Child Chain)          | A Boolean flag indicating whether the cell's formula
//                              | is on a child chain. True if this cell is part of a
//                              | child chain, false otherwise. If this is omitted, it
//                              | is assumed to be the same as the s value of the
//                              | previous cell .A child chain is a list of calculations
//                              | that occur which depend on the parent to the chain.
//                              | There shall not be cross dependencies between child
//                              | chains. Child chains are not the same as dependency
//                              | levels - a child chain and its parent are all on the
//                              | same dependency level. Child chains are series of
//                              | calculations that can be independently farmed out to
//                              | other threads or processors.The possible values for
//                              | this attribute are defined by the W3C XML Schema
//                              | boolean datatype.
//                              |
//     t (New Thread)           | A Boolean flag indicating whether the cell's formula
//                              | starts a new thread. True if the cell's formula starts
//                              | a new thread, false otherwise.The possible values for
//                              | this attribute are defined by the W3C XML Schema
//                              | boolean datatype.
//
type xlsxCalcChainC struct {
	R string `xml:"r,attr"`
	I int    `xml:"i,attr"`
	L bool   `xml:"l,attr,omitempty"`
	S bool   `xml:"s,attr,omitempty"`
	T bool   `xml:"t,attr,omitempty"`
	A bool   `xml:"a,attr,omitempty"`
}
