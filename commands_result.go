package orient

import (
	"fmt"
	"reflect"
)

var (
	_ Results = errorResult{}
	_ Results = (*unknownResult)(nil)
)

// Results is an interface for database command results. Must be closed.
//
// Individual results can be iterated in a next way:
//
//		results := db.Command(cmd)
//		if err := results.Err(); err != nil {
//			// handle command errors; can be omitted and checked later with Err or Close
//		}
//		var one SomeStruct
//		for results.Next(&one) {
//			// process result, if any
//		}
//		if err := results.Close(); err != nil {
//			// handle command and/or type conversion errors
//		}
//
// Or just retrieved all at once:
//
//		var arr []SomeStruct
//		if err := results.All(&arr); err != nil {
//			// handle command and/or type conversion errors
//		}
//
// Some commands may return just one int/bool value:
//
//		var affected int
//		results.All(&affected)
//
// Also results can be handled manually:
//
//		var out interface{}
//		results.All(&out)
//		switch out.(type) {
//		case []OIdentifiable:
//			// ...
//		case *DocumentRecord:
//			// ...
//		}
//
type Results interface {
	Err() error
	Close() error
	Next(result interface{}) bool
	All(result interface{}) error
}

// errorResult is a simple result type that returns one specific error. Useful for server-side errors.
type errorResult struct {
	err error
}

func (e errorResult) Err() error                   { return e.err }
func (e errorResult) Close() error                 { return e.err }
func (e errorResult) Next(result interface{}) bool { return false }
func (e errorResult) All(result interface{}) error { return e.err }

func newResults(o interface{}) Results {
	return &unknownResult{result: o}
}

// unknownResult is a generic result type that uses reflection to iterate over returned records
type unknownResult struct {
	err    error
	parsed bool
	result interface{}
}

func (r *unknownResult) Err() error                     { return r.err }
func (r *unknownResult) Close() error                   { return r.err }
func (r *unknownResult) Next(result interface{}) bool { // TODO: implement
	if r.parsed {
		return false
	}
	r.parsed = true
	r.All(result)
	return false
}
func (r *unknownResult) All(result interface{}) error {
	//	if r.parsed {
	//		return fmt.Errorf("results are already parsed")
	//	}
	//	r.parsed = true

	// check for pointer
	targ := reflect.ValueOf(result)
	if targ.Kind() != reflect.Ptr {
		return fmt.Errorf("result is not a pointer: %T", result)
	} else if targ.IsNil() {
		return fmt.Errorf("nil result pointer")
	}
	targ = targ.Elem()

	return convertTypes(targ, reflect.ValueOf(r.result))
}

type ErrUnsupportedConversion struct {
	From reflect.Value
	To   reflect.Value
}

func (e ErrUnsupportedConversion) Error() string {
	var a, b string
	if e.From.IsValid() {
		a = fmt.Sprintf("%v(%v)", e.From.Type(), e.From.Kind())
	} else {
		a = "<nil>"
	}
	if e.To.IsValid() {
		b = fmt.Sprintf("%v(%v)", e.To.Type(), e.To.Kind())
	} else {
		b = "<nil>"
	}
	return fmt.Sprintf("unsupported conversion: %v -> %v", a, b)
}

func mapToStruct(m interface{}, val interface{}) error {
	dec, err := newMapDecoder(val)
	if err != nil {
		return err
	}
	return dec.Decode(m)
}

const debugTypeConversion = false

func convertTypes(targ, src reflect.Value) error {
	if debugTypeConversion {
		fmt.Printf("conv: %T -> %T, %+v -> %+v\n", src.Interface(), targ.Interface(), src.Interface(), targ.Interface())
		defer func() {
			fmt.Printf("conv out: %T -> %T, %+v -> %+v\n", src.Interface(), targ.Interface(), src.Interface(), targ.Interface())
		}()
	}
	if targ.Type() == src.Type() {
		targ.Set(src)
		return nil
	} else if src.Type().ConvertibleTo(targ.Type()) {
		targ.Set(src.Convert(targ.Type()))
		return nil
	} else if src.Kind() == reflect.Interface {
		return convertTypes(targ, src.Elem())
	}
	//	if targ.Kind() == reflect.Ptr {
	//		if targ.IsNil() {
	//			targ.Set(reflect.New(targ.Type().Elem()))
	//		}
	//		targ = targ.Elem()
	//	}
	//	if src.Kind() == reflect.Ptr {
	//		src = src.Elem()
	//	}

	if targ.Kind() == reflect.Struct || (targ.Kind() == reflect.Ptr && targ.Type().Elem().Kind() == reflect.Struct) {
		if targ.Kind() == reflect.Ptr && targ.IsNil() {
			targ.Set(reflect.New(targ.Type().Elem()))
		}
		if src.Kind() == reflect.Map {
			return mapToStruct(src.Interface(), targ.Addr().Interface())
		}
	} else if targ.Kind() == reflect.Slice {
		if src.Kind() == reflect.Slice { // slice into slice
			if targ.Len() != src.Len() {
				targ.Set(reflect.MakeSlice(targ.Type(), src.Len(), src.Len()))
			}
			for i := 0; i < src.Len(); i++ {
				if err := convertTypes(targ.Index(i), src.Index(i)); err != nil {
					return err
				}
			}
			return nil
		}
		// one value into slice
		targ.Set(reflect.MakeSlice(targ.Type(), 1, 1))
		if err := convertTypes(targ.Index(0), src); err != nil {
			targ.Set(reflect.Zero(targ.Type()))
			return err
		}
		return nil
	} else if targ.Kind() == reflect.Map {
		if src.Kind() == reflect.Map {
			targ.Set(reflect.MakeMap(targ.Type()))
			for _, k := range src.MapKeys() {
				nk := reflect.New(targ.Type().Key()).Elem()
				if err := convertTypes(nk, k); err != nil {
					return err
				}
				nv := reflect.New(targ.Type().Elem()).Elem()
				if err := convertTypes(nv, src.MapIndex(k)); err != nil {
					return err
				}
				targ.SetMapIndex(nk, nv)
			}
			return nil
		}
	}

	switch rec := src.Interface().(type) {
	case MapSerializable:
		m, err := rec.ToMap()
		if err != nil {
			return err
		}
		return convertTypes(targ, reflect.ValueOf(m))
	case *Document: // Document implements DocumentSerializable for convenience, no need to convert it
	case DocumentSerializable:
		doc, err := rec.ToDocument()
		if err != nil {
			return err
		}
		return convertTypes(targ, reflect.ValueOf(doc))
	}

	// Target is now converted, process the result set
	if src.Kind() == reflect.Slice {
		switch src.Len() {
		case 0:
			return ErrNoRecord
		case 1:
			return convertTypes(targ, src.Index(0))
		default:
			return ErrMultipleRecords{N: src.Len(), Err: ErrUnsupportedConversion{From: src, To: targ}}
		}
	}

	return ErrUnsupportedConversion{From: src, To: targ}
}
