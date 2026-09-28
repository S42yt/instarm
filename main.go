package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"
)

const (
	likesURL    = "https://www.instagram.com/your_activity/interactions/likes"
	commentsURL = "https://www.instagram.com/your_activity/interactions/comments"
)

type Config struct {
	Mode     string
	Headless bool
	Batch    int
	DryRun   bool
	Profile  string
}

func main() {
	cfg := Config{}

	flag.StringVar(&cfg.Mode, "mode", "likes", "likes, comments, or all")
	flag.BoolVar(&cfg.Headless, "headless", true, "run browser headlessly")
	flag.IntVar(&cfg.Batch, "batch", 20, "maximum items per batch")
	flag.BoolVar(&cfg.DryRun, "dry-run", false, "select items without performing the action")
	flag.StringVar(&cfg.Profile, "profile", "data/browser", "persistent browser profile")
	flag.Parse()

	if cfg.Batch < 1 {
		log.Fatal("batch must be at least 1")
	}

	switch cfg.Mode {
	case "likes", "comments", "all":
	default:
		log.Fatal("mode must be likes, comments, or all")
	}

	if err := os.MkdirAll(cfg.Profile, 0700); err != nil {
		log.Fatalf("create browser profile: %v", err)
	}

	pw, err := playwright.Run()
	if err != nil {
		log.Fatalf("start playwright: %v", err)
	}
	defer pw.Stop()

	context, err := pw.Chromium.LaunchPersistentContext(
		cfg.Profile,
		playwright.BrowserTypeLaunchPersistentContextOptions{
			Headless: playwright.Bool(cfg.Headless),
			Viewport: &playwright.Size{
				Width:  1280,
				Height: 900,
			},
		},
	)
	if err != nil {
		log.Fatalf("launch chromium: %v", err)
	}
	defer context.Close()

	page, err := context.NewPage()
	if err != nil {
		log.Fatalf("create page: %v", err)
	}

	page.SetDefaultTimeout(7000)

	modes := []string{cfg.Mode}
	if cfg.Mode == "all" {
		modes = []string{"likes", "comments"}
	}

	for _, mode := range modes {
		if err := process(page, mode, cfg); err != nil {
			log.Printf("%s: %v", mode, err)
		}
	}
}

func process(page playwright.Page, mode string, cfg Config) error {
	target := likesURL
	action := "Unlike"
	empty := "You haven't liked anything"

	if mode == "comments" {
		target = commentsURL
		action = "Delete"
		empty = "You haven't commented on anything"
	}

	log.Printf("%s: opening activity page...", mode)

	if _, err := page.Goto(target, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return fmt.Errorf("open page: %w", err)
	}

	if err := loginCheck(page, cfg.Headless); err != nil {
		return err
	}

	dismissDialogs(page)

	for {
		if emptyState(page, empty) {
			log.Printf("%s: no more items", mode)
			return nil
		}

		if err := clickText(page, "Select"); err != nil {
			log.Printf("%s: Select button not found yet; scrolling...", mode)
			scroll(page)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		count, err := selectBatch(page, cfg.Batch)
		if err != nil {
			return fmt.Errorf("select items: %w", err)
		}

		if count == 0 {
			log.Printf("%s: no selectable items found; scrolling...", mode)
			scroll(page)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		log.Printf("%s: selected %d item(s)", mode, count)

		if cfg.DryRun {
			log.Printf(
				"%s: dry-run enabled; stopping before %s",
				mode,
				action,
			)
			return nil
		}

		if err := clickConfirmation(page, action); err != nil {
			return fmt.Errorf("click %s: %w", action, err)
		}

		time.Sleep(400 * time.Millisecond)

		if err := clickConfirmation(page, action); err != nil {
			log.Printf("%s: no second confirmation needed", mode)
		}

		time.Sleep(800 * time.Millisecond)
	}
}

func loginCheck(page playwright.Page, headless bool) error {
	if !strings.Contains(page.URL(), "/accounts/login") {
		return nil
	}

	if headless {
		return fmt.Errorf(
			"not logged in; run once with -headless=false",
		)
	}

	log.Println(
		"Instagram login required. Log in normally in the browser...",
	)

	deadline := time.Now().Add(3 * time.Minute)

	for time.Now().Before(deadline) {
		if !strings.Contains(page.URL(), "/accounts/login") {
			log.Println("login detected")
			return nil
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("login timeout")
}

func dismissDialogs(page playwright.Page) {
	for _, text := range []string{"Not now", "Not Now"} {
		loc := page.GetByText(
			text,
			playwright.PageGetByTextOptions{
				Exact: playwright.Bool(true),
			},
		)

		count, err := loc.Count()
		if err != nil || count == 0 {
			continue
		}

		_ = loc.First().Click(
			playwright.LocatorClickOptions{
				Timeout: playwright.Float(1500),
			},
		)
	}
}

func clickText(page playwright.Page, text string) error {
	loc := page.GetByText(
		text,
		playwright.PageGetByTextOptions{
			Exact: playwright.Bool(true),
		},
	)

	count, err := loc.Count()
	if err != nil {
		return err
	}

	if count == 0 {
		return fmt.Errorf("text %q not found", text)
	}

	return loc.Last().Click()
}

func selectBatch(page playwright.Page, batch int) (int, error) {
	selectors := []string{
		"[aria-label='Toggle checkbox']",
		"[role='checkbox']",
		"input[type='checkbox']",
	}

	selected := 0

	for _, selector := range selectors {
		loc := page.Locator(selector)

		count, err := loc.Count()
		if err != nil {
			continue
		}

		for i := 0; i < count && selected < batch; i++ {
			item := loc.Nth(i)

			visible, err := item.IsVisible()
			if err != nil || !visible {
				continue
			}

			checked, err := item.GetAttribute("aria-checked")
			if err == nil && checked == "true" {
				continue
			}

			if err := item.Click(); err != nil {
				continue
			}

			selected++
		}

		if selected >= batch {
			break
		}
	}

	return selected, nil
}

func clickConfirmation(page playwright.Page, action string) error {
	dialog := page.Locator("[role='dialog']")

	dialogCount, _ := dialog.Count()

	if dialogCount > 0 {
		button := dialog.GetByText(
			action,
			playwright.LocatorGetByTextOptions{
				Exact: playwright.Bool(true),
			},
		)

		buttonCount, _ := button.Count()

		if buttonCount > 0 {
			return button.Last().Click()
		}
	}

	return clickText(page, action)
}

func emptyState(page playwright.Page, text string) bool {
	loc := page.GetByText(
		text,
		playwright.PageGetByTextOptions{
			Exact: playwright.Bool(true),
		},
	)

	count, err := loc.Count()

	return err == nil && count > 0
}

func scroll(page playwright.Page) {
	_, _ = page.Evaluate(
		"window.scrollTo(0, document.body.scrollHeight)",
	)
}
