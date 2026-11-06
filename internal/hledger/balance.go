package hledger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

// Cells in the hledger JSON shapes konto reads.
const (
	digitGroupCells = 2 // [separator, sizes]
	balanceRowCells = 4 // [name, display name, indent, amounts]
)

// jsonAmount is the part of an hledger amount konto reads. Quantities come as
// mantissa and places, never the lossy floatingPoint (spec §4.1).
type jsonAmount struct {
	Commodity string `json:"acommodity"`
	Quantity  struct {
		Mantissa int64 `json:"decimalMantissa"`
		Places   int   `json:"decimalPlaces"`
	} `json:"aquantity"`
	Style struct {
		Side        string `json:"ascommodityside"`
		Spaced      bool   `json:"ascommodityspaced"`
		DecimalMark string `json:"asdecimalmark"`
		// DigitGroups is [separator, [group sizes]], or null when ungrouped.
		DigitGroups []json.RawMessage `json:"asdigitgroups"`
	} `json:"astyle"`
}

func (a jsonAmount) amount() (domain.Amount, error) {
	style := domain.AmountStyle{
		SymbolLeft:  a.Style.Side == "L",
		Spaced:      a.Style.Spaced,
		DecimalMark: a.Style.DecimalMark,
	}
	if len(a.Style.DigitGroups) == digitGroupCells {
		mark, size, err := digitGroups(a.Style.DigitGroups)
		if err != nil {
			return domain.Amount{}, err
		}
		style.GroupMark, style.GroupSize = mark, size
	}

	return domain.Amount{
		Commodity: a.Commodity, Mantissa: a.Quantity.Mantissa, Places: a.Quantity.Places, Style: style,
	}, nil
}

// digitGroups reads hledger's [separator, [group sizes]].
func digitGroups(cells []json.RawMessage) (mark string, size int, err error) {
	var sizes []int

	if err := json.Unmarshal(cells[0], &mark); err != nil {
		return "", 0, fmt.Errorf("reading digit groups: %w", err)
	}

	if err := json.Unmarshal(cells[1], &sizes); err != nil {
		return "", 0, fmt.Errorf("reading digit group sizes: %w", err)
	}

	if len(sizes) == 0 {
		return "", 0, errors.New("digit groups without a size")
	}

	return mark, sizes[0], nil
}

// balanceRow is one row of `hledger bal -O json`: [name, display name, indent, amounts].
type balanceRow struct {
	name    string
	amounts []jsonAmount
}

func (r *balanceRow) UnmarshalJSON(data []byte) error {
	var cells []json.RawMessage
	if err := json.Unmarshal(data, &cells); err != nil {
		return fmt.Errorf("reading balance row: %w", err)
	}

	if len(cells) != balanceRowCells {
		return fmt.Errorf("balance row has %d cells, want %d", len(cells), balanceRowCells)
	}

	if err := json.Unmarshal(cells[0], &r.name); err != nil {
		return fmt.Errorf("reading balance row name: %w", err)
	}

	if err := json.Unmarshal(cells[balanceRowCells-1], &r.amounts); err != nil {
		return fmt.Errorf("reading balance row amounts: %w", err)
	}

	return nil
}

// Balances returns every account's current balance, empty ones included, as a flat list.
func (c *Client) Balances(ctx context.Context) ([]domain.Account, error) {
	return c.flatBalances(ctx, "-E")
}

// MonthlyActivity returns the change in each income and expense account, month by month, from the month
// of first through last (inclusive): one entry per calendar month, accounts without activity left out.
func (c *Client) MonthlyActivity(ctx context.Context, first, last time.Time) ([][]domain.Account, error) {
	incomeAndExpenses := under(domain.ExpensesPrefix, domain.IncomePrefix)
	monthly := slices.Concat([]string{"-M"}, through(first, last).flags(), incomeAndExpenses)

	out, err := c.query(ctx, flatReport(monthly...)...)
	if err != nil {
		return nil, err
	}

	var report multiPeriodReport
	if err := json.Unmarshal(out, &report); err != nil {
		return nil, fmt.Errorf("reading hledger bal output: %w", err)
	}

	return report.months()
}

// multiPeriodReport is the part of `hledger bal -M -O json` konto reads.
type multiPeriodReport struct {
	Dates [][]struct {
		Contents string `json:"contents"`
	} `json:"prDates"`
	Rows []struct {
		Name    string         `json:"prrName"`
		Amounts [][]jsonAmount `json:"prrAmounts"`
	} `json:"prRows"`
}

// months splits the report's rows into the accounts active in each month's column.
func (r multiPeriodReport) months() ([][]domain.Account, error) {
	months := make([][]domain.Account, len(r.Dates))
	for _, row := range r.Rows {
		if len(row.Amounts) != len(months) {
			return nil, fmt.Errorf("hledger bal row %s has %d months, want %d", row.Name, len(row.Amounts), len(months))
		}
		for month, cell := range row.Amounts {
			if len(cell) == 0 {
				continue
			}

			balance, err := amounts(cell)
			if err != nil {
				return nil, fmt.Errorf("reading hledger bal output: %w", err)
			}
			months[month] = append(months[month], domain.Account{Name: row.Name, Balance: balance})
		}
	}

	return months, nil
}

// amounts reads one cell of hledger amounts.
func amounts(cell []jsonAmount) (domain.MultiAmount, error) {
	balance := make(domain.MultiAmount, len(cell))
	for i, raw := range cell {
		var err error
		if balance[i], err = raw.amount(); err != nil {
			return nil, err
		}
	}

	return balance, nil
}

// Activity returns the change in the accounts under the prefixes (all when there are none) from
// first through last (inclusive).
func (c *Client) Activity(ctx context.Context, first, last time.Time, prefixes ...string) ([]domain.Account, error) {
	return c.flatBalances(ctx, slices.Concat(through(first, last).flags(), []string{notCarry}, under(prefixes...))...)
}

// notCarry leaves the carry out: bringing last year's balance into 1 January is no change of the period.
const notCarry = "not:tag:" + domain.KontoTag + "=" + domain.CarryValue

// BalancesUntil returns the balances of the accounts under the prefixes, counting everything
// dated up to and including last; zero balances are left out.
func (c *Client) BalancesUntil(ctx context.Context, last time.Time, prefixes ...string) ([]domain.Account, error) {
	return c.flatBalances(ctx, slices.Concat(endingBefore(dayAfter(last)), under(prefixes...))...)
}

// under is the hledger query for the accounts under each of the name prefixes.
func under(prefixes ...string) []string {
	queries := make([]string, len(prefixes))
	for i, prefix := range prefixes {
		queries[i] = "^" + regexp.QuoteMeta(prefix)
	}

	return queries
}

// exactly is the hledger query for the one account and none under it.
func exactly(account string) string { return "acct:^" + regexp.QuoteMeta(account) + "$" }

// flatBalances runs `hledger bal` as a flat JSON report with the given extra arguments.
func (c *Client) flatBalances(ctx context.Context, extra ...string) ([]domain.Account, error) {
	out, err := c.query(ctx, flatReport(extra...)...)
	if err != nil {
		return nil, err
	}

	return readFlatBalances(out)
}

// TextBalances returns the asset and liability balances of journal text given on standard input, counting
// every transaction dated before the day; zero balances are left out. Nothing on disk is read.
func (c *Client) TextBalances(ctx context.Context, text string, before time.Time) ([]domain.Account, error) {
	report := flatReport(slices.Concat(endingBefore(before), under(domain.AssetsPrefix, domain.LiabilitiesPrefix))...)

	out, err := c.queryText(ctx, text, report...)
	if err != nil {
		return nil, err
	}

	return readFlatBalances(out)
}

// flatReport is the arguments of a flat JSON `hledger bal` with the given extra arguments.
func flatReport(extra ...string) []string {
	return append([]string{"bal", "--flat", "-O", "json"}, extra...)
}

func readFlatBalances(out []byte) ([]domain.Account, error) {
	var report []json.RawMessage // [rows, totals]
	if err := json.Unmarshal(out, &report); err != nil {
		return nil, fmt.Errorf("reading hledger bal output: %w", err)
	}

	if len(report) == 0 {
		return nil, errors.New("reading hledger bal output: the report is empty")
	}

	var rows []balanceRow
	if err := json.Unmarshal(report[0], &rows); err != nil {
		return nil, fmt.Errorf("reading hledger bal output: %w", err)
	}

	return accountsOf(rows)
}

// accountsOf is each balance row as an account with its balance.
func accountsOf(rows []balanceRow) ([]domain.Account, error) {
	accounts := make([]domain.Account, len(rows))
	for i, row := range rows {
		balance, err := amounts(row.amounts)
		if err != nil {
			return nil, fmt.Errorf("reading hledger bal output: %w", err)
		}
		accounts[i] = domain.Account{Name: row.name, Balance: balance}
	}

	return accounts, nil
}

// span is a run of days as hledger reads its -b/-e and --forecast=begin..end: begin included, end not.
// Client methods name an included last day last, and an excluded end until (or before).
type span struct{ begin, end time.Time }

// through is the span from first through last, both included.
func through(first, last time.Time) span { return span{first, dayAfter(last)} }

// endingBefore is the hledger flag that counts only what is dated before date.
func endingBefore(date time.Time) []string { return []string{"-e", day(date)} }

func (s span) flags() []string { return []string{"-b", day(s.begin), "-e", day(s.end)} }

func (s span) forecast() string { return "--forecast=" + day(s.begin) + ".." + day(s.end) }

func dayAfter(last time.Time) time.Time { return last.AddDate(0, 0, 1) }

func day(date time.Time) string { return date.Format(time.DateOnly) }
