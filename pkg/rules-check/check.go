package rulescheck

import "github.com/Omarmeks89/notr/pkg/audit"

func detectInvalidAuditUsage() {
	var text = "test-error"
	audit.ErrorMessage(text)
}

func functionAlias() {
	alias := functionAlias
	alias()
}

func recursionCall(n int) int {
	if n == 0 {
		return n
	}

	return recursionCall(n - 1)
}

func recursionCall2(n int) int {
	if n == 0 {
		return n
	}

	al := recursionCall2

	return al(al(n-1) - al(n-2))
}

func recursionCall3(n int) int {
	var fnc func(i int) int

	if n == 0 {
		return n
	}

	a, b := 0, 1
	fnc, a = recursionCall3, b
	_ = a

	return fnc(fnc(n-1) - fnc(n-2))
}

func recursionCall4(n int) int {
	if n == 0 {
		return n
	}

	al := recursionCall4

	return al(n-1) - al(n-2)
}

func recursionCall5(n int) int {
	var fnc func(i int) int

	if n == 0 {
		return n
	}

	a, b, c := 0, 1, 0
	b, fnc, a = c, recursionCall5, b
	_ = a

	return fnc(fnc(n-1) - fnc(n-2))
}

func recursionCall6(n int) int {
	if n == 0 {
		return n
	}

	return recursionCall6(n-1) + recursionCall6(n-2)
}

func recursionCall7(n int) int {
	var fnc func(i int) int

	if n == 0 {
		return n
	}

	a, b, c, e := 0, 1, 0, 12
	b, fnc, a, e = c, recursionCall7, b, c
	_ = a
	_ = e

	return fnc(fnc(n-1) - fnc(n-2))
}

func recursionCall8(n int) int {
	var fnc func(i int) int

	if n == 0 {
		return n
	}

	a, b, c, e := 0, 1, 0, 12
	b, fnc, a, e = c, recursionCall8, b, c
	_ = a
	_ = e

	return fnc(fnc(n-1) - recursionCall8(n-2))
}

type testStruct struct{}

func (ts *testStruct) methodAlias() {
	alias := ts.methodAlias
	alias()
}

func (ts *testStruct) methodCall(n int) int {
	if n == 0 {
		return n
	}

	return ts.methodCall(n - 1)
}

func (ts *testStruct) methodCall2(n int) int {
	if n == 0 {
		return n
	}

	al := ts.methodCall2

	return al(al(n-1) - al(n-2))
}

func (ts *testStruct) methodCall3(n int) int {
	var fnc func(i int) int

	if n == 0 {
		return n
	}

	a, b := 0, 1
	fnc, a = ts.methodCall3, b
	_ = a

	return fnc(fnc(n-1) - fnc(n-2))
}

func (ts *testStruct) methodCall4(n int) int {
	if n == 0 {
		return n
	}

	al := ts.methodCall4

	return al(n-1) - al(n-2)
}

func (ts *testStruct) methodCall5(n int) int {
	var fnc func(i int) int

	if n == 0 {
		return n
	}

	a, b, c := 0, 1, 0
	b, fnc, a = c, ts.methodCall5, b
	_ = a

	return fnc(fnc(n-1) - fnc(n-2))
}

func (ts *testStruct) methodCall6(n int) int {
	if n == 0 {
		return n
	}

	return ts.methodCall6(n-1) + ts.methodCall6(n-2)
}

func (ts *testStruct) methodCall7(n int) int {
	var fnc func(i int) int

	if n == 0 {
		return n
	}

	a, b, c, e := 0, 1, 0, 12
	b, fnc, a, e = c, ts.methodCall7, b, c
	_ = a
	_ = e

	return fnc(fnc(n-1) - fnc(n-2))
}

func (ts *testStruct) methodCall8(n int) int {
	var fnc func(i int) int

	if n == 0 {
		return n
	}

	a, b, c, e := 0, 1, 0, 12
	b, fnc, a, e = c, ts.methodCall8, b, c
	_ = a
	_ = e

	return fnc(fnc(n-1) - ts.methodCall8(n-2))
}

func (ts *testStruct) methodCallMiddleRecursion(n int) int {
	var fnc func(i int) int

	if n == 0 {
		return n
	}

	a, b, c := 0, 1, 0
	b, fnc, a = c, ts.methodCallMiddleRecursion, b

	a = fnc(fnc(n-1) - fnc(n-2))

	return a
}

func (ts *testStruct) methodCallMiddleRecursion2(n int) int {
	if n == 0 {
		return n
	}

	s := ts.methodCallMiddleRecursion2(n-1) + ts.methodCallMiddleRecursion2(n-2)

	return s
}

func recursionMiddle(n int) int {
	var fnc func(i int) int

	if n == 0 {
		return n
	}

	a, b, c := 0, 1, 0
	b, fnc, a = c, recursionMiddle, b

	a = fnc(fnc(n-1) - fnc(n-2))

	return a
}

func recursionMiddle2(n int) int {
	if n == 0 {
		return n
	}

	s := recursionMiddle2(n-1) + recursionMiddle2(n-2)

	return s
}

func recursionMiddle3(n int) int {
	if n == 0 {
		return n
	}

	// tree recursion as a function parameter
	a := recursionCall(recursionMiddle3(n-1) - recursionMiddle3(n-2))

	return a
}

// simple cases
func _r0(i int) int {
	if i == 0 {
		return i
	}

	i = _r0(i)

	return i
}

func _r1(i int) int {
	if i == 0 {
		return i
	}

	J := _r1

	return J(i - 1)
}
