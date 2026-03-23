package codegraph

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// rawEdge holds unresolved edge data before symbol IDs are assigned.
type rawEdge struct {
	FromName string
	ToName   string
	Relation string
}

// parseGoFile parses a Go source file and extracts symbols and raw edges.
func parseGoFile(path string) ([]Symbol, []rawEdge, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("parse: %w", err)
	}

	pkgName := file.Name.Name

	var symbols []Symbol
	var edges []rawEdge

	// Extract top-level declarations.
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			sym := extractFuncDecl(fset, d, pkgName)
			symbols = append(symbols, sym)

			// Extract calls made by this function.
			callEdges := extractCalls(d, sym.Name)
			edges = append(edges, callEdges...)

		case *ast.GenDecl:
			syms := extractGenDecl(fset, d, pkgName)
			symbols = append(symbols, syms...)
		}
	}

	// Extract import edges: package -> imported package.
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		edges = append(edges, rawEdge{
			FromName: pkgName,
			ToName:   importPath,
			Relation: "imports",
		})
	}

	return symbols, edges, nil
}

// extractFuncDecl extracts a Symbol from a function or method declaration.
func extractFuncDecl(fset *token.FileSet, fn *ast.FuncDecl, pkg string) Symbol {
	kind := "function"
	name := fn.Name.Name

	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		kind = "method"
		recvType := receiverTypeName(fn.Recv.List[0].Type)
		if recvType != "" {
			name = recvType + "." + fn.Name.Name
		}
	}

	return Symbol{
		Name:    name,
		Kind:    kind,
		Line:    fset.Position(fn.Pos()).Line,
		Package: pkg,
	}
}

// extractGenDecl extracts symbols from type, const, and var declarations.
func extractGenDecl(fset *token.FileSet, gd *ast.GenDecl, pkg string) []Symbol {
	var symbols []Symbol

	for _, spec := range gd.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			kind := "type"
			if _, ok := s.Type.(*ast.InterfaceType); ok {
				kind = "interface"
			}
			symbols = append(symbols, Symbol{
				Name:    s.Name.Name,
				Kind:    kind,
				Line:    fset.Position(s.Pos()).Line,
				Package: pkg,
			})

			// Extract embedded types.
			if st, ok := s.Type.(*ast.StructType); ok {
				for _, field := range st.Fields.List {
					if len(field.Names) == 0 { // embedded
						if ident := fieldTypeName(field.Type); ident != "" {
							symbols = append(symbols, Symbol{
								Name:    s.Name.Name + "." + ident,
								Kind:    "embed",
								Line:    fset.Position(field.Pos()).Line,
								Package: pkg,
							})
						}
					}
				}
			}

		case *ast.ValueSpec:
			kind := "var"
			if gd.Tok == token.CONST {
				kind = "const"
			}
			for _, name := range s.Names {
				if name.Name == "_" {
					continue
				}
				symbols = append(symbols, Symbol{
					Name:    name.Name,
					Kind:    kind,
					Line:    fset.Position(name.Pos()).Line,
					Package: pkg,
				})
			}
		}
	}

	return symbols
}

// extractCalls walks a function body and extracts direct call edges.
func extractCalls(fn *ast.FuncDecl, callerName string) []rawEdge {
	if fn.Body == nil {
		return nil
	}

	var edges []rawEdge
	seen := make(map[string]bool)

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		var calleeName string

		switch fun := call.Fun.(type) {
		case *ast.Ident:
			// Direct function call: foo()
			calleeName = fun.Name
		case *ast.SelectorExpr:
			// Method or package call: x.Foo() or pkg.Foo()
			if ident, ok := fun.X.(*ast.Ident); ok {
				calleeName = ident.Name + "." + fun.Sel.Name
			}
		}

		if calleeName != "" && !seen[calleeName] {
			seen[calleeName] = true
			edges = append(edges, rawEdge{
				FromName: callerName,
				ToName:   calleeName,
				Relation: "calls",
			})
		}

		return true
	})

	return edges
}

// receiverTypeName extracts the type name from a method receiver expression.
func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.IndexExpr:
		// Generic type: T[U]
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.IndexListExpr:
		// Generic type: T[U, V]
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	}

	return ""
}

// fieldTypeName extracts the name from a field type expression (for embedded fields).
func fieldTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return fieldTypeName(t.X)
	case *ast.SelectorExpr:
		return t.Sel.Name
	}

	return ""
}
