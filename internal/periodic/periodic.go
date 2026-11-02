package periodic

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

// ErrNotEditable is the refusal to edit a rule the form cannot show.
var ErrNotEditable = domain.RefusalError{Reason: "the rule does not fit the form — edit the journal by hand"}

// Input is what the form holds: every field as it was typed or chosen.
type Input struct {
	Date        string // the next due date, 2026-10-15
	Frequency   Frequency
	Remaining   string // the payments left; empty for no end, unused for a once rule
	Description string
	Amount      string // a positive number in the journal's style
	Currency    string
	From, To    string // the account the money comes from, and the one it goes to
}

type gateway interface {
	Occurrences(ctx context.Context, text string, begin, until time.Time) ([]time.Time, error)
}

// Service plans the writes of the Scheduled form.
type Service struct {
	gw    gateway
	today func() time.Time
}

// NewService returns a Service that asks gw which dates a rule gives rise to, with today read from the clock today.
func NewService(gw gateway, today func() time.Time) *Service { return &Service{gw: gw, today: today} }

// Add plans a new rule, appended to the main journal (ADR 0013: rules go to main.journal).
func (s *Service) Add(ctx context.Context, input Input, ledger domain.Ledger) (domain.Changeset, error) {
	lines, err := s.block(ctx, input, ledger)
	if err != nil {
		return domain.Changeset{}, err
	}
	return domain.Changeset{Edits: []domain.Edit{domain.AddRule{Lines: lines}}}, nil
}

// Replace plans the new text of an existing rule, in place.
func (s *Service) Replace(
	ctx context.Context, input Input, rule domain.PeriodicRule, ledger domain.Ledger,
) (domain.Changeset, error) {
	lines, err := s.block(ctx, input, ledger)
	if err != nil {
		return domain.Changeset{}, err
	}
	return domain.Changeset{Edits: []domain.Edit{domain.ReplaceRule{Pos: rule.Pos, Lines: lines}}}, nil
}

// Form is what the form shows for editing the rule: its next occurrence as hledger projects it, and the
// payments left when it has an end. A rule the form cannot show is refused.
func (s *Service) Form(ctx context.Context, rule domain.PeriodicRule, ledger domain.Ledger) (Input, error) {
	read, ok := parseRule(rule.Lines)
	if !ok {
		return Input{}, ErrNotEditable
	}
	for _, schedule := range ledger.Schedules() {
		if next, found := schedule.Next(); found && schedule.Rule.Pos == rule.Pos {
			read.input.Date = next.Format(time.DateOnly)
		}
	}
	if !read.until.IsZero() {
		dates, err := s.gw.Occurrences(ctx, rule.Entry()+"\n", s.today(), read.until)
		if err != nil {
			return Input{}, err //nolint:wrapcheck // the gateway's error names what failed
		}
		read.input.Remaining = strconv.Itoa(len(dates))
	}
	return read.input, nil
}

// entered is the input once it has been checked and read.
type entered struct {
	date      time.Time
	remaining int
	amount    domain.Amount
}

// missingField is the first field that is empty, or "" when none is.
func (in Input) missingField() string {
	for _, field := range []struct{ name, value string }{
		{"next due date", in.Date},
		{"description", in.Description},
		{"amount", in.Amount},
		{"currency", in.Currency},
		{"from account", in.From},
		{"to account", in.To},
	} {
		if field.value == "" {
			return field.name
		}
	}
	return ""
}

// check reads the input, or refuses it by the first field that is missing or wrong.
func (in Input) check(ledger domain.Ledger) (entered, error) {
	var read entered
	if field := in.missingField(); field != "" {
		return read, domain.RefusalError{Reason: field + " is required"}
	}
	if in.From == in.To {
		return read, domain.RefusalError{Reason: "from account and to account must differ"}
	}
	var err error
	if read.date, err = time.Parse(time.DateOnly, in.Date); err != nil {
		return read, domain.RefusalError{Reason: "next due date must be a date like 2026-10-15"}
	}
	if read.amount, err = parseAmount(ledger.AmountLike(in.Currency), in.Amount); err != nil {
		return read, err
	}
	if in.Frequency != Once && in.Remaining != "" {
		if read.remaining, err = strconv.Atoi(in.Remaining); err != nil || read.remaining < 1 {
			return read, domain.RefusalError{Reason: "remaining payments must be a positive whole number"}
		}
	}
	return read, nil
}

// parseAmount reads a positive amount in the style of the commodity.
func parseAmount(like domain.Amount, text string) (domain.Amount, error) {
	mantissa, err := domain.ParseNumber(text, like.Style.DecimalMark, like.Style.GroupMark, like.Places)
	if err != nil || mantissa <= 0 {
		return domain.Amount{}, domain.RefusalError{Reason: "amount must be a positive number"}
	}
	like.Mantissa = mantissa
	return like, nil
}

// block is the `~` block of the input: the `~` line and the two postings, the amount on the account that
// receives it. A count of remaining payments becomes an exclusive end date, the day after the last of them
// as hledger projects the rule.
func (s *Service) block(ctx context.Context, input Input, ledger domain.Ledger) ([]string, error) {
	read, err := input.check(ledger)
	if err != nil {
		return nil, err
	}
	postings := []string{
		domain.NewPosting{Account: input.To, Amount: domain.MultiAmount{read.amount}}.Line(),
		domain.NewPosting{Account: input.From}.Line(),
	}
	header := input.header(read.date, time.Time{})
	if read.remaining > 0 {
		until, err := s.endAfter(ctx, append([]string{header}, postings...), read)
		if err != nil {
			return nil, err
		}
		header = input.header(read.date, until)
	}
	return append([]string{header}, postings...), nil
}

// header is the `~` line; until is the exclusive end date, zero for none.
func (in Input) header(date, until time.Time) string {
	if in.Frequency == Once {
		return "~ " + date.Format(time.DateOnly) + "  " + in.Description
	}
	line := "~ " + in.Frequency.expression() + " from " + date.Format(time.DateOnly)
	if !until.IsZero() {
		line += " to " + until.Format(time.DateOnly)
	}
	return line + "  " + in.Description
}

// endAfter is the day after the last of the remaining payments, found by asking hledger for the occurrences of
// the rule without an end.
func (s *Service) endAfter(ctx context.Context, rule []string, read entered) (time.Time, error) {
	// yearly, the sparsest frequency, needs a year per payment; one more keeps the last inside the exclusive end
	horizon := read.date.AddDate(read.remaining+1, 0, 0)
	dates, err := s.gw.Occurrences(ctx, strings.Join(rule, "\n")+"\n", read.date, horizon)
	if err != nil {
		return time.Time{}, err //nolint:wrapcheck // the gateway's error names what failed
	}
	if len(dates) < read.remaining {
		return time.Time{}, domain.RefusalError{Reason: "the rule does not have that many payments"}
	}
	return dates[read.remaining-1].AddDate(0, 0, 1), nil
}
