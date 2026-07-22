package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"aurora/internal/core"
	"aurora/internal/model_data"
)

// Defaults mirror config.ModelListConfig defaults; duplicated here to keep the
// CLI runnable without loading full config (which would require valid YAML and
// provider env vars).
const (
	defaultModelListURL       = "https://raw.githubusercontent.com/aurorallm/aurora/refs/heads/main/docs-assets/assets/models.json"
	defaultModelListLocalPath = "data/models.local.json"
	defaultUserOverridesPath  = "data/user_pricing.yaml"
)

// runModelsSubcommand dispatches `aurora models <sub> [flags]`. Returns true
// when the args matched a models subcommand (and the process should exit with
// the returned code), false when args were not for this dispatcher.
func runModelsSubcommand(args []string, stdout, stderr io.Writer) (handled bool, exitCode int) {
	if len(args) < 2 || args[0] != "models" {
		return false, 0
	}

	switch args[1] {
	case "sync":
		return true, modelsSyncCmd(args[2:], stdout, stderr)
	case "diff":
		return true, modelsDiffCmd(args[2:], stdout, stderr)
	case "show":
		return true, modelsShowCmd(args[2:], stdout, stderr)
	case "-h", "--help", "help":
		printModelsHelp(stdout)
		return true, 0
	default:
		fmt.Fprintf(stderr, "unknown models subcommand: %q\n\n", args[1])
		printModelsHelp(stderr)
		return true, 2
	}
}

func printModelsHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage: aurora models <command> [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  sync   Download upstream model registry to the local snapshot file.")
	fmt.Fprintln(w, "  diff   Show pricing/metadata diff between upstream and local snapshot.")
	fmt.Fprintln(w, "  show   Print effective pricing for a model after merging user overrides.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Run `aurora models <command> -h` for command-specific flags.")
}

func modelsSyncCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("models sync", flag.ContinueOnError)
	fs.SetOutput(stderr)
	url := fs.String("url", "", "URL to fetch from (defaults to MODEL_LIST_URL or built-in default)")
	out := fs.String("out", "", "Output path (defaults to MODEL_LIST_LOCAL_PATH or data/models.local.json)")
	timeout := fs.Duration("timeout", 60*time.Second, "Fetch timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	resolvedURL, resolvedOut := resolveSyncTargets(*url, *out)
	if resolvedURL == "" {
		fmt.Fprintln(stderr, "no URL configured: pass -url, set MODEL_LIST_URL, or define cache.model.model_list.url")
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	fmt.Fprintf(stdout, "Fetching %s...\n", resolvedURL)
	list, raw, err := modeldata.Fetch(ctx, resolvedURL)
	if err != nil {
		fmt.Fprintf(stderr, "fetch failed: %v\n", err)
		return 1
	}
	if list == nil {
		fmt.Fprintln(stderr, "fetch returned no data (URL was empty?)")
		return 1
	}

	if err := modeldata.SaveToFile(resolvedOut, raw); err != nil {
		fmt.Fprintf(stderr, "save failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "Wrote %s (%d models, %d providers, %d provider_models, %d bytes)\n",
		resolvedOut, len(list.Models), len(list.Providers), len(list.ProviderModels), len(raw))
	return 0
}

func modelsDiffCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("models diff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	url := fs.String("url", "", "Remote URL to compare against (defaults to MODEL_LIST_URL)")
	localPath := fs.String("local", "", "Local snapshot path (defaults to MODEL_LIST_LOCAL_PATH)")
	timeout := fs.Duration("timeout", 60*time.Second, "Fetch timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	resolvedURL, resolvedLocal := resolveSyncTargets(*url, *localPath)
	if resolvedURL == "" {
		fmt.Fprintln(stderr, "no URL configured for comparison")
		return 1
	}

	localList, _, err := modeldata.LoadFromFile(resolvedLocal)
	if err != nil {
		fmt.Fprintf(stderr, "load local failed: %v\n", err)
		return 1
	}
	if localList == nil {
		fmt.Fprintf(stderr, "local snapshot %s not present; run `aurora models sync` first\n", resolvedLocal)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	remote, _, err := modeldata.Fetch(ctx, resolvedURL)
	if err != nil {
		fmt.Fprintf(stderr, "fetch failed: %v\n", err)
		return 1
	}

	printPricingDiff(stdout, localList, remote)
	return 0
}

func modelsShowCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("models show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	localPath := fs.String("local", "", "Local snapshot path (defaults to MODEL_LIST_LOCAL_PATH)")
	overridesPath := fs.String("overrides", "", "User overrides path (defaults to MODEL_LIST_USER_OVERRIDES_PATH)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(stderr, "usage: aurora models show <modelID> [providerType/modelID ...]")
		return 2
	}

	if *localPath == "" {
		*localPath = envOr("MODEL_LIST_LOCAL_PATH", defaultModelListLocalPath)
	}
	if *overridesPath == "" {
		*overridesPath = envOr("MODEL_LIST_USER_OVERRIDES_PATH", defaultUserOverridesPath)
	}

	list, _, err := modeldata.LoadFromFile(*localPath)
	if err != nil {
		fmt.Fprintf(stderr, "load local failed: %v\n", err)
		return 1
	}
	if list == nil {
		fmt.Fprintf(stderr, "local snapshot %s not present; run `aurora models sync` first\n", *localPath)
		return 1
	}

	overrides, err := modeldata.LoadUserOverrides(*overridesPath)
	if err != nil {
		fmt.Fprintf(stderr, "load overrides failed: %v\n", err)
		return 1
	}
	modeldata.ApplyUserOverrides(list, overrides)

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	for _, query := range rest {
		providerType, modelID, ok := strings.Cut(query, "/")
		if !ok {
			modelID = query
			providerType = ""
		}
		meta := modeldata.Resolve(list, providerType, modelID)
		if meta == nil {
			fmt.Fprintf(stdout, "%s: not found\n", query)
			continue
		}
		fmt.Fprintf(stdout, "%s:\n", query)
		_ = enc.Encode(meta)
	}
	return 0
}

func resolveSyncTargets(url, out string) (resolvedURL, resolvedOut string) {
	resolvedURL = url
	if resolvedURL == "" {
		resolvedURL = envOr("MODEL_LIST_URL", defaultModelListURL)
	}
	resolvedOut = out
	if resolvedOut == "" {
		resolvedOut = envOr("MODEL_LIST_LOCAL_PATH", defaultModelListLocalPath)
	}
	return resolvedURL, resolvedOut
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// printPricingDiff prints a stable, line-oriented diff of pricing changes
// between the local snapshot and the remote registry. Format:
//
//	~ <key>           input/output local -> remote
//	+ <key>           added in remote
//	- <key>           missing from remote (deprecated upstream)
//
// Only models whose pricing differs are shown, so a clean run prints only a
// summary line.
func printPricingDiff(w io.Writer, local, remote *modeldata.ModelList) {
	if remote == nil {
		fmt.Fprintln(w, "remote returned no data")
		return
	}

	added, removed, changed := comparePricing(local, remote)
	sort.Strings(added)
	sort.Strings(removed)
	sort.Slice(changed, func(i, j int) bool { return changed[i].Key < changed[j].Key })

	for _, key := range removed {
		fmt.Fprintf(w, "- %s\n", key)
	}
	for _, key := range added {
		fmt.Fprintf(w, "+ %s\n", key)
	}
	for _, c := range changed {
		fmt.Fprintf(w, "~ %s  in: %s -> %s  out: %s -> %s\n",
			c.Key,
			fmtFloatPtr(c.LocalIn), fmtFloatPtr(c.RemoteIn),
			fmtFloatPtr(c.LocalOut), fmtFloatPtr(c.RemoteOut),
		)
	}
	fmt.Fprintf(w, "\n%d added, %d removed, %d pricing changes\n", len(added), len(removed), len(changed))
}

type pricingChange struct {
	Key       string
	LocalIn   *float64
	RemoteIn  *float64
	LocalOut  *float64
	RemoteOut *float64
}

func comparePricing(local, remote *modeldata.ModelList) (added, removed []string, changed []pricingChange) {
	localPricing := flattenPricing(local)
	remotePricing := flattenPricing(remote)

	for key, lp := range localPricing {
		rp, ok := remotePricing[key]
		if !ok {
			removed = append(removed, key)
			continue
		}
		if !samePricingScalars(lp, rp) {
			changed = append(changed, pricingChange{
				Key:       key,
				LocalIn:   lp.InputPerMtok,
				RemoteIn:  rp.InputPerMtok,
				LocalOut:  lp.OutputPerMtok,
				RemoteOut: rp.OutputPerMtok,
			})
		}
	}
	for key := range remotePricing {
		if _, ok := localPricing[key]; !ok {
			added = append(added, key)
		}
	}
	return added, removed, changed
}

func flattenPricing(list *modeldata.ModelList) map[string]*core.ModelPricing {
	out := make(map[string]*core.ModelPricing)
	if list == nil {
		return out
	}
	for id, model := range list.Models {
		if model.Pricing != nil {
			out[id] = model.Pricing
		}
	}
	for key, pm := range list.ProviderModels {
		if pm.Pricing != nil {
			out[key] = pm.Pricing
		}
	}
	return out
}

func samePricingScalars(a, b *core.ModelPricing) bool {
	if a == nil || b == nil {
		return a == b
	}
	return floatPtrEq(a.InputPerMtok, b.InputPerMtok) &&
		floatPtrEq(a.OutputPerMtok, b.OutputPerMtok) &&
		floatPtrEq(a.CachedInputPerMtok, b.CachedInputPerMtok) &&
		floatPtrEq(a.CacheWritePerMtok, b.CacheWritePerMtok) &&
		floatPtrEq(a.ReasoningOutputPerMtok, b.ReasoningOutputPerMtok)
}

func floatPtrEq(a, b *float64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func fmtFloatPtr(p *float64) string {
	if p == nil {
		return "—"
	}
	return fmt.Sprintf("%g", *p)
}
