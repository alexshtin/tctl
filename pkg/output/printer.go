// The MIT License
//
// Copyright (c) 2020 Temporal Technologies Inc.  All rights reserved.
//
// Copyright (c) 2020 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package output

import (
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/temporalio/tctl/pkg/format"
	"github.com/temporalio/tctl/pkg/pager"
	"github.com/urfave/cli/v2"
	"go.temporal.io/server/common/collection"
)

const (
	BatchPrintSize = 100 // for consistent formatting, print items in batches (ex. in Table output)
)

type PrintOptions struct {
	Fields      []string
	FieldsLong  []string
	IgnoreFlags bool
	Output      OutputOption
	Pager       io.Writer
	NoPager     bool
	NoHeader    bool
	Separator   string
}

func PrintItems(c *cli.Context, items []interface{}, opts *PrintOptions) {
	outputFlag := c.String(FlagOutput)
	fields := c.String(FlagFields)

	if opts.Pager == nil {
		pager, close := newPagerWithDefault(c)
		opts.Pager = pager
		defer close()
	}

	if !opts.IgnoreFlags && c.IsSet(FlagFields) {
		if fields == FieldsLong {
			opts.Fields = append(opts.Fields, opts.FieldsLong...)
			opts.FieldsLong = []string{}
		} else {
			f := strings.Split(fields, ",")
			for i := range f {
				f[i] = strings.TrimSpace(f[i])
			}
			opts.Fields = f
			opts.FieldsLong = []string{}
		}
	}

	output := Table
	if !opts.IgnoreFlags && c.IsSet(FlagOutput) {
		output = OutputOption(outputFlag)
	} else if opts.Output != "" {
		output = opts.Output
	}

	switch output {
	case Table:
		PrintTable(c, items, opts)
	case JSON:
		PrintJSON(c, items, opts)
	case Card:
		PrintCards(c, items, opts)
	default:
	}
}

// Pager creates an interactive CLI mode to control the printing of items
func Pager(c *cli.Context, iter collection.Iterator, opts *PrintOptions) error {
	limit := c.Int(FlagLimit)

	pager, close := newPagerWithDefault(c)
	defer close()

	if opts == nil {
		opts = &PrintOptions{}
	}
	opts.Pager = pager

	itemsPrinted := 0
	var batch []interface{}
	for iter.HasNext() {
		item, err := iter.Next()
		if err != nil {
			return err
		}

		if c.IsSet(FlagLimit) && itemsPrinted >= limit {
			break
		}

		batch = append(batch, item)
		itemsPrinted++

		isLastBatch := limit-itemsPrinted < BatchPrintSize
		isBatchFilled := (len(batch) == BatchPrintSize) || (isLastBatch && len(batch) == limit%BatchPrintSize)

		if isBatchFilled || !iter.HasNext() {
			PrintItems(c, batch, opts)
			batch = batch[:0]
			opts.NoHeader = true
		}
	}

	return nil
}

func newPagerWithDefault(c *cli.Context) (io.Writer, func()) {
	outputFlag := c.String(FlagOutput)
	output := OutputOption(outputFlag)

	var defaultPager string
	if output == Table {
		defaultPager = string(pager.Less)
	} else {
		defaultPager = string(pager.More)
	}
	return pager.NewPager(c, defaultPager)
}

func formatField(c *cli.Context, i interface{}) string {
	val := reflect.ValueOf(i)
	val = reflect.Indirect(val)

	var typ reflect.Type
	if val.IsValid() && !val.IsZero() {
		typ = val.Type()
	}
	kin := val.Kind()

	if typ == reflect.TypeOf(time.Time{}) {
		return format.FormatTime(c, val.Interface().(time.Time))
	} else if kin == reflect.Struct && val.CanInterface() {
		str, _ := ParseToJSON(c, i, false)

		return str
	} else {
		return fmt.Sprintf("%v", i)
	}
}
