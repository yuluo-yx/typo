package cmd

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/yuluo-yx/typo/internal/config"
	"github.com/yuluo-yx/typo/internal/engine"
)

func cmdConfig(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: subcommand required (list, get, set, reset, gen)")
		return 1
	}

	cfg := config.Load()
	return runConfigSubcommand(cfg, args)
}

func runConfigSubcommand(cfg *config.Config, args []string) int {
	switch args[0] {
	case "list":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "Error: config list does not accept arguments")
			return 1
		}
		return cmdConfigList(cfg)
	case "get":
		return cmdConfigGet(cfg, args)
	case "set":
		return cmdConfigSet(cfg, args)
	case "reset":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "Error: config reset does not accept arguments")
			return 1
		}
		return cmdConfigReset(cfg)
	case "gen":
		return cmdConfigGen(cfg, args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", args[0])
		return 1
	}
}

func cmdConfigList(cfg *config.Config) int {
	if err := requireLoadedConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	for _, setting := range cfg.ListSettings() {
		fmt.Printf("%s=%s\n", setting.Key, setting.Value)
	}
	return 0
}

func cmdConfigGet(cfg *config.Config, args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Error: <key> required")
		return 1
	}
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "Error: config get requires exactly <key>")
		return 1
	}
	if err := requireLoadedConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	value, err := cfg.Get(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Println(value)
	return 0
}

func cmdConfigSet(cfg *config.Config, args []string) int {
	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: <key> and <value> required")
		return 1
	}
	if len(args) != 3 {
		fmt.Fprintln(os.Stderr, "Error: config set requires exactly <key> and <value>")
		return 1
	}
	if err := requireLoadedConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if err := cfg.Set(args[1], args[2]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Printf("Set %s=%s\n", args[1], args[2])
	return 0
}

func cmdConfigReset(cfg *config.Config) int {
	if err := requireConfigDirectory(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	if err := cfg.Reset(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Printf("Reset configuration: %s\n", cfg.ConfigFilePath())
	return 0
}

func cmdConfigGen(cfg *config.Config, args []string) int {
	fs := flag.NewFlagSet("config gen", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	force := fs.Bool("force", false, "overwrite existing config file")
	if err := fs.Parse(args[1:]); err != nil {
		return 1
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Error: config gen does not accept positional arguments")
		return 1
	}
	if err := requireConfigDirectory(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	if err := cfg.Generate(*force); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Printf("Generated configuration: %s\n", cfg.ConfigFilePath())
	return 0
}

func cmdRules(args []string) int {
	if message := rulesArgsError(args); message != "" {
		fmt.Fprintln(os.Stderr, message)
		return 1
	}

	cfg := config.Load()

	switch args[0] {
	case "list":
		return cmdRulesList(cfg)
	case "add":
		if err := requireConfigDirectory(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		eng := createEngine(cfg)
		if err := eng.AddRule(args[1], args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Printf("Added rule: %s -> %s\n", args[1], args[2])
		return 0
	case "remove":
		if err := requireConfigDirectory(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		eng := engine.NewEngine(
			engine.WithRules(engine.NewRules(cfg.ConfigDir)),
			engine.WithHistory(engine.NewHistory(cfg.ConfigDir)),
		)
		if err := eng.RemoveRule(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Printf("Removed rule: %s\n", args[1])
		return 0
	case "enable":
		return cmdRulesSetScopeEnabled(cfg, args, true)
	case "disable":
		return cmdRulesSetScopeEnabled(cfg, args, false)
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", args[0])
		return 1
	}
}

func rulesArgsError(args []string) string {
	if len(args) == 0 {
		return "Error: subcommand required (list, add, remove, enable, disable)"
	}

	switch args[0] {
	case "list":
		if len(args) != 1 {
			return "Error: rules list does not accept arguments"
		}
	case "add":
		if len(args) != 3 {
			return "Error: exactly two arguments required: <from> and <to>"
		}
	case "remove":
		if len(args) != 2 {
			return "Error: remove requires exactly <from>"
		}
	case "enable", "disable":
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Sprintf("Error: %s requires exactly one <scope>", args[0])
		}
	default:
		return fmt.Sprintf("Unknown subcommand: %s", args[0])
	}

	return ""
}

func cmdRulesList(cfg *config.Config) int {
	if err := requireLoadedConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	rulesStore := engine.NewRules(cfg.ConfigDir)
	for scope, ruleCfg := range cfg.User.Rules {
		if !ruleCfg.Enabled {
			_ = rulesStore.EnableRuleSet(scope, false)
		}
	}

	for _, r := range rulesStore.ListRules() {
		status := "enabled"
		if !r.Enable {
			status = "disabled"
		}
		fmt.Printf("%s -> %s [%s] (%s)\n", r.From, r.To, r.Scope, status)
	}
	return 0
}

func cmdRulesSetScopeEnabled(cfg *config.Config, args []string, enabled bool) int {
	action := args[0]
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "Error: %s requires exactly one <scope>\n", action)
		return 1
	}

	scope := strings.TrimSpace(args[1])
	if scope == "" {
		fmt.Fprintf(os.Stderr, "Error: %s requires exactly one <scope>\n", action)
		return 1
	}

	if _, exists := cfg.User.Rules[scope]; !exists {
		fmt.Fprintf(
			os.Stderr,
			"Error: unknown rule scope: %s (valid options: %s)\n",
			scope,
			strings.Join(SortedConfigRuleScopes(cfg), ", "),
		)
		return 1
	}

	key := fmt.Sprintf("rules.%s.enabled", scope)
	if err := requireLoadedConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	if err := cfg.Set(key, strconv.FormatBool(enabled)); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if enabled {
		fmt.Printf("Enabled rule scope: %s\n", scope)
		return 0
	}

	fmt.Printf("Disabled rule scope: %s\n", scope)
	return 0
}

func requireConfigDirectory(cfg *config.Config) error {
	if cfg == nil || cfg.ConfigDir == "" {
		return errors.New("config directory unavailable; cannot persist changes")
	}
	return nil
}

func requireLoadedConfig(cfg *config.Config) error {
	if err := requireConfigDirectory(cfg); err != nil {
		return err
	}
	return cfg.LoadError()
}
