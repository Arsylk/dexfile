package dexfile_test

import (
	"fmt"
	"os"

	"github.com/arsylk/dexfile"
)

// Example parses a DEX file and walks its classes, methods and instructions.
func Example() {
	data, err := os.ReadFile("testdata/dex/tiny.dex")
	if err != nil {
		panic(err)
	}
	f, err := dexfile.Parse(data) // bounds-safe: hostile input yields an error, never a panic
	if err != nil {
		panic(err)
	}
	for i, cd := range f.ClassDefs {
		fmt.Println(f.TypeDesc(cd.ClassIdx))
		cls := f.ClassData(uint32(i))
		if cls == nil {
			continue // no class_data_item (e.g. marker interface)
		}
		for _, m := range append(cls.DirectMethods, cls.VirtualMethods...) {
			if m.CodeOff == 0 {
				fmt.Printf("  %s (abstract or native)\n", f.MethodDesc(m.MethodIdx))
				continue
			}
			code, err := f.Code(m.CodeOff)
			if err != nil {
				panic(err)
			}
			recs, err := code.InsnRecs(f)
			if err != nil {
				panic(err)
			}
			first, _ := code.Decode(f, recs[0].Off)
			fmt.Printf("  %s: %d instructions, registers=%d, first=%s\n",
				f.MethodDesc(m.MethodIdx), len(recs), code.RegistersSize, first.Name())
		}
	}
	// Output:
	// Lfixtures/tiny/Hello;
	//   Lfixtures/tiny/Hello;-><init>()V: 2 instructions, registers=1, first=invoke-direct
	//   Lfixtures/tiny/Hello;->main([Ljava/lang/String;)V: 4 instructions, registers=3, first=sget-object
}
