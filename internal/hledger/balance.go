package hledger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// PeriodActivity returns the change in each income and expense account from first through last (inclusive).
func (c *Client) PeriodActivity(ctx context.Context, first, last time.Time) ([]domain.Account, error) {
	return c.Activity(ctx, first, last, "^expenses:", "^income:")
}

// MonthlyActivity returns the change in each income and expense account, month by month, from the month
// of first through last (inclusive): one entry per calendar month, accounts without activity left out.
func (c *Client) MonthlyActivity(ctx context.Context, first, last time.Time) ([][]domain.Account, error) {
	out, err := c.query(ctx, "bal", "--flat", "-M", "-O", "json",
		"-b", first.Format(time.DateOnly), "-e", last.AddDate(0, 0, 1).Format(time.DateOnly), "^expenses:", "^income:")
	if err != nil {
		return nil, err
	}
	var report multiPeriodReport
	if err := json.Unmarshal(out, &report); err != nil {
		return nil, fmt.Errorf("reading hledger bal output: %w", err)
	}
	return report.months()
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
				return nil, err
			}
			months[month] = append(months[month], domain.Account{Name: row.Name, Balance: balance})
		}
	}
	return months, nil
}

func amounts(cell []jsonAmount) (domain.MultiAmount, error) {
	balance := make(domain.MultiAmount, len(cell))
	for i, raw := range cell {
		var err error
		if balance[i], err = raw.amount(); err != nil {
			return nil, fmt.Errorf("reading hledger bal output: %w", err)
		}
	}
	return balance, nil
}

// Activity returns the change in the accounts the queries match (all when there are none) from
// first through last (inclusive).
func (c *Client) Activity(ctx context.Context, first, last time.Time, queries ...string) ([]domain.Account, error) {
	return c.flatBalances(ctx, append([]string{
		"-b", first.Format(time.DateOnly), "-e", last.AddDate(0, 0, 1).Format(time.DateOnly),
	}, queries...)...)
}

// BalancesUntil returns the balances of the accounts the queries match, counting everything
// dated up to and including last; zero balances are left out.
func (c *Client) BalancesUntil(ctx context.Context, last time.Time, queries ...string) ([]domain.Account, error) {
	return c.flatBalances(ctx, append([]string{"-e", last.AddDate(0, 0, 1).Format(time.DateOnly)}, queries...)...)
}

// flatBalances runs `hledger bal` as a flat JSON report with the given extra arguments.
func (c *Client) flatBalances(ctx context.Context, extra ...string) ([]domain.Account, error) {
	out, err := c.query(ctx, append([]string{"bal", "--flat", "-O", "json"}, extra...)...)
	if err != nil {
		return nil, err
	}
	var report []json.RawMessage // [rows, totals]
	if err := json.Unmarshal(out, &report); err != nil || len(report) == 0 {
		return nil, fmt.Errorf("reading hledger bal output: %w", err)
	}
	var rows []balanceRow
	if err := json.Unmarshal(report[0], &rows); err != nil {
		return nil, fmt.Errorf("reading hledger bal output: %w", err)
	}
	accounts := make([]domain.Account, len(rows))
	for i, row := range rows {
		balance := make(domain.MultiAmount, len(row.amounts))
		for j, raw := range row.amounts {
			if balance[j], err = raw.amount(); err != nil {
				return nil, fmt.Errorf("reading hledger bal output: %w", err)
			}
		}
		accounts[i] = domain.Account{Name: row.name, Balance: balance}
	}
	return accounts, nil
}
