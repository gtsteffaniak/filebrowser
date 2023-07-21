package q

import (
	"go/constant"
	"go/token"
	"reflect"
	"strconv"
)

func compare(a, b interface{}, tok token.Token) bool {
	vala := reflect.ValueOf(a)
	valb := reflect.ValueOf(b)

	ak := vala.Kind()
	bk := valb.Kind()
	switch {
	// comparing nil values
	case (ak == reflect.Ptr || ak == reflect.Slice || ak == reflect.Interface || ak == reflect.Invalid) &&
		(bk == reflect.Ptr || ak == reflect.Slice || bk == reflect.Interface || bk == reflect.Invalid) &&
		(!vala.IsValid() || vala.IsNil()) && (!valb.IsValid() || valb.IsNil()):
		return true
	case ak >= reflect.Int && ak <= reflect.Int64:
		if bk >= reflect.Int && bk <= reflect.Int64 {
			return constant.Compare(constant.MakeInt64(vala.Int()), tok, constant.MakeInt64(valb.Int()))
		}

		if bk >= reflect.Uint && bk <= reflect.Uint64 {
			return constant.Compare(constant.MakeInt64(vala.Int()), tok, constant.MakeInt64(int64(valb.Uint())))
		}

		if bk == reflect.Float32 || bk == reflect.Float64 {
			return constant.Compare(constant.MakeFloat64(float64(vala.Int())), tok, constant.MakeFloat64(valb.Float()))
		}

		if bk == reflect.String {
			bla, err := strconv.ParseFloat(valb.String(), 64)
			if err != nil {
				return false
			}

			return constant.Compare(constant.MakeFloat64(float64(vala.Int())), tok, constant.MakeFloat64(bla))
		}
	case ak >= reflect.Uint && ak <= reflect.Uint64:
		if bk >= reflect.Uint && bk <= reflect.Uint64 {
			return constant.Compare(constant.MakeUint64(vala.Uint()), tok, constant.MakeUint64(valb.Uint()))
		}

		if bk >= reflect.Int && bk <= reflect.Int64 {
			return constant.Compare(constant.MakeUint64(vala.Uint()), tok, constant.MakeUint64(uint64(valb.Int())))
		}

		if bk == reflect.Float32 || bk == reflect.Float64 {
			return constant.Compare(constant.MakeFloat64(float64(vala.Uint())), tok, constant.MakeFloat64(valb.Float()))
		}

		if bk == reflect.String {
			bla, err := strconv.ParseFloat(valb.String(), 64)
			if err != nil {
				return false
			}

			return constant.Compare(constant.MakeFloat64(float64(vala.Uint())), tok, constant.MakeFloat64(bla))
		}
	case ak == reflect.Float32 || ak == reflect.Float64:
		if bk == reflect.Float32 || bk == reflect.Float64 {
			return constant.Compare(constant.MakeFloat64(vala.Float()), tok, constant.MakeFloat64(valb.Float()))
		}

		if bk >= reflect.Int && bk <= reflect.Int64 {
			return constant.Compare(constant.MakeFloat64(vala.Float()), tok, constant.MakeFloat64(float64(valb.Int())))
		}

		if bk >= reflect.Uint && bk <= reflect.Uint64 {
			return constant.Compare(constant.MakeFloat64(vala.Float()), tok, constant.MakeFloat64(float64(valb.Uint())))
		}

		if bk == reflect.String {
			bla, err := strconv.ParseFloat(valb.String(), 64)
			if err != nil {
				return false
			}

			return constant.Compare(constant.MakeFloat64(vala.Float()), tok, constant.MakeFloat64(bla))
		}
	case ak == reflect.String:
		if bk == reflect.String {
			return constant.Compare(constant.MakeString(vala.String()), tok, constant.MakeString(valb.String()))
		}
	}

	typea, typeb := reflect.TypeOf(a), reflect.TypeOf(b)

	if typea != nil && (typea.String() == "time.Time" || typea.String() == "*time.Time") &&
		typeb != nil && (typeb.String() == "time.Time" || typeb.String() == "*time.Time") {

		if typea.String() == "*time.Time" && vala.IsNil() {
			return true
		}

		if typeb.String() == "*time.Time" {
			if valb.IsNil() {
				return true
			}
			valb = valb.Elem()
		}

		var x, y int64
		x = 1
		if vala.MethodByName("Equal").Call([]reflect.Value{valb})[0].Bool() {
			y = 1
		} else if vala.MethodByName("Before").Call([]reflect.Value{valb})[0].Bool() {
			y = 2
		}
		return constant.Compare(constant.MakeInt64(x), tok, constant.MakeInt64(y))
	}

	if tok == token.EQL {
		return reflect.DeepEqual(a, b)
	}

	return false
}
