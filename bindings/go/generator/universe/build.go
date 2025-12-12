package universe

import (
	"context"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
	"golang.org/x/tools/go/packages"
)

const RuntimePackage = "ocm.software/open-component-model/bindings/go/runtime"

// New creates an empty Universe.
func New() *Universe {
	return &Universe{
		Types:      map[TypeKey]*TypeInfo{},
		ImportMaps: map[string]map[string]string{},
	}
}

// Universe represents all Go types and imports discovered during scanning.
//
//   - Types maps each discovered struct type to its TypeInfo.
//   - ImportMaps tracks, per file, the import alias → full import path mapping.
//     This is used to resolve selector expressions (e.g. external.Type).
//
// The Universe is immutable after Build() and is consumed by the Generator.
type Universe struct {
	Types      map[TypeKey]*TypeInfo        // all named struct types in all scanned packages
	ImportMaps map[string]map[string]string // pkgPath → (alias → full package path)
}

// TypeKey uniquely identifies a Go type within the Universe.
//
// PkgPath is the resolved Go module import path of the package containing
// the type (e.g. "ocm.software/open-component-model/bindings/go/runtime").
//
// TypeName is the name of the exported type (e.g. "Raw", "Type").
type TypeKey struct {
	PkgPath  string
	TypeName string
}

// TypeInfo stores all structural information needed for schema generation.
//
// Key is the unique (PkgPath, TypeName) key identifying the type.
// Struct is the underlying *ast.StructType of the named struct.
// File is the parsed *ast.File containing the type definition.
// FilePath is the absolute path to the Go source file declaring the type.
// TypeSpec is the *ast.TypeSpec for the type.
// GenDecl is the surrounding *ast.GenDecl, used for comment extraction.
//
// TypeInfo does NOT store whether the type should be emitted — that is
// determined by the generator (root type) or by reference tracking.
//
// Extended: Consts holds constants explicitly declared with this type,
// detected via AST-based analysis in RegisterConstsFromFile.
type TypeInfo struct {
	Key      TypeKey
	Expr     ast.Expr
	Struct   *ast.StructType
	FilePath string
	TypeSpec *ast.TypeSpec
	GenDecl  *ast.GenDecl
	Obj      *types.TypeName // canonical type identity
	Consts   []*Const
	Pkg      *packages.Package
}

// ConstInfo stores AST information about a single constant belonging to a type.
//
// The Value field may be nil if the constant declaration does not include
// an explicit RHS for this name (e.g., `const ( A = "x"; B )`).
type ConstInfo struct {
	Name    string   // identifier name (e.g. "SignatureEncodingPolicyPlain")
	Value   ast.Expr // literal/expression assigned (may be nil)
	Doc     *ast.CommentGroup
	Comment *ast.CommentGroup
	Obj     *types.Const // ⬅ canonical const identity
}

type Const struct {
	Name    string            // Go const name (optional but useful)
	Obj     *types.Const      // ⬅ canonical const identity
	Doc     *ast.CommentGroup // doc comment associated with the enum value
	Comment *ast.CommentGroup // doc comment associated with the value
}

func (c *ConstInfo) Literal() (string, bool) {
	if c.Obj == nil {
		return "", false
	}

	v := c.Obj.Val()
	if v.Kind() != constant.String {
		return "", false
	}

	return constant.StringVal(v), true
}

// Definition generates an absolute, globally-unique $defs key:
//
//	ocm.software.open-component-model.bindings.go.runtime.Raw
//
// The convention is:
//
//	<pkgPath with "/" replaced by "."> + "." + <typeName>
//
// This is the canonical identity for schemas.
func Definition(key TypeKey) string {
	// Convert pkgPath from slashes → dots
	pkg := strings.ReplaceAll(key.PkgPath, "/", ".")
	return pkg + "." + key.TypeName
}

func IsRuntimeType(ti *TypeInfo) bool {
	return ti.Key.PkgPath == RuntimePackage && ti.Key.TypeName == "Type"
}

func IsRuntimeRaw(ti *TypeInfo) bool {
	return ti.Key.PkgPath == RuntimePackage && ti.Key.TypeName == "Raw"
}

func IsRuntimeTyped(ti *TypeInfo) bool {
	return ti.Key.PkgPath == RuntimePackage && ti.Key.TypeName == "Typed"
}

// Build loads all Go files from the given root directories and
// populates the Universe with import maps and discovered structs.
func Build(ctx context.Context, roots []string) (*Universe, error) {
	u := New()

	modRoots, err := findModuleRoots(roots)
	if err != nil {
		return nil, err
	}

	g, ctx := errgroup.WithContext(ctx)

	var mu sync.Mutex

	for _, modRoot := range modRoots {
		modRoot := modRoot
		g.Go(func() error {
			pkgs, err := u.loadModule(ctx, modRoot)
			if err != nil {
				return err
			}

			mu.Lock()
			defer mu.Unlock()
			for _, pkg := range pkgs {
				u.recordImportsOnce(pkg)
				for i, file := range pkg.Syntax {
					u.scanFile(pkg, pkg.GoFiles[i], file)
				}
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return u, nil
}

// LookupType retrieves a type by full package path and name.
func (u *Universe) LookupType(pkgPath, typeName string) *TypeInfo {
	key := TypeKey{PkgPath: pkgPath, TypeName: typeName}
	return u.Types[key]
}

// ResolveIdent attempts to resolve an ident (Raw, LocalBlobAccess, etc.)
// either to a type in the same package, or to an imported alias to another package.
//
// This is required because fields may use:
//
//	runtime.Raw
//	Raw                  (alias)
//	type Raw = runtime.Raw
//	type Raw runtime.Raw
//	type Raw *runtime.Raw
func (u *Universe) ResolveIdent(
	pkgPath string,
	id *ast.Ident,
) (*TypeInfo, bool) {

	if id == nil {
		return nil, false
	}

	ti, ok := u.Types[TypeKey{
		PkgPath:  pkgPath,
		TypeName: id.Name,
	}]
	return ti, ok
}

func (u *Universe) ResolveTypeFromTypes(t types.Type) (*TypeInfo, bool) {
	named, ok := t.(*types.Named)
	if !ok {
		return nil, false
	}

	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return nil, false
	}

	key := TypeKey{
		PkgPath:  obj.Pkg().Path(),
		TypeName: obj.Name(),
	}

	ti := u.Types[key]
	return ti, ti != nil
}

func (u *Universe) ResolveFieldType(
	ctx *TypeInfo,
	field *ast.Field,
) (*TypeInfo, bool) {
	if ctx == nil || ctx.Obj.Type() == nil || field == nil {
		return nil, false
	}

	named, ok := ctx.Obj.Type().(*types.Named)
	if !ok {
		return nil, false
	}

	st, ok := named.Underlying().(*types.Struct)
	if !ok {
		return nil, false
	}

	// Match by field position (AST and types align)
	for i := 0; i < st.NumFields(); i++ {
		tf := st.Field(i)

		if len(field.Names) > 0 && tf.Name() == field.Names[0].Name {
			return u.ResolveTypeFromTypes(tf.Type())
		}
	}

	return nil, false
}

func (u *Universe) ResolveIdentViaTypes(
	ctx *TypeInfo,
	id *ast.Ident,
) (*TypeInfo, bool) {

	if id == nil || ctx == nil || ctx.Obj == nil {
		return nil, false
	}

	pkg := ctx.Obj.Pkg()
	if pkg == nil {
		return nil, false
	}

	// Look up in package scope
	obj := pkg.Scope().Lookup(id.Name)
	if obj == nil {
		return nil, false
	}

	tn, ok := obj.(*types.TypeName)
	if !ok {
		return nil, false
	}

	key := TypeKey{
		PkgPath:  pkg.Path(),
		TypeName: tn.Name(),
	}

	ti := u.Types[key]
	return ti, ti != nil
}

func (u *Universe) ResolveSelectorViaTypes(
	info *types.Info,
	sel *ast.SelectorExpr,
) (*TypeInfo, bool) {

	if info == nil || sel == nil {
		return nil, false
	}

	// sel.Sel is the identifier being selected
	obj := info.Uses[sel.Sel]
	if obj == nil {
		return nil, false
	}

	tn, ok := obj.(*types.TypeName)
	if !ok {
		return nil, false
	}

	if tn.Pkg() == nil {
		return nil, false
	}

	key := TypeKey{
		PkgPath:  tn.Pkg().Path(),
		TypeName: tn.Name(),
	}

	ti := u.Types[key]
	return ti, ti != nil
}

// ResolveSelector resolves a SelectorExpr like `foo.Bar` using the import map
// of the file that references it.
func (u *Universe) ResolveSelector(
	pkgPath string,
	sel *ast.SelectorExpr,
) (*TypeInfo, bool) {

	if sel == nil {
		return nil, false
	}

	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil, false
	}

	imports := u.ImportMaps[pkgPath]
	if imports == nil {
		return nil, false
	}

	importedPkg, ok := imports[pkgIdent.Name]
	if !ok {
		return nil, false
	}

	ti, ok := u.Types[TypeKey{
		PkgPath:  importedPkg,
		TypeName: sel.Sel.Name,
	}]
	return ti, ok
}

func findModuleRoots(roots []string) ([]string, error) {
	seen := map[string]struct{}{}
	var modules []string

	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !d.IsDir() {
				return nil
			}

			mod := filepath.Join(path, "go.mod")
			if _, err := os.Stat(mod); err == nil {
				abs, err := filepath.Abs(path)
				if err != nil {
					return err
				}

				if _, ok := seen[abs]; !ok {
					seen[abs] = struct{}{}
					modules = append(modules, abs)
				}

				// Do NOT descend into nested modules again
				return filepath.SkipDir
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	if len(modules) == 0 {
		return nil, fmt.Errorf("no go.mod found in provided roots")
	}

	return modules, nil
}

func (u *Universe) loadModule(ctx context.Context, modRoot string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Context: ctx,
		Mode: packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedFiles |
			packages.NeedImports,
		Dir:   modRoot,
		Tests: false,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, err
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("package load errors in module %s", modRoot)
	}

	return pkgs, nil
}

func (u *Universe) scanFile(pkg *packages.Package, filePath string, file *ast.File) {
	pkgPath := pkg.Types.Path()

	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}

		switch gd.Tok {

		case token.TYPE:
			for _, spec := range gd.Specs {
				ts := spec.(*ast.TypeSpec)

				obj, ok := pkg.TypesInfo.Defs[ts.Name].(*types.TypeName)
				if !ok {
					continue
				}

				key := TypeKey{PkgPath: pkgPath, TypeName: obj.Name()}
				if _, exists := u.Types[key]; exists {
					continue
				}

				st, _ := ts.Type.(*ast.StructType)

				u.Types[key] = &TypeInfo{
					Key:      key,
					FilePath: filePath,
					TypeSpec: ts,
					GenDecl:  gd,
					Expr:     ts.Type,
					Struct:   st,
					Obj:      obj,
					Pkg:      pkg,
				}
			}

		case token.CONST:
			u.registerConstsFromDecl(pkg, gd)
		}
	}
}

func (u *Universe) registerConstsFromDecl(pkg *packages.Package, gd *ast.GenDecl) {
	for _, spec := range gd.Specs {
		vs := spec.(*ast.ValueSpec)

		for _, name := range vs.Names {
			obj, ok := pkg.TypesInfo.Defs[name].(*types.Const)
			if !ok {
				continue
			}

			named, ok := obj.Type().(*types.Named)
			if !ok {
				continue
			}

			key := TypeKey{
				PkgPath:  obj.Pkg().Path(),
				TypeName: named.Obj().Name(),
			}

			ti := u.Types[key]
			if ti == nil {
				continue
			}

			ti.Consts = append(ti.Consts, &Const{
				Name:    obj.Name(),
				Obj:     obj,
				Doc:     vs.Doc,
				Comment: vs.Comment,
			})
		}
	}
}

func (u *Universe) recordImportsOnce(pkg *packages.Package) {
	pkgPath := pkg.Types.Path()
	if _, exists := u.ImportMaps[pkgPath]; exists {
		return
	}

	m := make(map[string]string, len(pkg.Types.Imports()))
	for _, imp := range pkg.Types.Imports() {
		alias := imp.Name()
		if alias == "" {
			alias = path.Base(imp.Path())
		}
		m[alias] = imp.Path()
	}

	u.ImportMaps[pkgPath] = m
}
