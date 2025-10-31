# notr
golang linter detect tree recursion cases like:

```go
func fib(n int) int {
	if n == 0 || n == 1 {
		return n
	}

	return fib(n-1) + fib(n-2) // tree recursion here
}
```

or like this:

```go
func strangeFunc(i, j int) int {
	// some recursion base
    // ...

	return cf(cf(i-1, j), cf(k, n-1)) // tree recursion here
}
```

```go
func (t T) B(i int) int {
    // some recursion base
    // ...

	return t.B(t.B(i - 1)) // tree recursion here
}
```

### Output sample
```
pkg/notr/testdata/src/test_func.go:11:9: tree recursion in call 'fib'
6               }
7
8               fn := fib
9               a := fn
10
11              return fn(n-1) + a(n-2) // want "tree recursion in call 'fib'"
12      }
13
14      func complexFunc(i, j int) int {
15              if i <= 0 || j <= 0 {
16                      return i - j
pkg/notr/testdata/src/test_func.go:22:9: tree recursion in call 'complexFunc'
17              }
18
19              k, n := i, j
20              cf := complexFunc
21
22              return cf(cf(i-1, j), cf(k, n-1)) // want "tree recursion in call 'complexFunc'"
23      }
24
25      type T struct{}
26
27      func (t T) A(i int) int {
pkg/notr/testdata/src/test_func.go:31:9: tree recursion in call 't.A'
26
27      func (t T) A(i int) int {
28              a := t.A
29              b := a
30              _ = b
31              return b(2) + a(2+1) // want "tree recursion in call 't.A'"
32      }
33
34      func (t T) B(i int) int {
35              return t.B(2) + t.B(2+1) // want "tree recursion in call 't.B'"
36      }
pkg/notr/testdata/src/test_func.go:35:9: tree recursion in call 't.B'
30              _ = b
31              return b(2) + a(2+1) // want "tree recursion in call 't.A'"
32      }
33
34      func (t T) B(i int) int {
35              return t.B(2) + t.B(2+1) // want "tree recursion in call 't.B'"
36      }
37
38      func A(i int) int {
39              return i + 1
40      }
```

## Setup project

### Build
```bash
make init
make build
```

### Use with `vet`
```bash
make install
make vet
```

Vet will detect all cases