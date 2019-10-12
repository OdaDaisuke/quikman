package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

func main() {
	dirF := flag.String("dir", "", "dir must be set")
	log.SetFlags(0)
	log.SetPrefix("db_quik: ")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	dir := *dirF
	if len(dir) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	if !IsDirectory(dir) {
		log.Fatalf("not supported files")
		os.Exit(2)
	}
	g, err := NewGenerator(dir)
	if err != nil {
		log.Fatalf("cannot process directory %s: %s", dir, err)
	}

	files := make([]File, len(g.pkg.gofiles))
	for i, v := range g.pkg.gofiles {
		files[i] = File{
			Name: fmt.Sprintf("%s/%s", g.pkg.dir, v),
		}
	}
	g.pkg.files = files

	fs := token.NewFileSet()
	for i, v := range g.pkg.files {
		parsedFile, err := parser.ParseFile(fs, v.Name, nil, 0)
		if err != nil {
			log.Fatalf("parsing package: %s: %s", v.Name, err)
		}
		g.pkg.files[i].AstFile = parsedFile
	}

	for _, v := range g.pkg.files {
		for _, decl := range v.AstFile.Decls {
			g.BufferReset()
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}

			var existingFile bytes.Buffer

			// override import specs
			if genDecl.Tok == token.IMPORT {
				genDecl.Specs = AppendImportPackage("database/sql", genDecl.Specs)
				v.AstFile.Decls[0] = genDecl
			}

			if genDecl.Tok != token.TYPE {
				continue
			}

			format.Node(&existingFile, token.NewFileSet(), v.AstFile)
			g.Printfln("%s\n", existingFile.String())

			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				name := typeSpec.Name.Name
				if !strings.Contains(name, "Model") {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				if err := g.Setup(name, v.AstFile.Decls, genDecl.Specs); err != nil {
					log.Printf("%s\n", err)
					continue
				}
				g.Generate(structType)
				src := g.Format()
				err := ioutil.WriteFile(v.Name, src, 0644)
				if err != nil {
					log.Fatalf("writing output: %s", err)
				}
			}

		}
	}
}
