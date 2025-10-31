package notr

import (
	"container/list"
	"fmt"
	"go/ast"
	"go/token"
	"math"
	"reflect"
	"slices"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	linterName = "notr"

	defaultScopesCount  = 64
	defaultAliasesCount = 8

	fnCallInArgs opTyp = iota
	fnCallInBinOp
)

var (
	ErrTooManyOperations = errors.New("too many operations")
	ErrNoIdent           = errors.New("selector.X is not an *ast.Ident node")
)

// funcScope представление области видимости функции для
// работы с псевдонимами.
type (
	funcScope struct {
		name    string
		aliases map[string]map[string]struct{}
	}

	callContext struct {
		opNo             int    // operation number like operation scope
		scope            int    // function scope
		opType           opTyp  // operation type - call in args or call in binOp
		boundedFuncAlias string // funcname or func alias
		start            int    // expression start pos
		end              int    // expression end position
	}

	opTyp uint8

	// report - create type with methods.
	//
	// We can exclude from report (in this case): count, opNo, opType
	report struct {
		opType             opTyp // to handle reports by operation type.
		start, count, opNo int
		funcName           string
	}
)

// linter is a linter type
type linter struct {
	calls  *list.List         // deque of steps
	scopes map[int]*funcScope // function scope to localize calls
	opNo   int
}

func NewLinter() *linter {
	return &linter{
		calls:  list.New(),
		scopes: make(map[int]*funcScope, defaultScopesCount),
		opNo:   0,
	}
}

// NewAnalyzer создает новый анализатор (нужно прокинуть конфиг).
func NewAnalyzer() *analysis.Analyzer {
	a := NewLinter()
	return &analysis.Analyzer{
		Name:             linterName,
		Doc:              "linter that detect tree recursion calls for functions and methods",
		Run:              a.run,
		ResultType:       reflect.TypeOf(true),
		RunDespiteErrors: true,
		Requires:         []*analysis.Analyzer{inspect.Analyzer},
	}
}

func (lr *linter) run(p *analysis.Pass) (any, error) {
	wishedNodes := []ast.Node{&ast.FuncDecl{}}
	astCursor := p.ResultOf[inspect.Analyzer].(*inspector.Inspector).Root()

	// inspect AST using Cursor
	astCursor.Inspect(wishedNodes, lr.inspectAst)

	reports, err := lr.analyzeCalls()
	if err != nil {
		return false, errors.Wrap(err, "analyze calls")
	}

	slices.SortStableFunc(reports, func(a report, b report) int {
		return a.opNo - b.opNo
	})

	lr.report(p, reports)

	return true, nil
}

func (lr *linter) report(p *analysis.Pass, reports []report) {
	for _, report := range reports {
		msg := fmt.Sprintf("tree recursion in call '%s'", report.funcName)

		p.Report(analysis.Diagnostic{
			Pos:     token.Pos(report.start),
			Message: msg,
		})
	}
}

func (lr *linter) inspectAst(c inspector.Cursor) bool {
	switch nodeType := c.Node().(type) {
	case *ast.FuncDecl:
		scopeNo := lr.opNo

		if err := lr.handleFuncDeclaration(nodeType, scopeNo); err != nil {
			panic(err)
		}

		// handle assignment expression
		for cursor := range c.Preorder(&ast.AssignStmt{}) {
			if err := lr.handleAssignmentExpression(cursor, scopeNo); err != nil {
				if !errors.Is(err, ErrNoIdent) {
					panic(err)
				}
			}
		}

		// try to fetch like a(a(1)) - a(a(a(-2)))
		for cursor := range c.Preorder(&ast.BinaryExpr{}) {
			if err := lr.preorderNestedCall(cursor, scopeNo, fnCallInBinOp); err != nil {
				panic(err)
			}
		}

		for cursor := range c.Preorder(&ast.CallExpr{}) {
			if err := lr.preorderNestedCall(cursor, scopeNo, fnCallInArgs); err != nil {
				panic(err)
			}
		}

		if err := lr.incOp(); err != nil {
			panic(err)
		}
	}

	return false
}

func (lr *linter) handleFuncDeclaration(decl *ast.FuncDecl, scope int) error {
	if decl.Recv == nil {
		lr.registerFunc(scope, decl.Name.Name)

		return nil
	}

	for _, field := range decl.Recv.List {
		if len(field.Names) == 0 {
			return errors.Errorf("no field names found in node.Recv.List: %+v", decl.Recv.List)
		}

		methodName := field.Names[0].Name + "." + decl.Name.Name
		lr.registerFunc(scope, methodName)
	}

	return nil
}

func (lr *linter) handleAssignmentExpression(c inspector.Cursor, scope int) error {
	if err := lr.incOp(); err != nil {
		errors.Wrap(err, "too many operations")
	}

	assignment, _ := c.Node().(*ast.AssignStmt)
	if len(assignment.Lhs) > len(assignment.Rhs) {

		// means expr like: a, b := x[i]
		//
		// we can`t handle this case correctly because we have no possibility to check value inside
		// a slice on a static analysis stage.
		return nil
	}

	for i, lhsExpr := range assignment.Lhs {
		rhsExpr := assignment.Rhs[i]

		// we have to be sure about LHS and RHS
		identLeft, okIdentLeft := lhsExpr.(*ast.Ident)
		identRight, okIdentRight := rhsExpr.(*ast.Ident)

		if okIdentLeft && okIdentRight {
			// we have both Idents so we have to register them and continue.
			lr.registerAlias(scope, identRight.Name, identLeft.Name)

			continue
		}

		// check selector on RHS
		selectorRhs, okSelector := rhsExpr.(*ast.SelectorExpr)
		if !(okSelector && okIdentLeft) {
			continue
		}

		methodName, err := lr.getFullSelectorName(selectorRhs)
		if err != nil {
			return err
		}

		lr.registerAlias(scope, methodName, identLeft.Name)
	}

	return nil
}

func (lr *linter) getFullSelectorName(s *ast.SelectorExpr) (string, error) {
	receiverIdent, ok := s.X.(*ast.Ident)
	if !ok {
		return "", errors.Wrapf(ErrNoIdent, "skip unsupported selector 'X' type: %T", s.X)
	}

	return receiverIdent.Name + "." + s.Sel.Name, nil
}

func (lr *linter) preorderNestedCall(c inspector.Cursor, scope int, opType opTyp) error {
	if err := lr.incOp(); err != nil {
		return errors.Wrap(err, "too many operations")
	}

	for call := range c.Preorder(&ast.CallExpr{}) {
		callExpr, _ := call.Node().(*ast.CallExpr)
		lr.registerCall(scope, opType, callExpr)
	}

	return nil
}

func (lr *linter) incOp() error {
	if lr.opNo == math.MaxInt64 {
		return errors.Errorf("operations limit exeed: %d", math.MaxInt64)
	}

	lr.opNo++

	return nil
}

func (lr *linter) registerFunc(scope int, fName string) {
	if _, ok := lr.scopes[scope]; ok {
		return
	}

	lr.scopes[scope] = &funcScope{
		name:    fName,
		aliases: make(map[string]map[string]struct{}, defaultAliasesCount),
	}
}

// registerAlias сохраняет имя символа слева и имя символа справа, рассматривая их
// как потенциальные имена и псевдонимы функций.
func (lr *linter) registerAlias(scope int, fName, alias string) {
	fs, ok := lr.scopes[scope]
	if !ok {
		return
	}

	if _, ok := fs.aliases[alias]; !ok {
		fs.aliases[alias] = make(map[string]struct{}, defaultAliasesCount)
	}

	fs.aliases[alias][fName] = struct{}{}
}

func (lr *linter) registerCall(scope int, op opTyp, call *ast.CallExpr) {
	var (
		funcName string
		err      error
	)

	switch funExprType := call.Fun.(type) {
	case *ast.SelectorExpr:
		if funcName, err = lr.getFullSelectorName(funExprType); err != nil {
			return
		}

	case *ast.Ident:
		funcName = funExprType.Name
	}

	step := callContext{
		opNo:             lr.opNo, // 32bits
		opType:           op,      // 4bit
		scope:            scope,   // 20bit
		boundedFuncAlias: funcName,
		start:            int(call.Pos()),
		end:              int(call.End()),
	}

	lr.calls.PushBack(step)
}

func (lr *linter) analyzeCalls() ([]report, error) {
	var (
		r       *report
		callCtx callContext
		ok      bool
	)

	reports := make(map[int]*report)

	for lr.calls.Len() != 0 {
		call := lr.calls.Front()
		lr.calls.Remove(call)

		callCtx, ok = call.Value.(callContext)
		if !ok {
			return nil, errors.New("call structure is not a 'callContext'")
		}

		funcScope, ok := lr.scopes[callCtx.scope]
		if !ok {
			continue
		}

		if callCtx.boundedFuncAlias != funcScope.name {
			if _, ok := funcScope.aliases[callCtx.boundedFuncAlias]; !ok {
				continue
			}
		}

		r, ok = reports[callCtx.opNo]
		if !ok {
			reports[callCtx.opNo] = &report{
				start:    callCtx.start,
				funcName: funcScope.name,
				opType:   callCtx.opType,
				opNo:     callCtx.opNo,
				count:    1,
			}

			continue
		}

		if r.opType == callCtx.opType && r.opNo == callCtx.opNo {
			// we found another call in current operation with same operation type
			// and we have to register it
			r.count++
			if callCtx.start < r.start {
				// report have to start from start
				r.start = callCtx.start
			}
		}
	}

	result := make([]report, 0, len(reports))
	for _, report := range reports {
		if report.count >= 2 {
			result = append(result, *report)
		}
	}

	return result, nil
}
