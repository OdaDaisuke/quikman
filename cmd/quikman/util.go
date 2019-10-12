package main

import (
	"go/ast"
	"go/token"
	"log"
	"os"
	"strconv"
)

// AppendImportPackage appends an import package(pkg) to AST specs.
func AppendImportPackage(pkg string, specs []ast.Spec) []ast.Spec {
	specs = append(specs, &ast.ImportSpec{
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: strconv.Quote(pkg),
		},
	})
	return specs
}

// ArrayContains reports whether list contains string element of key.
func ArrayContains(list []string, key string) bool {
	for _, v := range list {
		if v == key {
			return true
		}
	}
	return false
}

// IsDirectory reports whether dir describes a directory.
func IsDirectory(dir string) bool {
	inf, err := os.Stat(dir)
	if err != nil {
		log.Fatal(err)
	}
	return inf.IsDir()
}
