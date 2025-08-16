package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTranslations(t *testing.T) {
	englishKeys := make(map[string]bool)
	for key := range Messages["en"] {
		englishKeys[key] = true
	}

	for lang, translations := range Messages {
		if lang == "en" {
			continue
		}

		for key := range englishKeys {
			if _, ok := translations[key]; !ok {
				t.Errorf("Missing key '%s' in language '%s'", key, lang)
			}
		}

		for key := range translations {
			if _, ok := englishKeys[key]; !ok {
				t.Errorf("Extra key '%s' in language '%s'", key, lang)
			}
		}
	}
}

func TestI18nKeysInCode(t *testing.T) {
	englishKeys := make(map[string]bool)
	for key := range Messages["en"] {
		englishKeys[key] = true
	}

	usedKeys := make(map[string]bool)

	err := filepath.Walk("..", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}

			ast.Inspect(node, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}

				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "i18n" && sel.Sel.Name == "T" {
					if len(call.Args) > 1 {
						key, ok := call.Args[1].(*ast.BasicLit)
						if ok && key.Kind == token.STRING {
							// remove quotes from the string literal
							usedKeys[strings.Trim(key.Value, `"`)] = true
						}
					}
				}
				return true
			})
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Error walking through files: %v", err)
	}

	for key := range usedKeys {
		if _, ok := englishKeys[key]; !ok {
			t.Errorf("i18n key '%s' is used in the code but not defined in the english translation", key)
		}
	}
}
