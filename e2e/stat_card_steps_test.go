//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cucumber/godog"
)

// statCard is one stat card as drawn: its top edge, and the two rows under it.
type statCard struct {
	title string
	value string
	note  string
}

// statCards finds the cards on the dashboard: every titled panel except Accounts, on the first rows.
func (w *world) statCards() ([]statCard, error) {
	s, err := w.session()
	if err != nil {
		return nil, err
	}
	lines := s.screenLines()
	top := -1 // the row the panels start on, just under the tab bar and its rule
	for y, line := range lines {
		if strings.Contains(line, "╭─ ACCOUNTS ") {
			top = y
			break
		}
	}
	if top < 0 {
		return nil, errors.New("no dashboard on screen")
	}
	var cards []statCard
	for x := 0; x+len("╭─ ") < len([]rune(lines[top])); x++ {
		runes := []rune(lines[top])
		if string(runes[x:x+3]) != "╭─ " {
			continue
		}
		title, _, _ := strings.Cut(strings.TrimLeft(string(runes[x+3:]), " "), " ─")
		if title == "ACCOUNTS" {
			continue
		}
		cards = append(cards, statCard{
			title: title,
			value: cardCell(lines[top+1], x),
			note:  cardCell(lines[top+2], x),
		})
	}
	return cards, nil
}

// cardCell is the text inside the card whose left edge is column x of row.
func cardCell(row string, x int) string {
	runes := []rune(row)
	if x >= len(runes) {
		return ""
	}
	inside, _, _ := strings.Cut(string(runes[x+1:]), "│")
	return strings.TrimSpace(inside)
}

// expectCard waits for the card with the title and remembers it for the steps that follow.
func (w *world) expectCard(title string, check func(card statCard) error) error {
	var failure error
	s, err := w.session()
	if err != nil {
		return err
	}
	s.waitFor(w.suite.timeout, func(string) bool {
		cards, cerr := w.statCards()
		if cerr != nil {
			failure = cerr
			return false
		}
		failure = fmt.Errorf("no %q card on screen", title)
		for _, card := range cards {
			if card.title == title {
				w.focusedCard = card
				failure = check(card)
			}
		}
		return failure == nil
	})
	if failure != nil {
		return fmt.Errorf("%w\n%s", failure, s.screenText())
	}
	return nil
}

func registerStatCardSteps(sc *godog.ScenarioContext, w *world) {
	registerCardReadingSteps(sc, w)
	registerCardFixtureSteps(sc, w)
}

func registerCardReadingSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^I see a "([^"]*)" card showing "([^"]*)"$`, func(title, value string) error {
		return w.expectCard(title, func(card statCard) error {
			if card.value != value {
				return fmt.Errorf("the %q card shows %q, want %q", title, card.value, value)
			}
			return nil
		})
	})
	step(sc, `^the card note reads "([^"]*)"$`, func(note string) error {
		if w.focusedCard.note != note {
			return fmt.Errorf("the card note reads %q, want %q", w.focusedCard.note, note)
		}
		return nil
	})
	step(sc, `^I see exactly (\d+) stat cards$`, w.expectCardCount)
	step(sc, `^the "([^"]*)" figure does not include the 500,00 EUR transfer to savings$`, func(title string) error {
		return w.expectFigure(title, "1.815,00 EUR")
	})
	step(sc, `^the "([^"]*)" figure does not include any September transaction$`, func(title string) error {
		return w.expectFigure(title, "0,00 EUR")
	})
	step(sc, `^"([^"]*)" equals income minus expenses, not the change in asset accounts$`, func(title string) error {
		return w.expectFigure(title, "1.815,00 EUR")
	})
}

func registerCardFixtureSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^the row "([^"]*)" shows "([^"]*)"$`, func(account, text string) error {
		return w.expectAccounts(expectShows(account, text))
	})
	step(sc, `^the journal "([^"]*)" with a one-sided transfer leg of ([\d.,]+ EUR) on the clearing account$`,
		func(name, amount string) error {
			return w.useJournal(name, "\n2026-09-27 Sparen\n    assets:transfers  "+amount+"\n    assets:checking  -"+amount+"\n")
		})
	step(sc, `^the journal "([^"]*)" where closed account "([^"]*)" holds ([\d.,]+ EUR)$`,
		func(name, account, amount string) error {
			return w.useJournal(name, "\n2026-09-27 Stray\n    "+account+"  "+amount+"\n    equity:opening-balances\n")
		})
	step(sc, `^the journal "([^"]*)" where "([^"]*)" holds ([\d.,]+ PLN)$`, func(name, account, amount string) error {
		return w.useJournal(name, "\n2026-09-27 Zloty\n    "+account+"  "+amount+"\n    equity:opening-balances\n")
	})
	step(sc, `^I do not see the words "([^"]*)" or "([^"]*)"$`, func(a, b string) error {
		if err := w.expectScreen(false, a); err != nil {
			return err
		}
		return w.expectScreen(false, b)
	})
	step(sc, `^the "([^"]*)" row shows "([^"]*)" or "([^"]*)"$`, func(account, a, b string) error {
		return w.expectAccounts(func(rows []accountRow) error {
			if err := expectShows(account, a)(rows); err != nil {
				return expectShows(account, b)(rows)
			}
			return nil
		})
	})
}

func (w *world) expectFigure(title, value string) error {
	return w.expectCard(title, func(card statCard) error {
		if card.value != value {
			return fmt.Errorf("the %q card shows %q, want %q", title, card.value, value)
		}
		return nil
	})
}

func (w *world) expectCardCount(n int) error {
	var failure error
	s, err := w.session()
	if err != nil {
		return err
	}
	s.waitFor(w.suite.timeout, func(string) bool {
		cards, cerr := w.statCards()
		failure = cerr
		if cerr == nil && len(cards) != n {
			failure = fmt.Errorf("%d stat cards on screen, want %d", len(cards), n)
		}
		return failure == nil
	})
	return failure
}
