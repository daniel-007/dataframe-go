package dataframe

import (
	"errors"
	"sync"
)

type DataFrame struct {
	lock   sync.RWMutex
	Series []Series
	n      int // Number of rows
}

// NewDataFrame creates a new dataframe.
func NewDataFrame(se ...Series) *DataFrame {

	df := &DataFrame{
		Series: []Series{},
	}

	if len(se) > 0 {
		var count *int
		names := map[string]struct{}{}

		for _, s := range se {
			if count == nil {
				count = &[]int{s.NRows()}[0]
				names[s.Name()] = struct{}{}
			} else {
				if *count != s.NRows() {
					panic("different number of rows in series")
				}
				if _, exists := names[s.Name()]; exists {
					panic("names of series must be unique")
				}
				names[s.Name()] = struct{}{}
			}
			df.Series = append(df.Series, s)
		}
		df.n = *count
	}

	return df
}

func (df *DataFrame) NRows() int {
	df.lock.RLock()
	defer df.lock.RUnlock()

	return df.n
}

// ValuesOptions is used to modify the behaviour of Values()
type ValuesOptions struct {
	// InitialRow represents the starting value for iterating
	InitialRow int

	// Step represents by how much each iteration should step by.
	// It can be negative to represent iterating in backwards direction.
	// InitialRow should be adjusted to NRows()-1 if Step is negative.
	// If Step is 0, the function will panic.
	Step int

	// Don't apply lock. This is useful if you intend to Write lock
	// the entire dataframe
	DontLock bool
}

// Values will return an iterator that can be used to iterate through all the values
func (df *DataFrame) Values(options ...ValuesOptions) func() (*int, map[interface{}]interface{}) {

	var row int
	var step int = 1

	var dontlock bool

	if len(options) > 0 {
		dontlock = options[0].DontLock
	}

	if len(options) > 0 {
		row = options[0].InitialRow
		step = options[0].Step
		if step == 0 {
			panic("Step can not be zero")
		}
	}

	return func() (*int, map[interface{}]interface{}) {
		if !dontlock {
			df.lock.RLock()
			defer df.lock.RUnlock()
		}

		if row > df.n-1 || row < 0 {
			// Don't iterate further
			return nil, nil
		}

		out := map[interface{}]interface{}{}

		for idx, aSeries := range df.Series {
			val := aSeries.Value(row)
			out[aSeries.Name()] = val
			out[idx] = val
		}

		row = row + step

		return &[]int{row - step}[0], out
	}
}

// Prepend inserts a row at the beginning.
func (df *DataFrame) Prepend(vals ...interface{}) {
	df.lock.Lock()
	defer df.lock.Unlock()

	if len(vals) > 0 {

		switch v := vals[0].(type) {
		case map[string]interface{}:

			names := map[string]struct{}{}
			for name, _ := range v {
				names[name] = struct{}{}
			}

			// Check if number of vals is equal to number of series
			if len(names) != len(df.Series) {
				panic("no. of args not equal to no. of series")
			}

			for name, val := range v {
				col, err := df.NameToColumn(name)
				if err != nil {
					panic(err)
				}
				df.Series[col].Prepend(val)
			}
		default:
			// Check if number of vals is equal to number of series
			if len(vals) != len(df.Series) {
				panic("no. of args not equal to no. of series")
			}

			for idx, val := range vals {
				df.Series[idx].Prepend(val)
			}
		}

		df.n++
	}
}

// Append inserts a row at the end.
func (df *DataFrame) Append(vals ...interface{}) {
	df.Insert(df.n, vals...)
}

// Insert adds a row to a particular position.
func (df *DataFrame) Insert(row int, vals ...interface{}) {
	df.lock.Lock()
	defer df.lock.Unlock()

	df.insert(row, vals...)
}

func (df *DataFrame) insert(row int, vals ...interface{}) {

	if len(vals) > 0 {

		switch v := vals[0].(type) {
		case map[string]interface{}:

			names := map[string]struct{}{}
			for name, _ := range v {
				names[name] = struct{}{}
			}

			// Check if number of vals is equal to number of series
			if len(names) != len(df.Series) {
				panic("no. of args not equal to no. of series")
			}

			for name, val := range v {
				col, err := df.NameToColumn(name)
				if err != nil {
					panic(err)
				}
				df.Series[col].Insert(row, val)
			}
		default:
			// Check if number of vals is equal to number of series
			if len(vals) != len(df.Series) {
				panic("no. of args not equal to no. of series")
			}

			for idx, val := range vals {
				df.Series[idx].Insert(row, val)
			}
		}

		df.n++
	}
}

// Remove deletes a row.
func (df *DataFrame) Remove(row int) {
	df.lock.Lock()
	defer df.lock.Unlock()

	for i := range df.Series {
		df.Series[i].Remove(row)
	}
	df.n--
}

// Update is used to update a specific entry.
// col can the name of the series or the column number
func (df *DataFrame) Update(row int, col interface{}, val interface{}) {
	df.lock.Lock()
	defer df.lock.Unlock()

	switch name := col.(type) {
	case string:
		_col, err := df.NameToColumn(name)
		if err != nil {
			panic(err)
		}
		col = _col
	}

	df.Series[col.(int)].Update(row, val)
}

// UpdateRow will update an entire row
func (df *DataFrame) UpdateRow(row int, vals ...interface{}) {
	df.lock.Lock()
	defer df.lock.Unlock()

	if len(vals) > 0 {

		switch v := vals[0].(type) {
		case map[string]interface{}:
			for name, val := range v {
				col, err := df.NameToColumn(name)
				if err != nil {
					panic(err)
				}
				df.Series[col].Update(row, val)
			}
		default:
			// Check if number of vals is equal to number of series
			if len(vals) != len(df.Series) {
				panic("no. of args not equal to no. of series")
			}

			for idx, val := range vals {
				df.Series[idx].Update(row, val)
			}
		}
	}
}

// Names will return a list of all the series names.
func (df *DataFrame) Names() []string {
	names := []string{}

	for _, aSeries := range df.Series {
		names = append(names, aSeries.Name())
	}

	return names
}

// NameToColumn returns the index of the series based on the name.
// The starting index is 0.
func (df *DataFrame) NameToColumn(seriesName string) (int, error) {
	for idx, aSeries := range df.Series {
		if aSeries.Name() == seriesName {
			return idx, nil
		}
	}

	return 0, errors.New("no series contains name")
}

// Swap is used to swap 2 values based on their row position.
func (df *DataFrame) Swap(row1, row2 int) {
	df.lock.Lock()
	defer df.lock.Unlock()

	df.swap(row1, row2)
}

func (df *DataFrame) swap(row1, row2 int) {
	for idx := range df.Series {
		df.Series[idx].Swap(row1, row2)
	}
}

// Lock will lock the dataframe allowing you to directly manipulate
// the underlying series with confidence.
func (df *DataFrame) Lock() {
	df.lock.Lock()
}

// Unlock will unlock the dataframe that was previously locked.
func (df *DataFrame) Unlock() {
	df.lock.Unlock()
}
