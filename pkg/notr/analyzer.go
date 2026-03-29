package notr

import (
	"fmt"
	"go/ast"
	"go/token"
	"math"
	"reflect"
	"slices"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	linterName = "notr"

	defaultAliasesCount = 8
	defaultScopesCount  = 64
)

const (
	fnCallInArgs opTyp = iota
	fnCallInBinOp
)

var (
	ErrTooManyOperations = errors.New("too many operations")
	ErrNoIdent           = errors.New("selector.X is not an *ast.Ident node")
	ErrNoReceiver        = errors.New("recursion call impossible: no receiver")
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

type scopesMap struct {
	m      *sync.RWMutex
	scopes map[int]*funcScope
}

func (m *scopesMap) GetScope(key int) (*funcScope, bool) {
	m.m.RLock()
	scope, ok := m.scopes[key]
	m.m.RUnlock()

	return scope, ok
}

func (m *scopesMap) AddScope(key int, scope *funcScope) {
	m.m.RLock()
	_, ok := m.scopes[key]
	m.m.RUnlock()

	if ok {
		return
	}

	m.m.Lock()
	defer m.m.Unlock()

	m.scopes[key] = scope
}

type linter struct {
	calls  []callContext // deque of steps
	scopes scopesMap     // function scope to localize calls
	opNo   int
}

func NewLinter() *linter {
	return &linter{
		calls: make([]callContext, 0),
		scopes: scopesMap{
			m:      &sync.RWMutex{},
			scopes: make(map[int]*funcScope, defaultScopesCount),
		},
		opNo: 0,
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
	astCursor.Inspect(wishedNodes, lr.inspectAst)

	reports := lr.analyzeCalls()
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
	var errMsgHeader = "inspectAst"

	switch nodeType := c.Node().(type) {
	case *ast.FuncDecl:
		scopeNo := lr.opNo

		if err := lr.handleFuncDeclaration(nodeType, scopeNo); err != nil {
			if errors.Is(err, ErrNoReceiver) {
				return false
			}

			panic(errors.Errorf("%s: node %q: %v", errMsgHeader, nodeType.Name.Name, err))
		}

		for cursor := range c.Preorder(&ast.AssignStmt{}) {
			if err := lr.handleAssignmentExpression(cursor, scopeNo); err != nil {
				if !errors.Is(err, ErrNoIdent) {
					panic(errors.Errorf("%s: %v", errMsgHeader, err))
				}
			}
		}

		// try to fetch like a(a(1)) - a(a(a(-2)))
		for cursor := range c.Preorder(&ast.BinaryExpr{}) {
			if err := lr.preorderNestedCall(cursor, scopeNo, fnCallInBinOp); err != nil {
				panic(errors.Errorf("%s: %v", errMsgHeader, err))
			}
		}

		for cursor := range c.Preorder(&ast.CallExpr{}) {
			if err := lr.preorderNestedCall(cursor, scopeNo, fnCallInArgs); err != nil {
				panic(errors.Errorf("%s: %v", errMsgHeader, err))
			}
		}

		if err := lr.incOp(); err != nil {
			panic(errors.Errorf("%s: %v", errMsgHeader, err))
		}
	}

	return false
}

func (lr *linter) handleFuncDeclaration(decl *ast.FuncDecl, scope int) error {
	if decl.Recv == nil {
		lr.registerFunc(scope, decl.Name.Name)

		return nil
	}

	if decl.Recv.List == nil {
		return errors.Errorf("handle func declaration: symbol %q has no receiver field", decl.Name.Name)
	}

	err := ErrNoReceiver

	field := decl.Recv.List[0]
	if len(field.Names) != 0 {
		methodName := field.Names[0].Name + "." + decl.Name.Name
		lr.registerFunc(scope, methodName)
		err = nil
	}

	// we may have method declaration like:
	//		func (*structName) X() any {}
	//
	// there is no receiver, so we can't make a recursion call.
	return err
}

func (lr *linter) handleAssignmentExpression(c inspector.Cursor, scope int) error {
	if err := lr.incOp(); err != nil {
		errors.Wrap(err, "handle assignment: too many operations")
	}

	// Get function scope. It have to be created at the moment
	functionScope, ok := lr.scopes.GetScope(scope)
	if !ok {
		return errors.New("handle assignment: function scope does not exists")
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
			//
			// compare RHS.Name with function name - optimize memory usage.
			if lr.needRegisterAlias(functionScope, identRight.Name) {
				lr.registerAlias(scope, identRight.Name, identLeft.Name)
			}

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

		if lr.needRegisterAlias(functionScope, methodName) {
			lr.registerAlias(scope, methodName, identLeft.Name)
		}
	}

	return nil
}

func (lr *linter) needRegisterAlias(functionScope *funcScope, rhsName string) bool {
	namesEqual := functionScope.name == rhsName
	_, aliasExists := functionScope.aliases[rhsName]
	return namesEqual || aliasExists
}

func (lr *linter) getFullSelectorName(s *ast.SelectorExpr) (string, error) {
	receiverIdent, ok := s.X.(*ast.Ident)
	if !ok {
		return "", errors.Wrapf(ErrNoIdent, "get full selector name: skip unsupported selector 'X' type: %T", s.X)
	}

	return receiverIdent.Name + "." + s.Sel.Name, nil
}

func (lr *linter) preorderNestedCall(c inspector.Cursor, scope int, opType opTyp) error {
	if err := lr.incOp(); err != nil {
		return errors.Wrap(err, "preorder nested call: too many operations")
	}

	for call := range c.Preorder(&ast.CallExpr{}) {
		callExpr, _ := call.Node().(*ast.CallExpr)
		lr.registerCall(scope, opType, callExpr)
	}

	return nil
}

func (lr *linter) incOp() error {
	if lr.opNo == math.MaxInt64 {
		return errors.Errorf("increment operation: operations limit exeed: %d", math.MaxInt64)
	}

	lr.opNo++

	return nil
}

func (lr *linter) registerFunc(scope int, fName string) {
	fs := &funcScope{
		name:    fName,
		aliases: make(map[string]map[string]struct{}, defaultAliasesCount),
	}

	lr.scopes.AddScope(scope, fs)
}

// registerAlias сохраняет имя символа слева и имя символа справа, рассматривая их
// как потенциальные имена и псевдонимы функций.
func (lr *linter) registerAlias(scope int, fName, alias string) {
	fs, ok := lr.scopes.GetScope(scope)
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
		opNo:             lr.opNo,
		opType:           op,
		scope:            scope,
		boundedFuncAlias: funcName,
		start:            int(call.Pos()),
		end:              int(call.End()),
	}

	lr.calls = append(lr.calls, step)
}

func (lr *linter) analyzeCalls() []report {
	var r *report

	reports := make(map[int]*report)
	for _, callCtx := range lr.calls {
		funcScope, ok := lr.scopes.GetScope(callCtx.scope)
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

	return result
}
