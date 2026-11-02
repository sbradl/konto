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

// directiveLine is a `KEYWORD NAME  ; comment` line split up: the head is all before the comment, trailing
// space aside, and the name its first part, which two spaces end.
type directiveLine struct{ head, name, comment string }

// parseDirective splits a line of the keyword's directive; false for any other line.
func parseDirective(line, keyword string) (directiveLine, bool) {
	rest, found := directiveArgs(line, keyword)
	if !found {
		return directiveLine{}, false
	}
	head, comment, _ := strings.Cut(rest, ";")
	head = strings.TrimRight(head, blanks)
	return directiveLine{head: head, name: accountField(head), comment: comment}, true
}

// parseDeclaration reads `account NAME  ; tag, tag` into a Declaration.
func parseDeclaration(line string) (domain.Declaration, bool) {
	directive, found := parseDirective(line, domain.DirectiveAccount)
	if !found {
		return domain.Declaration{}, false
	}
	declaration := domain.Declaration{Name: directive.name}
	for tag := range strings.SplitSeq(directive.comment, ",") {
		declaration = declaration.WithTag(strings.TrimSpace(tag))
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
	rest, found := directiveArgs(line, "commodity")
	if !found {
		return "", false
	}
	rest, _, _ = strings.Cut(rest, ";")
	if _, quoted, found := strings.Cut(rest, `"`); found {
		symbol, _, _ := strings.Cut(quoted, `"`)
		return symbol, symbol != ""
	}
	// an unquoted symbol has no digits, spaces or marks (hledger wants any that has them quoted), so it is
	// what the sample amount leaves
	symbol := strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) || strings.ContainsRune(".,-", r) {
			return -1
		}
		return r
	}, rest))
	return symbol, symbol != ""
}
