package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/types"
	"log"
	"strings"
)

// Generator generates Database action code.
type Generator struct {
	buf               bytes.Buffer
	pkg               *Package
	rawModelName      string
	modelName         string
	repoName          string
	changableColNames string
	changableColMasks string
	changableColArgs  string
	readArgsWithP     string
	readArgs          string
}

func NewGenerator(dir string) (*Generator, error) {
	p, err := build.Default.ImportDir(dir, 0)
	if err != nil {
		return nil, err
	}

	return &Generator{
		pkg: &Package{
			dir:     dir,
			name:    p.Name,
			gofiles: p.GoFiles,
		},
	}, nil
}

type Package struct {
	dir     string
	name    string
	files   []File
	gofiles []string
}

type File struct {
	Name    string
	AstFile *ast.File
}

func (g *Generator) Setup(structName string, decls []ast.Decl, specs []ast.Spec) error {
	atomicCols := []string{
		"id",
		"created_at",
		"updated_at",
		"deleted_at",
		"createdat",
		"updatedat",
		"deletedat",
	}
	mIdx := strings.Index(structName, "Model")
	g.rawModelName = structName
	g.modelName = structName[:mIdx]
	g.repoName = fmt.Sprintf("%sRepository", g.modelName)
	for _, decl := range decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			if typeSpec.Name.Name == g.repoName {
				return errors.New("Repo already created -> " + typeSpec.Name.Name)
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			for _, f := range structType.Fields.List {
				fTyp := types.ExprString(f.Type)
				for _, ident := range f.Names {
					if !ArrayContains(atomicCols, strings.ToLower(ident.Name)) {
						ln := strings.ToLower(ident.Name)
						g.changableColNames += fmt.Sprintf("%s,", ln)
						g.changableColMasks += "?,"
						g.changableColArgs += fmt.Sprintf("%s %s,", ln, fTyp)
					}
					g.readArgsWithP += fmt.Sprintf("&m.%s,", ident.Name)
					g.readArgs += fmt.Sprintf("m.%s,", ident.Name)
				}
			}
		}
	}
	g.changableColNames = g.changableColNames[:len(g.changableColNames)-1]
	g.changableColMasks = g.changableColMasks[:len(g.changableColMasks)-1]
	g.changableColArgs = g.changableColArgs[:len(g.changableColArgs)-1]
	g.readArgsWithP = g.readArgsWithP[:len(g.readArgsWithP)-1]
	g.readArgs = g.readArgs[:len(g.readArgs)-1]
	return nil
}

func (g *Generator) BufferReset() {
	g.changableColNames = ""
	g.changableColMasks = ""
	g.changableColArgs = ""
	g.readArgsWithP = ""
	g.readArgs = ""
	g.buf.Reset()
}

func (g *Generator) Printfln(format string, args ...interface{}) {
	fmt.Fprintf(&g.buf, format+"\n", args...)
}

func (g *Generator) Format() []byte {
	src, err := format.Source(g.buf.Bytes())
	if err != nil {
		log.Printf("warning: an error occured while formatting generated program: %s", err)
		return g.buf.Bytes()
	}
	return src
}

func (g *Generator) Generate(structType *ast.StructType) {
	g.generateCodeHeader()

	// generate repository
	g.Printfln("type %s struct {", g.repoName)
	g.Printfln("dbCtx *sql.DB")
	g.Printfln("}\n")

	// generate initialization method
	g.Printfln("func New%s (dbCtx *sql.DB) *%s {", g.repoName, g.repoName)
	g.Printfln("	return &%s{", g.repoName)
	g.Printfln("	dbCtx: dbCtx,")
	g.Printfln("	}")
	g.Printfln("}\n")

	// create
	g.Printfln("func (r *%s) Create(%s) error {", g.repoName, g.changableColArgs)
	g.Printfln("ins, err := r.dbCtx.Prepare(\"INSERT INTO %s(%s) VALUES(%s)\")", strings.ToLower(g.modelName), g.changableColNames, g.changableColMasks)
	g.Printfln("if err != nil {")
	g.Printfln("  return err")
	g.Printfln("}")
	g.Printfln("_, err = ins.Exec(%s)", g.changableColNames)
	g.Printfln("return err")
	g.Printfln("}\n")

	// read
	g.Printfln("func (r *%s) Read(id uint64) (*%s, error) {", g.repoName, g.rawModelName)
	g.Printfln("var m *%s\n", g.rawModelName)
	g.Printfln("row := r.dbCtx.QueryRow(\"SELECT * FROM %s WHERE id = ?\", id)", g.modelName)
	g.Printfln("if err := row.Scan(%s); err != nil {", g.readArgsWithP)
	g.Printfln("  return nil, err")
	g.Printfln("}")
	g.Printfln("return m, nil")
	g.Printfln("}\n")

	// read all
	g.Printfln("func (r *%s) ReadAll() ([]*%s, error) {", g.repoName, g.rawModelName)
	g.Printfln("var res []*%s", g.rawModelName)
	g.Printfln("rows, err := r.dbCtx.Query(\"SELECT * FROM %s\")", g.modelName)
	g.Printfln("if err != nil {")
	g.Printfln("  return nil, err")
	g.Printfln("}")
	g.Printfln("for rows.Next() {")
	g.Printfln("  var m %s", g.rawModelName)
	g.Printfln("  err := rows.Scan(%s)", g.readArgsWithP)
	g.Printfln("  if err != nil {")
	g.Printfln("    return nil, err")
	g.Printfln("  }")
	g.Printfln("  res = append(res, &m)")
	g.Printfln("}")
	g.Printfln("return res, nil")
	g.Printfln("}\n")

	// update
	g.Printfln("func (r *%s) Update(id uint64) error {", g.repoName)
	g.Printfln("// FIXME Set your query")
	g.Printfln("upd, err := r.dbCtx.Prepare(\"UPDATE %s SET ...YOUR_QUERY... WHERE id = ?\")", g.modelName)
	g.Printfln("if err != nil {")
	g.Printfln("  return err")
	g.Printfln("}")
	g.Printfln("_, err = upd.Exec(id)")
	g.Printfln("return err")
	g.Printfln("}\n")

	// delete
	g.Printfln("func (r *%s) Delete(id uint64) error {", g.repoName)
	g.Printfln("_, err := r.dbCtx.Exec(\"DELETE FROM %s WHERE id = ?\", id)", g.modelName)
	g.Printfln("return err")
	g.Printfln("}\n")
}

func (g *Generator) generateCodeHeader() {
	g.Printfln("// Code generated by quikman (model: %s)", g.rawModelName)
	g.Printfln("// This section is editable.\n")
}
