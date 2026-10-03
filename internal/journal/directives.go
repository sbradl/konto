package journal

import (
	"slices"
	"strings"
	"unicode"

	"github.com/sbradl/konto/internal/domain"
)

// AccountDeclarations reads the `account` directives of the journal and the files it
// includes. It is a line reader, not a parser: it knows nothing of transactions.
func (f File) AccountDeclarations() ([]domain.Declaration, error) {
	sources, err := f.sources()
	if err != nil {
		return nil, err
	}
	var declarations []domain.Declaration
	for _, source := range sources {
		for _, line := range source.lines {
			if declaration, ok := parseDeclaration(line); ok {
				declarations = append(declarations, declaration)
			}
		}
	}
	return declarations, nil
}

// parseDeclaration reads `account NAME  ; tag, tag` into a Declaration.
func parseDeclaration(line string) (domain.Declaration, bool) {
	rest, found := strings.CutPrefix(line, "account ")
	if !found {
		return domain.Declaration{}, false
	}
	head, comment, _ := strings.Cut(rest, ";")
	name, _, _ := strings.Cut(strings.TrimSpace(head), "  ")
	declaration := domain.Declaration{Name: strings.TrimSpace(name)}
	for tag := range strings.SplitSeq(comment, ",") {
		declaration.ApplyTag(strings.TrimSpace(tag))
	}
	return declaration, true
}

// Commodities reads the `commodity` directives of the journal and the files it includes: the currencies it
// declares, in the order they are declared, each once.
func (f File) Commodities() ([]string, error) {
	sources, err := f.sources()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, source := range sources {
		for _, line := range source.lines {
			if name, ok := commodityName(line); ok && !slices.Contains(names, name) {
				names = append(names, name)
			}
		}
	}
	return names, nil
}

// commodityName is the commodity a `commodity` directive line declares, whether it gives a sample amount
// (`commodity 1.000,00 EUR`, `commodity $1,000.00`) or the bare symbol.
func commodityName(line string) (string, bool) {
	rest, found := strings.CutPrefix(line, "commodity ")
	if !found {
		return "", false
	}
	rest, _, _ = strings.Cut(rest, ";")
	symbol := strings.Trim(strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) || strings.ContainsRune(".,- ", r) {
			return -1
		}
		return r
	}, rest)), `"`)
	return symbol, symbol != ""
}
