package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/lexx13/deep-copy/deepcopy"
)

var (
	pointerReceiverF            = flag.Bool("pointer-receiver", false, "the generated receiver type")
	maxDepthF                   = flag.Int("maxdepth", 0, "max depth of deep copying")
	methodF                     = flag.String("method", "DeepCopy", "deep copy method name")
	returnInterfaceF            = flag.String("return-interface", "", "the generated return type of DeepCopy function")
	returnInterfaceDepF         = flag.String("return-interface-dep", "", "return interface dep name")
	returnInterfaceDepPathF     = flag.String("return-interface-dep-path", "", "return interface dep path")
	allowedCopyToAnotherStructF = flag.Bool("another-struct", false, "add parameter for copy to another struct. allowed only with pointer-receiver")

	typesF     typesVal
	skipsF     skipsVal
	outputF    outputVal
	buildTagsF buildTagsVal
)

type typesVal []string

func (f *typesVal) String() string {
	return strings.Join(*f, ",")
}

func (f *typesVal) Set(v string) error {
	*f = append(*f, v)
	return nil
}

type skipsVal deepcopy.SkipLists

func (f *skipsVal) String() string {
	parts := make([]string, 0, len(*f))
	for _, m := range *f {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		parts = append(parts, strings.Join(keys, ","))
	}

	return strings.Join(parts, ",")
}

func (f *skipsVal) Set(v string) error {
	parts := strings.Split(v, ",")
	set := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		set[p] = struct{}{}
	}

	*f = append(*f, set)

	return nil
}

type outputVal struct {
	file *os.File
	name string
}

func (f *outputVal) String() string {
	return f.name
}

func (f *outputVal) Set(v string) error {
	if v == "-" || v == "" {
		f.name = "stdout"

		if f.file != nil {
			_ = f.file.Close()
		}
		f.file = nil

		return nil
	}

	file, err := os.OpenFile(v, os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		return fmt.Errorf("opening file: %v", v)
	}

	f.name = v
	f.file = file

	return nil
}

func (f *outputVal) Open() (io.WriteCloser, error) {
	if f.file == nil {
		f.file = os.Stdout
	} else {
		err := f.file.Truncate(0)
		if err != nil {
			return nil, err
		}
	}

	return f.file, nil
}

type buildTagsVal []string

func (b *buildTagsVal) String() string {
	return strings.Join(*b, ",")
}

func (b *buildTagsVal) Set(v string) error {
	*b = append(*b, v)
	return nil
}

func init() {
	flag.Var(&typesF, "type", "the concrete type. Multiple flags can be specified")
	flag.Var(&skipsF, "skip", "comma-separated field/slice/map selectors to shallow copy. Multiple flags can be specified")
	flag.Var(&outputF, "o", "the output file to write to. Defaults to STDOUT")
	flag.Var(&buildTagsF, "tags", "comma-separated build tags to add to generated file")
}

func main() {
	flag.Parse()

	if len(typesF) == 0 || typesF[0] == "" {
		log.Fatalln("no type given")
	}

	if flag.NArg() != 1 {
		log.Fatalln("No package path given")
	}

	sl := deepcopy.SkipLists(skipsF)
	generator := deepcopy.NewGenerator(*pointerReceiverF, *methodF, sl, *maxDepthF, *allowedCopyToAnotherStructF, *returnInterfaceF, *returnInterfaceDepF, *returnInterfaceDepPathF, buildTagsF)

	output, err := outputF.Open()
	if err != nil {
		log.Fatalln("Error initializing output file:", err)
	}

	err = run(generator, output, flag.Args()[0], typesF)
	if err != nil {
		log.Fatalln("Error generating deep copy method:", err)
	}

	output.Close()
}

func run(
	g deepcopy.Generator, w io.Writer, path string, types typesVal,
) error {
	packages, err := load(path)
	if err != nil {
		return fmt.Errorf("loading package: %v", err)
	}
	if len(packages) == 0 {
		return errors.New("no package found")
	}

	return g.Generate(w, types, packages[0])
}

func load(patterns string) ([]*packages.Package, error) {
	return packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedDeps | packages.NeedImports,
	}, patterns)
}
