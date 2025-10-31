package main

func fib(n int) int {
	if n == 0 || n == 1 {
		return n
	}

	fn := fib
	a := fn

	return fn(n-1) + a(n-2) // want "tree recursion in call 'fib'"
}

func complexFunc(i, j int) int {
	if i <= 0 || j <= 0 {
		return i - j
	}

	k, n := i, j
	cf := complexFunc

	return cf(cf(i-1, j), cf(k, n-1)) // want "tree recursion in call 'complexFunc'"
}

type T struct{}

func (t T) A(i int) int {
	a := t.A
	b := a
	_ = b
	return b(2) + a(2+1) // want "tree recursion in call 't.A'"
}

func (t T) B(i int) int {
	return t.B(2) + t.B(2+1) // want "tree recursion in call 't.B'"
}

func A(i int) int {
	return i + 1
}
