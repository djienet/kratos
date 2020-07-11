package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/djienet/kratos/tool/protobuf/pkg/gen"
	"github.com/djienet/kratos/tool/protobuf/pkg/generator"
	bmgen "github.com/djienet/kratos/tool/protobuf/protoc-gen-bm/generator"
)

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *versionFlag {
		fmt.Println(generator.Version)
		os.Exit(0)
	}

	g := bmgen.BmGenerator()
	gen.Main(g)
}
