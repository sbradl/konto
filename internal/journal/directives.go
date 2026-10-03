package journal

import (
	"strings"

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
