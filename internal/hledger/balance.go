package hledger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
	out, err := c.query(ctx, "bal", "--flat", "-E", "-O", "json")
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
