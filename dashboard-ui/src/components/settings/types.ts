export type SettingsTab = "general" | "providers" | "infrastructure" | "caching" | "networking";

export interface RuntimeRefreshResponse {
  steps?: { name: string; status: string }[];
}

export interface DashboardSettingsFormState {
  client: {
    port?: string;
    base_path?: string;
    body_size_limit: string;
    swagger_enabled?: boolean;
    pprof_enabled?: boolean;
    configured_provider_models_mode: string;
    keep_only_aliases_at_models_endpoint: boolean;
    allow_passthrough_v1_alias: boolean;
    admin_endpoints_enabled?: boolean;
    admin_ui_enabled?: boolean;
    enable_anthropic_ingress?: boolean;
  };
  caching: {
    model_cache_backend?: string;
    model_cache_local_dir?: string;
    model_cache_redis_url?: string;
    model_cache_redis_key?: string;
    model_cache_redis_ttl_seconds?: number;
    model_refresh_interval_seconds: number;
    model_list_url: string;
    model_list_local_path?: string;
    model_list_user_overrides_path?: string;
    exact_cache_enabled: boolean;
    exact_cache_redis_url?: string;
    exact_cache_ttl_seconds: number;
    exact_cache_redis_key: string;
    semantic_cache_enabled: boolean;
    semantic_similarity_threshold: number;
    semantic_prompt_similarity_min: number;
    semantic_ttl_seconds: number;
    semantic_max_conversation_messages: number;
    semantic_exclude_system_prompt: boolean;
    semantic_embedder_provider: string;
    semantic_embedder_model: string;
    semantic_vector_store_type: string;
    semantic_vector_store_hints?: string[];
    semantic_vector_store_url?: string;
    semantic_vector_store_collection?: string;
    semantic_vector_store_table?: string;
    semantic_vector_store_namespace?: string;
    semantic_vector_store_class?: string;
    semantic_vector_store_dimension?: number;
    semantic_vector_store_api_key_set?: boolean;
    prompt_cache_mode: string;
    prompt_cache_system_prompt: boolean;
    prompt_cache_first_message: boolean;
    prompt_cache_tools: boolean;
    prompt_cache_min_tokens: number;
  };
  logging: {
    enabled: boolean;
    log_bodies: boolean;
    log_headers: boolean;
    buffer_size: number;
    flush_interval_seconds: number;
    retention_days: number;
    only_model_interactions: boolean;
  };
  observability: {
    metrics_enabled: boolean;
    metrics_endpoint: string;
  };
  performance: {
    http_timeout_seconds: number;
    http_response_header_timeout_seconds: number;
    workflow_refresh_interval_seconds: number;
    retry_max_retries: number;
    retry_initial_backoff_milliseconds: number;
    retry_max_backoff_milliseconds: number;
    retry_backoff_factor: number;
    retry_jitter_factor: number;
    circuit_breaker_failure_threshold: number;
    circuit_breaker_success_threshold: number;
    circuit_breaker_timeout_milliseconds: number;
  };
  security: {
    guardrails_enabled: boolean;
    batch_guardrails: boolean;

  };
  pricing: {
    enforce_returning_usage_data: boolean;
    pricing_recalculation_enabled: boolean;
    usage_retention_days: number;
  };
  token_saver: {
    enabled: boolean;
    apply_streaming: boolean;
    endpoints: string[];
    output_enabled: boolean;
    output_profile: string;
    output_level: string;
    emit_headers: boolean;
    on_error: string;
    model_include: string[];
    model_exclude: string[];
    provider_include: string[];
    provider_exclude: string[];
    audit_enabled: boolean;
  };
  proxy: {
    http_proxy: string;
    https_proxy: string;
    no_proxy: string;
    proxy_auth_enabled: boolean;
    ca_cert_pem: string;
  };
}

export interface DashboardSettingsSaveResponse {
  message: string;
  refresh_suggested: boolean;
  requires_restart: boolean;
  restart_reasons?: string[];
}
