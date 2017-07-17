// Copyright 2017 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/sessionctx/variable"
)

// Range is the interface of the three type of range.
type Range interface {
	fmt.Stringer
	Convert2IntRange() IntColumnRange
	Convert2ColumnRange() *ColumnRange
	Convert2IndexRange() *IndexRange
}

// IntColumnRange represents a range for a integer column, both low and high are inclusive.
type IntColumnRange struct {
	LowVal  int64
	HighVal int64
}

// IsPoint returns if the table range is a point.
func (tr *IntColumnRange) IsPoint() bool {
	return tr.HighVal == tr.LowVal
}

func (tr IntColumnRange) String() string {
	var l, r string
	if tr.LowVal == math.MinInt64 {
		l = "(-inf"
	} else {
		l = "[" + strconv.FormatInt(tr.LowVal, 10)
	}
	if tr.HighVal == math.MaxInt64 {
		r = "+inf)"
	} else if tr.HighVal == math.MinInt64 {
		// This branch is for nil
		r = "-inf)"
	} else {
		r = strconv.FormatInt(tr.HighVal, 10) + "]"
	}
	return l + "," + r
}

// Convert2IntRange implements the Convert2IntRange interface.
func (tr IntColumnRange) Convert2IntRange() IntColumnRange {
	return tr
}

// Convert2ColumnRange implements the Convert2ColumnRange interface.
func (tr IntColumnRange) Convert2ColumnRange() *ColumnRange {
	panic("you shouldn't call this method.")
}

// Convert2IndexRange implements the Convert2IndexRange interface.
func (tr IntColumnRange) Convert2IndexRange() *IndexRange {
	panic("you shouldn't call this method.")
}

// ColumnRange represents a range for a column.
type ColumnRange struct {
	Low      Datum
	High     Datum
	LowExcl  bool
	HighExcl bool
}

func (cr *ColumnRange) String() string {
	var l, r string
	if cr.LowExcl {
		l = "("
	} else {
		l = "["
	}
	if cr.HighExcl {
		r = ")"
	} else {
		r = "]"
	}
	return l + formatDatum(cr.Low) + "," + formatDatum(cr.High) + r
}

// Convert2IntRange implements the Convert2IntRange interface.
func (cr *ColumnRange) Convert2IntRange() IntColumnRange {
	panic("you shouldn't call this method.")
}

// Convert2ColumnRange implements the Convert2ColumnRange interface.
func (cr *ColumnRange) Convert2ColumnRange() *ColumnRange {
	return cr
}

// Convert2IndexRange implements the Convert2IndexRange interface.
func (cr *ColumnRange) Convert2IndexRange() *IndexRange {
	panic("you shouldn't call this method.")
}

// IndexRange represents a range for an index.
type IndexRange struct {
	LowVal  []Datum
	HighVal []Datum

	LowExclude  bool // Low value is exclusive.
	HighExclude bool // High value is exclusive.
}

// IsPoint returns if the index range is a point.
func (ir *IndexRange) IsPoint(sc *variable.StatementContext) bool {
	if len(ir.LowVal) != len(ir.HighVal) {
		return false
	}
	for i := range ir.LowVal {
		a := ir.LowVal[i]
		b := ir.HighVal[i]
		if a.Kind() == KindMinNotNull || b.Kind() == KindMaxValue {
			return false
		}
		cmp, err := a.CompareDatum(sc, b)
		if err != nil {
			return false
		}
		if cmp != 0 {
			return false
		}
	}
	return !ir.LowExclude && !ir.HighExclude
}

// Convert2IndexRange implements the Convert2IndexRange interface.
func (ir *IndexRange) String() string {
	lowStrs := make([]string, 0, len(ir.LowVal))
	for _, d := range ir.LowVal {
		lowStrs = append(lowStrs, formatDatum(d))
	}
	highStrs := make([]string, 0, len(ir.LowVal))
	for _, d := range ir.HighVal {
		highStrs = append(highStrs, formatDatum(d))
	}
	l, r := "[", "]"
	if ir.LowExclude {
		l = "("
	}
	if ir.HighExclude {
		r = ")"
	}
	return l + strings.Join(lowStrs, " ") + "," + strings.Join(highStrs, " ") + r
}

// Convert2IntRange implements the Convert2IntRange interface.
func (ir *IndexRange) Convert2IntRange() IntColumnRange {
	panic("you shouldn't call this method.")
}

// Convert2ColumnRange implements the Convert2ColumnRange interface.
func (ir *IndexRange) Convert2ColumnRange() *ColumnRange {
	panic("you shouldn't call this method.")
}

// Convert2IndexRange implements the Convert2IndexRange interface.
func (ir *IndexRange) Convert2IndexRange() *IndexRange {
	return ir
}

// Align appends low value and high value up to the number of columns with max value, min not null value or null value.
func (ir *IndexRange) Align(numColumns int) {
	for i := len(ir.LowVal); i < numColumns; i++ {
		if ir.LowExclude {
			ir.LowVal = append(ir.LowVal, MaxValueDatum())
		} else {
			ir.LowVal = append(ir.LowVal, Datum{})
		}
	}
	for i := len(ir.HighVal); i < numColumns; i++ {
		if ir.HighExclude {
			ir.HighVal = append(ir.HighVal, Datum{})
		} else {
			ir.HighVal = append(ir.HighVal, MaxValueDatum())
		}
	}
}

// PrefixEqualLen tells you how long the prefix of the range is a point.
// e.g. If this range is (1 2 3, 1 2 +inf), then the return value is 2.
func (ir *IndexRange) PrefixEqualLen(sc *variable.StatementContext) (int, error) {
	// Here, len(ir.LowVal) always equal to len(ir.HighVal)
	for i := 0; i < len(ir.LowVal); i++ {
		cmp, err := ir.LowVal[i].CompareDatum(sc, ir.HighVal[i])
		if err != nil {
			return 0, errors.Trace(err)
		}
		if cmp != 0 {
			return i, nil
		}
	}
	return len(ir.LowVal), nil
}

func formatDatum(d Datum) string {
	if d.Kind() == KindMinNotNull {
		return "-inf"
	}
	if d.Kind() == KindMaxValue {
		return "+inf"
	}
	return fmt.Sprintf("%v", d.GetValue())
}

// ReverseRange reverses range to the opposite order, e.g.
// (a, b] => [b, a)
// (-inf, b] => [b, +inf)
func (ir *IndexRange) ReverseRange() *IndexRange {
	for i := 0; i < len(ir.LowVal); i++ {
		if ir.LowVal[i].Kind() == KindMinNotNull {
			// (-inf, a) => (a, +inf)
			// (-inf, a] => [a, +inf)
			ir.LowVal[i], ir.HighVal[i] = ir.HighVal[i], ir.LowVal[i]
			ir.HighVal[i].SetKind(KindMaxValue)
		} else if (ir.LowVal[i].Kind() != KindNull &&
			ir.LowVal[i].Kind() != KindMaxValue) &&
			ir.HighVal[i].Kind() == KindMaxValue {
			// (a, +inf) => (-inf, a)
			// [a, +inf) => (-inf, a]
			ir.LowVal[i], ir.HighVal[i] = ir.HighVal[i], ir.LowVal[i]
			ir.LowVal[i].SetKind(KindMinNotNull)
		} else if ir.LowVal[i].Kind() == KindNull &&
			ir.HighVal[i].Kind() == KindMaxValue {
			// (a, b) => (b, a), or (-inf, +inf) => (-inf, +inf)
			// nothing to do
		} else if ir.LowVal[i].Kind() == KindMaxValue &&
			ir.HighVal[i].Kind() == KindMaxValue {
			// (a, b] => [b, a)
			ir.LowVal[i].SetKind(KindNull)
			ir.HighVal[i].SetKind(KindNull)
		} else if ir.LowVal[i].Kind() == KindNull &&
			ir.HighVal[i].Kind() == KindNull {
			// [a, b) => (b, a]
			ir.LowVal[i].SetKind(KindMaxValue)
			ir.HighVal[i].SetKind(KindMaxValue)
		} else {
			// [a, b] => [b, a]
			// and all the other values need swap also
			ir.LowVal[i], ir.HighVal[i] = ir.HighVal[i], ir.LowVal[i]
		}
	}
	// swap exclude flag
	ir.LowExclude, ir.HighExclude = ir.HighExclude, ir.LowExclude
	return ir
}
