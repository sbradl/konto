//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/cucumber/godog"
)

// writeKinds maps each kind of write to doing it the way a user would.
var writeKinds = map[string]func(w *world) error{
	"import with new rules and a new category": func(w *world) error {
		return w.importCreatingCategory("sept.csv", "expenses:cafe", "Kaffeehaus")
	},
	"import into a new account": (*world).importIntoNewAccount,
	"close an account": func(w *world) error {
		return w.afterWrite(func() error { return w.selectAndPress("assets:spare", "c", "y") })
	},
	"reopen an account": func(w *world) error {
		return w.afterWrite(func() error { return w.selectAndPress("assets:old-account", "o", "y") })
	},
	"new periodic rule":    (*world).newPeriodicRule,
	"edited periodic rule": (*world).editPeriodicRule,
	"deleted periodic rule": func(w *world) error {
		return w.afterWrite(func() error { return w.deleteRuleInScheduled("Streaming") })
	},
}

// performWrite does one kind of write and waits until the journal has been read again.
func (w *world) performWrite(kind string) error {
	write, ok := writeKinds[kind]
	if !ok {
		return fmt.Errorf("unknown write %q", kind)
	}
	return write(w)
}

func (w *world) importIntoNewAccount() error {
	if err := w.writeExport("new.csv", newAccountExport()); err != nil {
		return err
	}
	if err := w.openInImport("new.csv"); err != nil {
		return err
	}
	return w.createAndImport("assets:holiday", "")
}

func (w *world) newPeriodicRule() error {
	return w.afterWrite(func() error {
		if err := w.press("3", "n"); err != nil {
			return err
		}
		if err := w.fillForm(validForm()); err != nil {
			return err
		}
		return w.press("Ctrl+S")
	})
}

func (w *world) editPeriodicRule() error {
	return w.afterWrite(func() error {
		if err := w.press("3"); err != nil {
			return err
		}
		if err := w.selectRule("Acme GmbH"); err != nil {
			return err
		}
		if err := w.press("e"); err != nil {
			return err
		}
		if err := w.changeAmount("3.100,00"); err != nil {
			return err
		}
		return w.press("Ctrl+S")
	})
}

// selectAndPress selects the account on the dashboard and presses the keys.
func (w *world) selectAndPress(account string, keys ...string) error {
	if err := w.press("1"); err != nil {
		return err
	}
	if err := w.selectAccount(account); err != nil {
		return err
	}
	return w.press(keys...)
}

// afterWrite runs the write and then waits for the journal to be read again.
func (w *world) afterWrite(write func() error) error {
	if err := write(); err != nil {
		return err
	}
	return w.eventually(func() error {
		line, err := w.statusLine()
		if err == nil && strings.Contains(line, "⣾") {
			err = errors.New("still reloading")
		}
		return err
	})
}

func newAccountExport() postbankExport {
	return postbankExport{
		IBAN: "DE27100777770209299700", ClosingBalance: "10,00",
		Rows: []postbankRow{{Date: "29.9.2026", Type: "Gutschrift", Payee: "Oma", Purpose: "Geld", Amount: "10,00"}},
	}
}

func (w *world) performEveryWrite() error {
	for _, kind := range []string{
		"import with new rules and a new category", "import into a new account", "close an account",
		"reopen an account", "new periodic rule", "edited periodic rule", "deleted periodic rule",
	} {
		if err := w.performWrite(kind); err != nil {
			return fmt.Errorf("%s: %w", kind, err)
		}
	}
	return nil
}

func (w *world) expectChecksAllClear() error {
	if err := w.press("!"); err != nil {
		return err
	}
	return w.expectScreen(true, "✓ all")
}

func (w *world) closeAndReopen(account string) error {
	if err := w.selectAndPress(account, "c", "y"); err != nil {
		return err
	}
	return w.afterWrite(func() error { return w.selectAndPress(account, "o", "y") })
}

func (w *world) expectOneSourceTag(account string) error {
	return w.eventually(func() error {
		line, err := w.directiveLine(account)
		if err != nil {
			return err
		}
		if n := strings.Count(line, "konto:source:"); n != 1 {
			return fmt.Errorf("%q carries %d source tags", line, n)
		}
		return nil
	})
}

var accountLine = regexp.MustCompile(`^\s+(\S+)`)

// expectCounterparts checks that no posting of an imported transaction lies in a catch-all account.
func (w *world) expectNoCatchAll(names ...string) error {
	return w.eventually(func() error {
		text := strings.ToLower(allJournalText(w.journalFiles()))
		for _, name := range names {
			if strings.Contains(text, "account "+name) || strings.Contains(text, "    "+name) {
				return fmt.Errorf("the journal has an account named %q", name)
			}
		}
		return nil
	})
}

func (w *world) expectCounterpartsAreAccounts() error {
	return w.eventually(func() error {
		for _, block := range w.newBlocks(false) {
			for _, line := range strings.Split(block, "\n")[1:] {
				match := accountLine.FindStringSubmatch(line)
				if len(match) < 2 || !strings.Contains(match[1], ":") {
					return fmt.Errorf("a posting without an account: %q", line)
				}
			}
		}
		return nil
	})
}

// visitEveryView visits every view and presses the read-only keys on it.
func (w *world) visitEveryView() error {
	for _, view := range []string{"1", "2", "3", "4", "5"} {
		keys := []string{view, "Down", "Up", "End", "Home", "PgDn", "PgUp", "?", "Esc", "!", "Esc", "r"}
		if view == "5" {
			keys = append(keys, "p")
		}
		if err := w.press(keys...); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) expectTransactionCount(count string) error { return w.expectTabCount(count) }

func (w *world) expectSkippedRow(text string) error {
	return w.expectPanel(skippedTitle, func(rows []recentRow) error {
		for _, row := range rows {
			if strings.Join(strings.Fields(row.text), " ") == text {
				return nil
			}
		}
		return fmt.Errorf("the skipped list lacks %q", text)
	})
}

// startAgain starts konto anew after the previous one has gone.
func (w *world) startAgain() error {
	w.s = nil
	return w.start()
}

// tryWriteImport categorises every row and confirms the write without waiting for it to succeed.
func (w *world) tryWriteImport() error {
	if err := w.openUnlessStaged("sept.csv"); err != nil {
		return err
	}
	if err := w.categoriseEveryRow(); err != nil {
		return err
	}
	return w.press("w", "y")
}

func (w *world) expectNoCategoryRows() error {
	return w.eventually(func() error {
		rows, err := w.accountsPanel()
		for _, row := range rows {
			if name := row.name(); strings.HasPrefix(name, "expenses:") || strings.HasPrefix(name, "income:") {
				return fmt.Errorf("the Accounts panel lists the category %q", name)
			}
		}
		return err
	})
}

func (w *world) expectLockFile() error {
	return w.eventually(func() error {
		if _, err := os.Stat(w.journal + ".lock"); err != nil {
			return fmt.Errorf("no lock file next to the journal: %w", err)
		}
		return nil
	})
}

// signalKonto sends the signal to the running konto and waits for it to exit.
func (w *world) signalKonto(signal syscall.Signal) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	w.waitLoaded(s)
	if err := s.cmd.Process.Signal(signal); err != nil {
		return err
	}
	if _, err := s.waitExit(w.suite.timeout); err != nil && signal != syscall.SIGKILL {
		return err
	}
	return nil
}

func (w *world) expectTerminalRestored() error {
	s, err := w.session()
	if err != nil {
		return err
	}
	if _, err := s.waitExit(w.suite.timeout); err != nil {
		return err
	}
	raw := string(s.rawOutput())
	if !strings.Contains(raw, "\x1b[?1049l") || !strings.Contains(raw, "\x1b[?25h") {
		return errors.New("konto left the alternate screen or hid the cursor for good")
	}
	return nil
}

func (w *world) expectNoSockets() error {
	s, err := w.session()
	if err != nil {
		return err
	}
	dir := fmt.Sprintf("/proc/%d/fd", s.cmd.Process.Pid)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if target, err := os.Readlink(filepath.Join(dir, entry.Name())); err == nil && strings.HasPrefix(target, "socket:") {
			return fmt.Errorf("konto holds a socket: %s", target)
		}
	}
	return nil
}

// treeFiles lists the files below the directory.
func treeFiles(root string) []string {
	var files []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func (w *world) expectNoStateInHome() error {
	for _, dir := range []string{".config", ".local", ".cache"} {
		if files := treeFiles(filepath.Join(w.home, dir)); len(files) > 0 {
			return fmt.Errorf("files were created in %s: %v", dir, files)
		}
	}
	return nil
}

func (w *world) useJournalAt(name, path string) error {
	if err := w.useJournal(name, ""); err != nil {
		return err
	}
	target := w.expand(path)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	content, err := os.ReadFile(w.journal)
	if err != nil {
		return err
	}
	if err := os.WriteFile(target, content, 0o644); err != nil {
		return err
	}
	w.env["LEDGER_FILE"] = target
	w.journal = target
	return nil
}

func (w *world) useReadOnly(name string) error {
	if err := w.useJournal(name, ""); err != nil {
		return err
	}
	return os.Chmod(w.journal, 0o444)
}

func (w *world) deleteJournal() error {
	w.waitLoaded(w.s)
	return os.Remove(w.journal)
}

func (w *world) expectReportsWithin(seconds int) error {
	started := time.Now()
	if err := w.press("5"); err != nil {
		return err
	}
	if err := w.expectScreen(true, "BALANCE SHEET"); err != nil {
		return err
	}
	if elapsed := time.Since(started); elapsed > time.Duration(seconds)*time.Second {
		return fmt.Errorf("the reports took %v", elapsed)
	}
	return nil
}

func (w *world) expectTabBarWithin(seconds int) error {
	started := time.Now()
	s, err := w.session()
	if err != nil {
		return err
	}
	if !s.waitFor(time.Duration(seconds)*time.Second, func(screen string) bool { return strings.Contains(screen, tabBarMarker) }) {
		return fmt.Errorf("no tab bar after %v", time.Since(started))
	}
	return nil
}

func registerInvariantSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^I perform the write "([^"]*)"$`, w.performWrite)
	step(sc, `^I perform every kind of write in sequence$`, w.performEveryWrite)
	step(sc, `^the checks panel shows "✓ all"$`, w.expectChecksAllClear)
	step(sc, `^I close and reopen "([^"]*)"$`, w.closeAndReopen)
	step(sc, `^the journal's "account ([^"]*)" directive has exactly one "konto:source:" tag$`, w.expectOneSourceTag)
	step(sc, `^I start konto again$`, w.startAgain)
	step(sc, `^every transaction has a counterpart that is a category, an account, or "equity:opening-balances"$`, w.expectCounterpartsAreAccounts)
	step(sc, `^the journal has no account named "([^"]*)" or "([^"]*)" or "([^"]*)"$`, func(a, b, c string) error {
		return w.expectNoCatchAll(a, b, c)
	})
	step(sc, `^I visit every view and press every read-only key$`, w.visitEveryView)
	step(sc, `^I visit every view$`, w.visitEveryView)
	step(sc, `^I visit every view and quit$`, func() error {
		if err := w.visitEveryView(); err != nil {
			return err
		}
		return w.press("q")
	})
	step(sc, `^the transaction count is still (\d+)$`, w.expectTransactionCount)
	step(sc, `^the skipped list contains the row "([^"]*)"$`, w.expectSkippedRow)
	step(sc, `^a lock file exists next to the journal$`, w.expectLockFile)
	step(sc, `^konto was killed with SIGKILL$`, func() error { return w.signalKonto(syscall.SIGKILL) })
	step(sc, `^konto receives SIGTERM$`, func() error { return w.signalKonto(syscall.SIGTERM) })
	step(sc, `^konto exits$`, func() error { _, err := w.s.waitExit(w.suite.timeout); return err })
	step(sc, `^I can start konto again$`, func() error {
		if err := w.startAgain(); err != nil {
			return err
		}
		return w.expectScreen(true, tabBarMarker)
	})
	step(sc, `^the terminal left the alternate screen$`, w.expectTerminalRestored)
	step(sc, `^the cursor is visible again$`, func() error { return nil }) // checked with the alternate screen
	step(sc, `^konto opened no network sockets$`, w.expectNoSockets)
	step(sc, `^no file was created in the user's config or state directories$`, w.expectNoStateInHome)
	step(sc, `^no file other than the lock file was created next to the journal$`, w.expectNoStateFiles)
	step(sc, `^I see the tab bar or the parse-error screen$`, func() error {
		return w.eventually(func() error {
			text := w.s.screenText()
			if !strings.Contains(text, tabBarMarker) && !strings.Contains(text, "Enter to jump to the editor") &&
				!strings.Contains(text, "No journal found") {
				return errors.New("neither the tab bar nor an error screen is shown")
			}
			return nil
		})
	})
	step(sc, `^the journal "([^"]*)" at "([^"]*)"$`, w.useJournalAt)
	step(sc, `^I start konto with "--file" pointing there$`, func() error { return w.start("--file", w.journal) })
	step(sc, `^I see the tab bar within (\d+) seconds$`, w.expectTabBarWithin)
	step(sc, `^pressing "5" shows reports within (\d+) seconds$`, w.expectReportsWithin)
	step(sc, `^the journal "([^"]*)" is read-only$`, w.useReadOnly)
	step(sc, `^I try to write the import$`, w.tryWriteImport)
	step(sc, `^the Accounts panel lists no "expenses:\*" or "income:\*" entries$`, w.expectNoCategoryRows)
	step(sc, `^I see an error naming the file$`, func() error { return w.expectLine(-1, "status bar", filepath.Base(w.journal), strings.Contains) })
	step(sc, `^the journal file is deleted externally$`, w.deleteJournal)
}
