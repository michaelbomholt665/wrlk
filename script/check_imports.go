package main

import (
	"fmt"
	"os/exec"
	"strings"
	"encoding/json"
)

type Package struct {
	ImportPath string
	Imports    []string
}

func main() {
	cmd := exec.Command("go", "list", "-json", "./...")
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error running go list: %v\n", err)
		return
	}

	decoder := json.NewDecoder(strings.NewReader(string(out)))
	for decoder.More() {
		var pkg Package
		err := decoder.Decode(&pkg)
		if err != nil {
			fmt.Printf("Error decoding json: %v\n", err)
			return
		}

		if strings.Contains(pkg.ImportPath, "internal/ports") {
			for _, imp := range pkg.Imports {
				if strings.Contains(imp, "internal/router") || strings.Contains(imp, "internal/adapters") {
					fmt.Printf("VIOLATION: ports pkg %s imports %s\n", pkg.ImportPath, imp)
				}
			}
		}

		if strings.Contains(pkg.ImportPath, "internal/adapters") {
			for _, imp := range pkg.Imports {
				if strings.Contains(imp, "internal/adapters") && !strings.HasPrefix(pkg.ImportPath, imp) && pkg.ImportPath != imp {
					// Need to make sure it's not importing itself or its parent? Adapters shouldn't import other adapters.
					// Actually, no adapter should import any other adapter package.
                    // Let's just flag any import that has internal/adapters
                    if imp != pkg.ImportPath {
					    fmt.Printf("VIOLATION: adapter pkg %s imports %s\n", pkg.ImportPath, imp)
                    }
				}
			}
		}

		if strings.Contains(pkg.ImportPath, "internal/router") && !strings.Contains(pkg.ImportPath, "internal/router/ext") {
			for _, imp := range pkg.Imports {
				if strings.Contains(imp, "internal/adapters") || strings.Contains(imp, "internal/ports") {
					fmt.Printf("VIOLATION: router pkg %s imports %s\n", pkg.ImportPath, imp)
				}
			}
		}
	}
	fmt.Println("Check complete.")
}
