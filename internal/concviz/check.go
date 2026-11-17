package concviz

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"path"
	"slices"
	"strings"
)

type needKind uint8

const (
	needFunc   needKind = iota // the function exists
	needMake                   // the function makes the channel with this capacity and element type
	needSend                   // c <- x
	needRecv                   // <-c
	needRange                  // for x := range c
	needClose                  // close(c)
	needSelect                 // a select with exactly these cases, and a default or not
	needCall                   // a call of one of codes, e.g. "wg.Wait"
	needGo                     // a go statement
)

// need is a claim of a story about the code: some function among fns
// contains the construct.
type need struct {
	kind  needKind
	fns   []string
	codes []string // how the code names the channel (any of them), or the call
	text  string   // the capacity expression of make
	elem  string   // the element type of make
	cases []caseNeed
	dflt  bool
}

type caseNeed struct {
	send  bool
	codes []string
}

func (n need) String() string {
	fns := strings.Join(n.fns, " or ")
	on := strings.Join(n.codes, " or ")
	switch n.kind {
	case needFunc:
		return "function " + fns
	case needMake:
		c := "no capacity"
		if n.text != "" {
			c = "capacity " + n.text
		}
		return fmt.Sprintf("%s makes %s as chan %s with %s", fns, on, n.elem, c)
	case needSend:
		return fns + " sends on " + on
	case needRecv:
		return fns + " receives from " + on
	case needRange:
		return fns + " ranges over " + on
	case needClose:
		return fns + " closes " + on
	case needSelect:
		var ks []string
		for _, k := range n.cases {
			if k.send {
				ks = append(ks, strings.Join(k.codes, "/")+" <- …")
			} else {
				ks = append(ks, "<-"+strings.Join(k.codes, "/"))
			}
		}
		if n.dflt {
			ks = append(ks, "default")
		}
		return fns + " has select { " + strings.Join(ks, "; ") + " }"
	case needCall:
		return fns + " calls " + on
	case needGo:
		return fns + " has a go statement"
	}
	return "?"
}

// Source is the parsed code of the module, by function: "pkg.Func" or
// "pkg.Type.Method", pkg being the package name.
type Source struct {
	funcs map[string]*ast.FuncDecl
	dup   map[string]bool
}

// LoadSource parses the Go files of fsys (the module root), without tests and
// testdata.
func LoadSource(fsys fs.FS) (*Source, error) {
	src := &Source{funcs: map[string]*ast.FuncDecl{}, dup: map[string]bool{}}
	fset := token.NewFileSet()
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p != "." && (name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return fs.SkipDir
			}
			return nil
		}
		if path.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		f, err := parser.ParseFile(fset, p, b, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			key := f.Name.Name + "." + fd.Name.Name
			if fd.Recv != nil && len(fd.Recv.List) == 1 {
				key = f.Name.Name + "." + recvName(fd.Recv.List[0].Type) + "." + fd.Name.Name
			}
			if _, ok := src.funcs[key]; ok {
				src.dup[key] = true
			}
			src.funcs[key] = fd
		}
		return nil
	})
	return src, err
}

func recvName(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.StarExpr:
		return recvName(e.X)
	case *ast.IndexExpr:
		return recvName(e.X)
	case *ast.IndexListExpr:
		return recvName(e.X)
	case *ast.Ident:
		return e.Name
	}
	return "?"
}

// Check returns every claim of s that the source does not satisfy.
func (s *Story) Check(src *Source) []error {
	var errs []error
	for _, n := range s.needs {
		if err := src.check(n); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.File, err))
		}
	}
	return errs
}

func (src *Source) check(n need) error {
	var bodies []*ast.FuncDecl
	for _, f := range n.fns {
		fd, ok := src.funcs[f]
		if !ok {
			return fmt.Errorf("%s: function %s not found", n, f)
		}
		if src.dup[f] {
			return fmt.Errorf("%s: function %s is ambiguous", n, f)
		}
		bodies = append(bodies, fd)
	}
	if n.kind == needFunc {
		return nil
	}
	var hint string // what was found instead, for make
	for _, fd := range bodies {
		found := false
		ast.Inspect(fd.Body, func(x ast.Node) bool {
			if found {
				return false
			}
			ok, h := n.matches(x)
			found = ok
			if h != "" {
				hint = h
			}
			return true
		})
		if found {
			return nil
		}
	}
	if hint != "" {
		return fmt.Errorf("%s: not found, but %s", n, hint)
	}
	return fmt.Errorf("%s: not found", n)
}

// matches reports whether x is the construct of n; for make, a hint says how
// a channel of that name is made instead.
func (n need) matches(x ast.Node) (bool, string) {
	str := types.ExprString
	is := func(e ast.Expr) bool { return slices.Contains(n.codes, str(e)) }
	switch n.kind {
	case needMake:
		check := func(name ast.Expr, val ast.Expr) (bool, string) {
			if !slices.Contains(lastNames(n.codes), lastName(name)) {
				return false, ""
			}
			call, ok := val.(*ast.CallExpr)
			if !ok || str(call.Fun) != "make" || len(call.Args) == 0 {
				return false, ""
			}
			ct, ok := call.Args[0].(*ast.ChanType)
			if !ok {
				return false, ""
			}
			capText := ""
			if len(call.Args) > 1 {
				capText = str(call.Args[1])
			}
			if capText == n.text && str(ct.Value) == n.elem {
				return true, ""
			}
			return false, fmt.Sprintf("it is made as %s", str(call))
		}
		switch x := x.(type) {
		case *ast.AssignStmt:
			for i, r := range x.Rhs {
				if i < len(x.Lhs) && len(x.Lhs) == len(x.Rhs) {
					if ok, h := check(x.Lhs[i], r); ok || h != "" {
						return ok, h
					}
				}
			}
		case *ast.KeyValueExpr:
			return check(x.Key, x.Value)
		case *ast.ValueSpec:
			for i, v := range x.Values {
				if ok, h := check(x.Names[i], v); ok || h != "" {
					return ok, h
				}
			}
		}
	case needSend:
		if x, ok := x.(*ast.SendStmt); ok {
			return is(x.Chan), ""
		}
	case needRecv:
		if x, ok := x.(*ast.UnaryExpr); ok && x.Op == token.ARROW {
			return is(x.X), ""
		}
	case needRange:
		if x, ok := x.(*ast.RangeStmt); ok {
			return is(x.X), ""
		}
	case needClose:
		if x, ok := x.(*ast.CallExpr); ok && str(x.Fun) == "close" && len(x.Args) == 1 {
			return is(x.Args[0]), ""
		}
	case needCall:
		if x, ok := x.(*ast.CallExpr); ok {
			return is(x.Fun), ""
		}
	case needGo:
		_, ok := x.(*ast.GoStmt)
		return ok, ""
	case needSelect:
		if x, ok := x.(*ast.SelectStmt); ok {
			return n.selectMatches(x), ""
		}
	}
	return false, ""
}

func (n need) selectMatches(sel *ast.SelectStmt) bool {
	var comms []ast.Stmt
	dflt := false
	for _, cl := range sel.Body.List {
		c := cl.(*ast.CommClause)
		if c.Comm == nil {
			dflt = true
		} else {
			comms = append(comms, c.Comm)
		}
	}
	if dflt != n.dflt || len(comms) != len(n.cases) {
		return false
	}
	used := make([]bool, len(comms))
	for _, k := range n.cases {
		hit := false
		for i, c := range comms {
			if !used[i] && commIs(c, k) {
				used[i], hit = true, true
				break
			}
		}
		if !hit {
			return false
		}
	}
	return true
}

func commIs(c ast.Stmt, k caseNeed) bool {
	if k.send {
		s, ok := c.(*ast.SendStmt)
		return ok && slices.Contains(k.codes, types.ExprString(s.Chan))
	}
	var e ast.Expr
	switch c := c.(type) {
	case *ast.ExprStmt:
		e = c.X
	case *ast.AssignStmt:
		if len(c.Rhs) == 1 {
			e = c.Rhs[0]
		}
	}
	u, ok := e.(*ast.UnaryExpr)
	return ok && u.Op == token.ARROW && slices.Contains(k.codes, types.ExprString(u.X))
}

func lastName(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	case *ast.IndexExpr:
		return lastName(e.X)
	}
	return ""
}

func lastNames(codes []string) []string {
	var out []string
	for _, c := range codes {
		out = append(out, c[strings.LastIndex(c, ".")+1:])
	}
	return out
}
