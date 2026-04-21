package cmd

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/yuluo-yx/typo/internal/config"
	"github.com/yuluo-yx/typo/internal/engine"
	itypes "github.com/yuluo-yx/typo/internal/types"
)

const (
	defaultStatsSinceDays = 30
	defaultStatsTop       = 10
)

var statsNow = time.Now

type statsOptions struct {
	sinceDays int
	top       int
}

type statsToolTotal struct {
	Name  string
	Count int
}

func cmdStats(args []string) int {
	options, ok := parseStatsOptions(args)
	if !ok {
		return 1
	}

	cfg := config.Load()
	history := engine.NewHistory(cfg.ConfigDir)
	entries := filterStatsEntries(history.List(), options.sinceDays, statsNow())
	if len(entries) == 0 {
		fmt.Printf("No accepted corrections in the last %d days.\n", options.sinceDays)
		return 0
	}

	sortStatsEntries(entries)
	topEntries := entries
	if len(topEntries) > options.top {
		topEntries = topEntries[:options.top]
	}

	printStatsSummary(topEntries, entries, options.sinceDays)
	return 0
}

func parseStatsOptions(args []string) (statsOptions, bool) {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	sinceDays := fs.Int("since", defaultStatsSinceDays, "show entries whose last accepted correction was within the last N days")
	top := fs.Int("top", defaultStatsTop, "show at most N typo pairs")

	if err := fs.Parse(args); err != nil {
		return statsOptions{}, false
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Error: stats does not accept positional arguments")
		return statsOptions{}, false
	}
	if *sinceDays <= 0 {
		fmt.Fprintln(os.Stderr, "Error: --since must be greater than 0")
		return statsOptions{}, false
	}
	if *top <= 0 {
		fmt.Fprintln(os.Stderr, "Error: --top must be greater than 0")
		return statsOptions{}, false
	}

	return statsOptions{
		sinceDays: *sinceDays,
		top:       *top,
	}, true
}

func filterStatsEntries(entries []itypes.HistoryEntry, sinceDays int, now time.Time) []itypes.HistoryEntry {
	cutoff := now.AddDate(0, 0, -sinceDays).Unix()
	filtered := make([]itypes.HistoryEntry, 0, len(entries))

	for _, entry := range entries {
		entry.From = strings.TrimSpace(entry.From)
		entry.To = strings.TrimSpace(entry.To)
		if entry.From == "" || entry.To == "" || entry.Count <= 0 {
			continue
		}
		if entry.Timestamp < cutoff {
			continue
		}
		filtered = append(filtered, entry)
	}

	return filtered
}

func sortStatsEntries(entries []itypes.HistoryEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Count == entries[j].Count {
			if entries[i].Timestamp == entries[j].Timestamp {
				return entries[i].From < entries[j].From
			}
			return entries[i].Timestamp > entries[j].Timestamp
		}
		return entries[i].Count > entries[j].Count
	})
}

func printStatsSummary(topEntries, allEntries []itypes.HistoryEntry, sinceDays int) {
	fmt.Printf("Top typos (last %d days):\n", sinceDays)

	labels := make([]string, 0, len(topEntries))
	width := 0
	for _, entry := range topEntries {
		label := fmt.Sprintf("%s -> %s", entry.From, entry.To)
		labels = append(labels, label)
		if len(label) > width {
			width = len(label)
		}
	}

	for i, entry := range topEntries {
		fmt.Printf("  %-*s %d %s\n", width, labels[i], entry.Count, statsCountWord(entry.Count))
	}

	total := totalAcceptedCorrections(allEntries)
	fmt.Printf("\nTotal accepted corrections: %d\n", total)

	if tool, ok := mostTypoedTool(allEntries); ok {
		fmt.Printf("Most typoed tool: %s (%d)\n", tool.Name, tool.Count)
	}
}

func totalAcceptedCorrections(entries []itypes.HistoryEntry) int {
	total := 0
	for _, entry := range entries {
		total += entry.Count
	}
	return total
}

func mostTypoedTool(entries []itypes.HistoryEntry) (statsToolTotal, bool) {
	totals := make(map[string]int)
	for _, entry := range entries {
		tool := statsToolName(entry.To)
		if tool == "" {
			continue
		}
		totals[tool] += entry.Count
	}
	if len(totals) == 0 {
		return statsToolTotal{}, false
	}

	summary := make([]statsToolTotal, 0, len(totals))
	for name, count := range totals {
		summary = append(summary, statsToolTotal{Name: name, Count: count})
	}

	sort.Slice(summary, func(i, j int) bool {
		if summary[i].Count == summary[j].Count {
			return summary[i].Name < summary[j].Name
		}
		return summary[i].Count > summary[j].Count
	})

	return summary[0], true
}

func statsToolName(command string) string {
	parts := strings.Fields(strings.TrimSpace(command))
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func statsCountWord(count int) string {
	if count == 1 {
		return "time"
	}
	return "times"
}
