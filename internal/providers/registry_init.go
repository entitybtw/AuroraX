package providers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurora/configuration"
	"aurora/internal/core"
	"aurora/internal/model_data"
)

// Initialize fetches models from all registered providers and populates the registry.
// This should be called on application startup.
func (r *ModelRegistry) Initialize(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	release, err := r.acquireRefresh(ctx)
	if err != nil {
		return err
	}
	defer release()
	return r.initialize(ctx)
}

func (r *ModelRegistry) initialize(ctx context.Context) error {
	providers, providerTypes, providerNames := r.snapshotProviders()
	configuredProviderModels, configuredProviderModelsMode := r.snapshotConfiguredProviderModels()
	autoFetchModels := r.snapshotProviderAutoFetchModels()
	autoFetchFilters := r.snapshotProviderAutoFetchFilters()

	fetched := r.fetchAllProviderModels(
		ctx,
		providers,
		providerTypes,
		providerNames,
		configuredProviderModels,
		configuredProviderModelsMode,
		autoFetchModels,
		autoFetchFilters,
	)
	// Expose the partial-failure count so the background refresh loop can
	// retry soon instead of waiting a full interval for providers that failed.
	atomic.StoreInt32(&r.lastRefreshFailed, int32(fetched.failedProviders))

	if fetched.totalModels == 0 {
		r.applyProviderRuntimeUpdates(fetched.runtimeUpdates)
		if fetched.failedProviders == len(providers) {
			return fmt.Errorf("failed to fetch models from any provider")
		}
		return fmt.Errorf("no models available: providers returned empty model lists")
	}

	r.applyFetchedInventory(providerTypes, fetched, len(providers))
	return nil
}

// snapshotProviders copies the registry's provider slice and type/name maps
// under a read lock so the rest of initialize can run without contending with
// readers.
func (r *ModelRegistry) snapshotProviders() ([]core.Provider, map[core.Provider]string, map[core.Provider]string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	providers := make([]core.Provider, len(r.providers))
	copy(providers, r.providers)
	providerTypes := make(map[core.Provider]string, len(r.providerTypes))
	providerNames := make(map[core.Provider]string, len(r.providerNames))
	maps.Copy(providerTypes, r.providerTypes)
	maps.Copy(providerNames, r.providerNames)
	return providers, providerTypes, providerNames
}

// fetchedInventory captures the result of one full provider fetch sweep.
// Shared by initial population and full refresh.
type fetchedInventory struct {
	models           map[string]*ModelInfo
	modelsByProvider map[string]map[string]*ModelInfo
	runtimeUpdates   map[string]providerRuntimeState
	totalModels      int
	failedProviders  int
}

// snapshotPreviousModelsByProvider copies the current per-provider inventory
// under a read lock so a fetch sweep can fall back to it when a provider's
// fetch fails (stale-while-revalidate instead of dropping the provider).
func (r *ModelRegistry) snapshotPreviousModelsByProvider() map[string]map[string]*ModelInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.modelsByProvider) == 0 {
		return nil
	}
	out := make(map[string]map[string]*ModelInfo, len(r.modelsByProvider))
	for providerName, models := range r.modelsByProvider {
		out[providerName] = maps.Clone(models)
	}
	return out
}

// retainPreviousModels re-registers the last known inventory of a provider
// whose fetch failed so a transient upstream error, empty response, or open
// circuit breaker does not remove its models until the next successful
// refresh. Entries are rebound to the current provider instance because a
// Rebuild may have replaced it since the inventory was recorded. Returns
// true when a previous inventory was found and retained.
func retainPreviousModels(out *fetchedInventory, previous map[string]map[string]*ModelInfo, provider core.Provider, providerName, providerType string) bool {
	prev := previous[providerName]
	if len(prev) == 0 {
		return false
	}
	retained := make(map[string]*ModelInfo, len(prev))
	for modelID, info := range prev {
		retained[modelID] = &ModelInfo{
			Model:        info.Model,
			Provider:     provider,
			ProviderName: providerName,
			ProviderType: providerType,
		}
	}
	out.modelsByProvider[providerName] = retained
	for modelID, info := range retained {
		if _, exists := out.models[modelID]; exists {
			continue
		}
		out.models[modelID] = info
		out.totalModels++
	}
	slog.Info("retained last known models after refresh failure",
		"provider", providerName,
		"models", len(retained),
	)
	return true
}

// fetchAllProviderModels runs ListModels (or applies a configured allowlist)
// for every registered provider and aggregates the results. Network calls
// happen outside any registry lock so live readers keep serving the previous
// inventory.
func (r *ModelRegistry) fetchAllProviderModels(
	ctx context.Context,
	providers []core.Provider,
	providerTypes map[core.Provider]string,
	providerNames map[core.Provider]string,
	configuredProviderModels map[string][]string,
	configuredProviderModelsMode config.ConfiguredProviderModelsMode,
	autoFetchModels map[string]bool,
	autoFetchFilters map[string]*compiledAutoFetchFilter,
) fetchedInventory {
	out := fetchedInventory{
		models:           make(map[string]*ModelInfo),
		modelsByProvider: make(map[string]map[string]*ModelInfo),
		runtimeUpdates:   make(map[string]providerRuntimeState),
	}
	previousModels := r.snapshotPreviousModelsByProvider()

	for _, provider := range providers {
		providerName := providerNames[provider]
		if providerName == "" {
			providerName = providerTypes[provider]
		}
		if providerName == "" {
			providerName = fmt.Sprintf("%p", provider)
		}

		configuredModels := configuredProviderModels[providerName]
		shouldAutoFetch := true
		if autoFetchModels != nil {
			if v, ok := autoFetchModels[providerName]; ok {
				shouldAutoFetch = v
			}
		}
		resp, configuredReason, fetchAt, err := fetchProviderInventory(
			ctx,
			provider,
			providerName,
			providerTypes[provider],
			configuredProviderModelsMode,
			configuredModels,
			shouldAutoFetch,
		)
		var configuredUpstreamError string
		if configuredReason != configuredProviderModelsNotApplied {
			attrs := []any{
				"provider", providerName,
				"reason", string(configuredReason),
				"configured_models", len(configuredModels),
			}
			if err != nil {
				configuredUpstreamError = err.Error()
				attrs = append(attrs, "error", err)
				slog.Warn("upstream ListModels failed, using configured provider models", attrs...)
			} else if configuredReason == configuredProviderModelsAllowlist ||
				configuredReason == configuredProviderModelsAutoFetchDisabled {
				slog.Debug("using configured provider models", attrs...)
			} else {
				slog.Warn("using configured provider models", attrs...)
			}
			err = nil
		}
		if err != nil {
			slog.Warn("failed to fetch models from provider",
				"provider", providerName,
				"error", err,
			)
			out.failedProviders++
			out.runtimeUpdates[providerName] = providerRuntimeState{
				registered:          true,
				lastModelFetchAt:    fetchAt,
				lastModelFetchError: err.Error(),
			}
			retainPreviousModels(&out, previousModels, provider, providerName, providerTypes[provider])
			continue
		}

		if resp == nil {
			err := errors.New("provider returned nil model list")
			slog.Warn("failed to fetch models from provider",
				"provider", providerName,
				"error", err,
			)
			out.failedProviders++
			out.runtimeUpdates[providerName] = providerRuntimeState{
				registered:          true,
				lastModelFetchAt:    fetchAt,
				lastModelFetchError: err.Error(),
			}
			retainPreviousModels(&out, previousModels, provider, providerName, providerTypes[provider])
			continue
		}

		// Narrow the discovered catalog to the operator's declared conditions.
		// This runs before the empty check so a provider whose entire catalog
		// is filtered away is treated as "returned no models" rather than
		// registering models the operator explicitly excluded.
		fetchedCount := len(resp.Data)
		resp, removedByFilter := applyAutoFetchFilter(providerName, autoFetchFilters[providerName], resp)

		if len(resp.Data) == 0 {
			// Say who emptied the list: an empty upstream response and a
			// filter that rejected every fetched model are different faults
			// with different fixes.
			err := errors.New("provider returned empty model list")
			if removedByFilter > 0 {
				err = fmt.Errorf("autofetch filter removed all %d fetched models", fetchedCount)
				slog.Warn("autofetch filter removed every model",
					"provider", providerName,
					"fetched", fetchedCount,
					"removed", removedByFilter,
				)
			} else {
				slog.Warn("provider returned empty model list",
					"provider", providerName,
				)
			}
			out.runtimeUpdates[providerName] = providerRuntimeState{
				registered:          true,
				lastModelFetchAt:    fetchAt,
				lastModelFetchError: err.Error(),
			}
			// An empty response is often a transient upstream hiccup: keep the
			// last known inventory rather than dropping the provider's models
			// for a whole refresh interval.
			if !retainPreviousModels(&out, previousModels, provider, providerName, providerTypes[provider]) {
				if _, ok := out.modelsByProvider[providerName]; !ok {
					out.modelsByProvider[providerName] = make(map[string]*ModelInfo)
				}
			}
			continue
		}

		runtimeUpdate := providerRuntimeState{
			registered:          true,
			lastModelFetchAt:    fetchAt,
			lastModelFetchError: configuredUpstreamError,
		}
		// Stamp a successful refresh whenever the inventory resolved cleanly:
		// a live upstream ListModels call succeeded (NotApplied), the
		// configured allowlist was applied without an upstream error, or
		// auto-fetch is intentionally off (AutoFetchDisabled). Without this,
		// allowlist-only and auto-fetch-disabled providers (e.g., Jina with an
		// explicit `models:` list) permanently report "Starting" in the admin
		// UI because the success timestamp never gets set.
		successfulRefresh := configuredReason == configuredProviderModelsNotApplied ||
			configuredReason == configuredProviderModelsAutoFetchDisabled ||
			(configuredReason == configuredProviderModelsAllowlist && configuredUpstreamError == "")
		if successfulRefresh {
			runtimeUpdate.lastModelFetchSuccessAt = fetchAt
		}
		out.runtimeUpdates[providerName] = runtimeUpdate

		if _, ok := out.modelsByProvider[providerName]; !ok {
			out.modelsByProvider[providerName] = make(map[string]*ModelInfo, len(resp.Data))
		}

		for _, model := range resp.Data {
			info := &ModelInfo{
				Model:        model,
				Provider:     provider,
				ProviderName: providerName,
				ProviderType: providerTypes[provider],
			}
			out.modelsByProvider[providerName][model.ID] = info

			if _, exists := out.models[model.ID]; exists {
				// First provider wins for unqualified lookups; later duplicates
				// stay reachable via modelsByProvider but lose the bare-id slot.
				slog.Debug("model already registered, skipping",
					"model", model.ID,
					"provider", providerName,
					"owner", model.OwnedBy,
				)
				continue
			}

			out.models[model.ID] = info
			out.totalModels++
		}
	}

	return out
}

// applyFetchedInventory enriches the freshly fetched maps with metadata,
// atomically swaps them onto the registry, marks initialized, and emits the
// summary log line.
func (r *ModelRegistry) applyFetchedInventory(
	providerTypes map[core.Provider]string,
	fetched fetchedInventory,
	totalProviders int,
) {
	r.mu.RLock()
	list := r.modelList
	r.mu.RUnlock()
	configOverrides := r.snapshotConfigOverrides()
	metadataStats := metadataEnrichmentStats{}
	if list != nil {
		metadataStats = enrichProviderModelMaps(list, providerTypes, fetched.modelsByProvider, nil)
	}
	metadataStats.Enriched += applyConfigMetadataOverrides(configOverrides, fetched.modelsByProvider, nil)

	r.mu.Lock()
	r.models = fetched.models
	r.modelsByProvider = fetched.modelsByProvider
	r.applyProviderRuntimeUpdatesLocked(fetched.runtimeUpdates)
	r.invalidateSortedCaches()
	r.mu.Unlock()

	r.initMu.Lock()
	r.initialized = true
	r.initMu.Unlock()

	attrs := []any{
		"total_models", fetched.totalModels,
		"providers", totalProviders,
		"failed_providers", fetched.failedProviders,
	}
	attrs = append(attrs, metadataStats.slogAttrs()...)
	slog.Info("model registry initialized", attrs...)
}

func fetchProviderInventory(
	ctx context.Context,
	provider core.Provider,
	providerName string,
	providerType string,
	mode config.ConfiguredProviderModelsMode,
	configuredModels []string,
	autoFetch bool,
) (*core.ModelsResponse, configuredProviderModelsApplyReason, time.Time, error) {
	fetchAt := time.Now().UTC()

	// OpenRouter exposes a huge multi-vendor catalog, so an explicit `models:`
	// list is always enforced as an allowlist: only the models the operator
	// listed are exposed/routed, never the full upstream catalog. Other
	// providers keep the configured global mode (fallback/allowlist).
	effectiveMode := mode
	if providerType == ProviderTypeOpenRouter && len(configuredModels) > 0 {
		effectiveMode = config.ConfiguredProviderModelsModeAllowlist
	}

	if effectiveMode == config.ConfiguredProviderModelsModeAllowlist && len(configuredModels) > 0 {
		resp, reason := applyConfiguredProviderModels(
			providerName,
			providerType,
			effectiveMode,
			configuredModels,
			nil,
			nil,
			fetchAt.Unix(),
		)
		return resp, reason, fetchAt, nil
	}

	// When auto_fetch_models is false, skip the upstream /models call and use
	// only the configured model list (or report nothing configured). This is a
	// healthy steady state, so it carries a dedicated reason instead of being
	// indistinguishable from a nil upstream response.
	if !autoFetch {
		if len(configuredModels) > 0 {
			resp, _ := applyConfiguredProviderModels(
				providerName,
				providerType,
				effectiveMode,
				configuredModels,
				nil,
				nil,
				fetchAt.Unix(),
			)
			return resp, configuredProviderModelsAutoFetchDisabled, fetchAt, nil
		}
		return nil, configuredProviderModelsNotApplied, fetchAt, nil
	}

	resp, err := provider.ListModels(ctx)
	fetchAt = time.Now().UTC()
	resp, reason := applyConfiguredProviderModels(
		providerName,
		providerType,
		effectiveMode,
		configuredModels,
		resp,
		err,
		fetchAt.Unix(),
	)
	return resp, reason, fetchAt, err
}

func (r *ModelRegistry) applyProviderRuntimeUpdates(updates map[string]providerRuntimeState) {
	if len(updates) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.applyProviderRuntimeUpdatesLocked(updates)
}

func (r *ModelRegistry) applyProviderRuntimeUpdatesLocked(updates map[string]providerRuntimeState) {
	for providerName, update := range updates {
		current := r.providerRuntime[providerName]
		current.registered = update.registered || current.registered
		if !update.lastModelFetchAt.IsZero() {
			current.lastModelFetchAt = update.lastModelFetchAt
			// A non-zero fetchAt represents a refresh attempt whose outcome
			// is captured authoritatively in lastModelFetchError (empty =
			// success, non-empty = failure). Overwrite unconditionally so an
			// old error doesn't survive a subsequent successful refresh —
			// this matters in particular for allowlist-mode refreshes which
			// don't bump SuccessAt but still produce usable models.
			current.lastModelFetchError = strings.TrimSpace(update.lastModelFetchError)
		}
		if !update.lastModelFetchSuccessAt.IsZero() {
			current.lastModelFetchSuccessAt = update.lastModelFetchSuccessAt
			current.lastAvailabilityOKAt = update.lastModelFetchSuccessAt
			current.lastAvailabilityError = ""
		}
		r.providerRuntime[providerName] = current
	}
}

// Refresh updates the model registry by fetching fresh model lists from providers.
// This can be called periodically to keep the registry up to date.
func (r *ModelRegistry) Refresh(ctx context.Context) error {
	return r.Initialize(ctx)
}

func (r *ModelRegistry) acquireRefresh(ctx context.Context) (func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, registryRefreshAcquireError(err)
	}
	ch := r.refreshSemaphore()
	select {
	case ch <- struct{}{}:
		return func() { <-ch }, nil
	case <-ctx.Done():
		return nil, registryRefreshAcquireError(ctx.Err())
	}
}

func (r *ModelRegistry) refreshSemaphore() chan struct{} {
	r.refreshOnce.Do(func() {
		if r.refreshCh == nil {
			r.refreshCh = make(chan struct{}, 1)
		}
	})
	return r.refreshCh
}

func registryRefreshAcquireError(err error) *core.GatewayError {
	if errors.Is(err, context.DeadlineExceeded) {
		return core.NewProviderError("model_registry", http.StatusGatewayTimeout, "model registry refresh timed out before start", err)
	}
	return core.NewProviderError("model_registry", http.StatusRequestTimeout, "model registry refresh canceled before start", err)
}

// InitializeAsync starts model fetching in a background goroutine.
// It first loads any cached models for immediate availability, then refreshes from network.
// Returns immediately after loading cache. The background goroutine will update models
// and save to cache when network fetch completes.
func (r *ModelRegistry) InitializeAsync(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	// First, try to load from cache for instant startup
	cached, err := r.LoadFromCache(ctx)
	if err != nil {
		slog.Warn("failed to load models from cache", "error", err)
	} else if cached > 0 {
		slog.Debug("serving traffic with cached models while refreshing", "cached_models", cached)
	}

	// Start background initialization. Derive the timeout from the caller's
	// ctx so shutdown cancellation propagates instead of leaving the goroutine
	// running until the 60s timeout fires on its own.
	go func() {
		initCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		if err := r.Initialize(initCtx); err != nil {
			slog.Warn("background model initialization failed", "error", err)
			return
		}

		// Save to cache for next startup
		if err := r.SaveToCache(initCtx); err != nil {
			slog.Warn("failed to save models to cache", "error", err)
		}
	}()
}

// IsInitialized returns true if at least one successful network fetch has completed.
// This can be used to check if the registry has fresh data or is only serving from cache.
func (r *ModelRegistry) IsInitialized() bool {
	r.initMu.Lock()
	defer r.initMu.Unlock()
	return r.initialized
}

// StartBackgroundRefresh starts a goroutine that periodically refreshes the model registry.
// If modelListURL is non-empty, the model list is also re-fetched on each tick.
// The returned stop function is blocking: it cancels the refresh loop and waits
// for the goroutine to exit before returning, so callers should expect it to
// block during shutdown until any in-flight refresh work unwinds.
func (r *ModelRegistry) StartBackgroundRefresh(interval time.Duration, modelListURL string) func() {
	if interval <= 0 {
		// time.NewTicker panics on non-positive durations and a refresh loop
		// with a zero interval would be meaningless. Skip the goroutine and
		// hand back a no-op stop so callers can still defer it safely.
		slog.Debug("model registry background refresh disabled", "interval", interval)
		return func() {}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var stopOnce sync.Once

	go func() {
		defer close(done)
		timer := time.NewTimer(interval)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				refreshCtx, refreshCancel := context.WithTimeout(ctx, 30*time.Second)
				err := r.Initialize(refreshCtx)
				refreshCancel()
				if err != nil {
					if !isBenignBackgroundRefreshError(ctx, err) {
						slog.Warn("background model refresh failed", "error", err)
					}
				} else {
					func() {
						cacheCtx, cacheCancel := context.WithTimeout(ctx, 10*time.Second)
						defer cacheCancel()
						if err := r.SaveToCache(cacheCtx); err != nil {
							if !isBenignBackgroundRefreshError(ctx, err) {
								slog.Warn("failed to save models to cache after refresh", "error", err)
							}
						}
					}()
				}

				// Also refresh model list if configured
				if modelListURL != "" {
					r.refreshModelList(ctx, modelListURL)
				}

				// After a partial failure, retry within a minute instead of
				// waiting the full interval, so recovered providers repopulate
				// quickly. Never slows down a sub-minute interval.
				next := interval
				if r.lastFailedProviders() > 0 && partialRefreshRetryDelay < next {
					next = partialRefreshRetryDelay
				}
				timer.Reset(next)
			}
		}
	}()

	return func() {
		stopOnce.Do(func() {
			cancel()
			<-done
		})
	}
}

// partialRefreshRetryDelay bounds how soon the background loop retries after
// a refresh in which at least one provider failed.
const partialRefreshRetryDelay = time.Minute

// lastFailedProviders reports how many providers failed in the most recent
// fetch sweep.
func (r *ModelRegistry) lastFailedProviders() int {
	return int(atomic.LoadInt32(&r.lastRefreshFailed))
}

// RefreshModelList fetches the external model metadata list and re-enriches all
// currently registered models. It does not persist the model cache; callers that
// want durable startup data should call SaveToCache after this succeeds.
func (r *ModelRegistry) RefreshModelList(ctx context.Context, url string) (int, error) {
	if strings.TrimSpace(url) == "" {
		return 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	release, err := r.acquireRefresh(ctx)
	if err != nil {
		return 0, err
	}
	defer release()

	models, _, err := r.refreshModelListLocked(ctx, url)
	return models, err
}

func (r *ModelRegistry) refreshModelListLocked(ctx context.Context, url string) (int, metadataEnrichmentStats, error) {
	list, raw, err := modeldata.Fetch(ctx, url)
	if err != nil {
		return 0, metadataEnrichmentStats{}, err
	}
	if list == nil {
		return 0, metadataEnrichmentStats{}, nil
	}

	r.applyUserOverridesToList(list)
	metadataStats := r.setModelListAndEnrich(list, raw)
	return len(list.Models), metadataStats, nil
}

// refreshModelList fetches the model list and re-enriches all models.
func (r *ModelRegistry) refreshModelList(ctx context.Context, url string) {
	fetchCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	release, err := r.acquireRefresh(fetchCtx)
	if err != nil {
		if !isBenignBackgroundRefreshError(ctx, err) {
			slog.Warn("failed to acquire model list refresh", "url", url, "error", err)
		}
		return
	}
	var (
		models        int
		metadataStats metadataEnrichmentStats
	)
	func() {
		defer release()
		models, metadataStats, err = r.refreshModelListLocked(fetchCtx, url)
	}()
	if err != nil {
		if !isBenignBackgroundRefreshError(ctx, err) {
			slog.Warn("failed to refresh model list", "url", url, "error", err)
		}
		return
	}
	if models == 0 {
		return
	}

	if err := r.SaveToCache(fetchCtx); err != nil {
		if !isBenignBackgroundRefreshError(ctx, err) {
			slog.Warn("failed to save cache after model list refresh", "error", err)
		}
	}
	attrs := []any{"models", models}
	attrs = append(attrs, metadataStats.slogAttrs()...)
	slog.Debug("model list refreshed", attrs...)
}

func isBenignBackgroundRefreshError(parent context.Context, err error) bool {
	if err == nil {
		return true
	}
	if parent == nil || parent.Err() == nil {
		return false
	}
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
