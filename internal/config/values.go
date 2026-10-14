package config

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/zarbanio/bundler/pkg/modules/entities"
	"github.com/zarbanio/bundler/pkg/signer"
	"github.com/zarbanio/bundler/pkg/store"
	"github.com/zarbanio/bundler/pkg/userop"
)

type Values struct {
	// Documented variables.
	PrivateKey                   string
	EthClientUrl                 string
	Port                         int
	DataDirectory                string
	SupportedEntryPoints         []common.Address
	MaxVerificationGas           *big.Int
	MaxBatchGasLimit             *big.Int
	MaxOpTTL                     time.Duration
	OpLookupLimit                uint64
	Beneficiary                  string
	NativeBundlerCollectorTracer string
	NativeBundlerExecutorTracer  string
	ReputationConstants          *entities.ReputationConstants

	// Searcher mode variables.
	EthBuilderUrls    []string
	BlocksInTheFuture int

	// Observability variables.
	OTELServiceName      string
	OTELCollectorHeaders map[string]string
	OTELCollectorUrl     string
	OTELInsecureMode     bool

	// Alternative mempool variables.
	AltMempoolIPFSGateway string
	AltMempoolIds         []string

	// Rollup related variables.
	IsOpStackNetwork   bool
	IsRIP7212Supported bool
	IsArbStackNetwork  bool

	// EntryPoint version of the preferred (index 0) EntryPoint. Retained for
	// the rollup preVerificationGas default and backward compatibility.
	EntryPointVersion userop.Version

	// EntryPointVersions is the deduped set of versions implied by every
	// configured EntryPoint, in the order they first appear. The bundler builds
	// one gas Overhead per entry so a single instance serves all of them.
	EntryPointVersions []userop.Version

	// Storage backend configuration.
	StoreBackend    string
	StoreURL        string
	StoreNamespace  string
	StoreUsername   string
	StorePassword   string
	StoreDB         int
	StorePoolSize   int
	StoreAddrs      []string
	StoreMasterName string
	StoreOpTTL      time.Duration
	// StoreOpTTLExplicit records whether the operator set the TTL, so an
	// explicit 0 can disable it while an unset value falls back to MaxOpTTL for
	// shared backends.
	StoreOpTTLExplicit bool

	// Undocumented variables.
	DebugMode bool
	GinMode   string
}

// StoreConfig builds the transport-agnostic store configuration. When the
// backend is left empty it is inferred: a store URL implies Redis/Valkey,
// otherwise BadgerDB is used for backward compatibility.
func (v *Values) StoreConfig() store.Config {
	backend := store.Backend(strings.ToLower(strings.TrimSpace(v.StoreBackend)))
	url := v.StoreURL
	switch {
	case backend == "" && (url != "" || len(v.StoreAddrs) > 0):
		// A URL or a Sentinel/Cluster address list implies a network backend.
		backend = store.BackendRedis
	case backend == "":
		backend = store.BackendBadger
	}
	// Normalize the valkey:// scheme to what go-redis understands.
	if strings.HasPrefix(url, "valkey://") {
		url = "redis://" + strings.TrimPrefix(url, "valkey://")
		backend = store.BackendValkey
	}

	opTTL := v.StoreOpTTL
	if !v.StoreOpTTLExplicit && (backend == store.BackendRedis || backend == store.BackendValkey) {
		// Default the shared-store pending-op TTL to the in-process op TTL so a
		// crashed replica cannot leave state behind forever. An explicit value
		// (including 0, to disable) is honored as-is.
		opTTL = v.MaxOpTTL
	}

	return store.Config{
		Backend:       backend,
		Namespace:     v.StoreNamespace,
		DataDirectory: v.DataDirectory,
		URL:           url,
		Addrs:         v.StoreAddrs,
		MasterName:    v.StoreMasterName,
		Username:      v.StoreUsername,
		Password:      v.StorePassword,
		DB:            v.StoreDB,
		PoolSize:      v.StorePoolSize,
		OpTTL:         opTTL,
	}
}

func envKeyValStringToMap(s string) map[string]string {
	out := map[string]string{}
	for _, pair := range strings.Split(s, "&") {
		kv := strings.Split(pair, "=")
		if len(kv) != 2 {
			break
		}
		out[kv[0]] = kv[1]
	}
	return out
}

// dedupeVersions returns the versions implied by the configured EntryPoints, in
// first-seen order with duplicates removed. When no EntryPoints are configured
// it returns the fallback version so the resolver is never empty.
func dedupeVersions(entryPoints []common.Address, fallback userop.Version) []userop.Version {
	seen := map[userop.Version]bool{}
	out := []userop.Version{}
	for _, ep := range entryPoints {
		if ep == (common.Address{}) {
			continue
		}
		v := userop.DetectVersionFromEntryPoint(ep)
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	if len(out) == 0 {
		return []userop.Version{fallback}
	}
	return out
}

func envArrayToAddressSlice(s string) []common.Address {
	env := strings.Split(s, ",")
	slc := []common.Address{}
	for _, ep := range env {
		slc = append(slc, common.HexToAddress(strings.TrimSpace(ep)))
	}

	return slc
}

func envArrayToStringSlice(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, ",")
}

func variableNotSetOrIsNil(env string) bool {
	return !viper.IsSet(env) || viper.GetString(env) == ""
}

// GetValues returns config for the bundler that has been read in from env vars.
func GetValues() *Values {
	// Default variables
	viper.SetDefault("erc4337_bundler_port", 4337)
	viper.SetDefault("erc4337_bundler_data_directory", "/tmp/qubenode_bundler")
	// Default to every canonical EntryPoint (v0.6–v0.9) so a single instance
	// serves all versions at once. Order matters: index 0 is the preferred
	// EntryPoint reported by eth_supportedEntryPoints.
	viper.SetDefault(
		"erc4337_bundler_supported_entry_points",
		strings.Join([]string{
			"0x433709009B8330FDa32311DF1C2AFA402eD8D009", // v0.9
			"0x4337084D9E255Ff0702461CF8895CE9E3b5Ff108", // v0.8
			"0x0000000071727De22E5E9d8BAf0edAc6f37da032", // v0.7
			"0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789", // v0.6
		}, ","),
	)
	viper.SetDefault("erc4337_bundler_entrypoint_version", "auto")
	viper.SetDefault("erc4337_bundler_max_verification_gas", 6000000)
	viper.SetDefault("erc4337_bundler_max_batch_gas_limit", 18000000)
	viper.SetDefault("erc4337_bundler_max_op_ttl_seconds", 180)
	viper.SetDefault("erc4337_bundler_op_lookup_limit", 2000)
	viper.SetDefault("erc4337_bundler_blocks_in_the_future", 6)
	viper.SetDefault("erc4337_bundler_otel_insecure_mode", false)
	viper.SetDefault("erc4337_bundler_is_op_stack_network", false)
	viper.SetDefault("erc4337_bundler_is_arb_stack_network", false)
	viper.SetDefault("erc4337_bundler_is_rip7212_supported", false)
	viper.SetDefault("erc4337_bundler_debug_mode", false)
	viper.SetDefault("erc4337_bundler_gin_mode", gin.ReleaseMode)

	// Storage backend defaults. An empty backend is inferred from the store URL
	// (Redis/Valkey) or falls back to BadgerDB.
	viper.SetDefault("erc4337_bundler_store_backend", "")
	viper.SetDefault("erc4337_bundler_store_namespace", "")
	viper.SetDefault("erc4337_bundler_store_db", 0)
	viper.SetDefault("erc4337_bundler_store_pool_size", 0)
	// No default for store_op_ttl_seconds: the shared-store TTL defaults to
	// MAX_OP_TTL_SECONDS unless the operator sets this explicitly, in which case
	// an explicit 0 disables it.

	// Read in from .env file if available
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found
			// Can ignore
		} else {
			panic(fmt.Errorf("fatal error config file: %w", err))
		}
	}

	// Read in from environment variables
	_ = viper.BindEnv("erc4337_bundler_eth_client_url")
	_ = viper.BindEnv("erc4337_bundler_private_key")
	_ = viper.BindEnv("erc4337_bundler_port")
	_ = viper.BindEnv("erc4337_bundler_data_directory")
	_ = viper.BindEnv("erc4337_bundler_supported_entry_points")
	_ = viper.BindEnv("erc4337_bundler_beneficiary")
	_ = viper.BindEnv("erc4337_bundler_native_bundler_collector_tracer")
	_ = viper.BindEnv("erc4337_bundler_native_bundler_executor_tracer")
	_ = viper.BindEnv("erc4337_bundler_max_verification_gas")
	_ = viper.BindEnv("erc4337_bundler_max_batch_gas_limit")
	_ = viper.BindEnv("erc4337_bundler_max_op_ttl_seconds")
	_ = viper.BindEnv("erc4337_bundler_op_lookup_limit")
	_ = viper.BindEnv("erc4337_bundler_eth_builder_urls")
	_ = viper.BindEnv("erc4337_bundler_blocks_in_the_future")
	_ = viper.BindEnv("erc4337_bundler_otel_service_name")
	_ = viper.BindEnv("erc4337_bundler_otel_collector_headers")
	_ = viper.BindEnv("erc4337_bundler_otel_collector_url")
	_ = viper.BindEnv("erc4337_bundler_otel_insecure_mode")
	_ = viper.BindEnv("erc4337_bundler_alt_mempool_ipfs_gateway")
	_ = viper.BindEnv("erc4337_bundler_alt_mempool_ids")
	_ = viper.BindEnv("erc4337_bundler_is_op_stack_network")
	_ = viper.BindEnv("erc4337_bundler_is_arb_stack_network")
	_ = viper.BindEnv("erc4337_bundler_is_rip7212_supported")
	_ = viper.BindEnv("erc4337_bundler_entrypoint_version")
	_ = viper.BindEnv("erc4337_bundler_debug_mode")
	_ = viper.BindEnv("erc4337_bundler_gin_mode")
	_ = viper.BindEnv("erc4337_bundler_store_backend")
	_ = viper.BindEnv("erc4337_bundler_store_url")
	_ = viper.BindEnv("erc4337_bundler_store_namespace")
	_ = viper.BindEnv("erc4337_bundler_store_username")
	_ = viper.BindEnv("erc4337_bundler_store_password")
	_ = viper.BindEnv("erc4337_bundler_store_db")
	_ = viper.BindEnv("erc4337_bundler_store_pool_size")
	_ = viper.BindEnv("erc4337_bundler_store_addrs")
	_ = viper.BindEnv("erc4337_bundler_store_master_name")
	_ = viper.BindEnv("erc4337_bundler_store_op_ttl_seconds")

	// Validate required variables
	if variableNotSetOrIsNil("erc4337_bundler_eth_client_url") {
		panic("Fatal config error: erc4337_bundler_eth_client_url not set")
	}

	if variableNotSetOrIsNil("erc4337_bundler_private_key") {
		panic("Fatal config error: erc4337_bundler_private_key not set")
	}

	if !viper.IsSet("erc4337_bundler_beneficiary") {
		s, err := signer.New(viper.GetString("erc4337_bundler_private_key"))
		if err != nil {
			panic(err)
		}
		viper.SetDefault("erc4337_bundler_beneficiary", s.Address.String())
	}

	switch viper.GetString("mode") {
	case "searcher":
		if variableNotSetOrIsNil("erc4337_bundler_eth_builder_urls") {
			panic("Fatal config error: erc4337_bundler_eth_builder_urls not set")
		}
	}

	// Validate O11Y variables
	if viper.IsSet("erc4337_bundler_otel_service_name") &&
		variableNotSetOrIsNil("erc4337_bundler_otel_collector_url") {
		panic("Fatal config error: erc4337_bundler_otel_service_name is set without a collector URL")
	}

	// Validate Alternative mempool variables
	if viper.IsSet("erc4337_bundler_alt_mempool_ids") &&
		variableNotSetOrIsNil("erc4337_bundler_alt_mempool_ipfs_gateway") {
		panic("Fatal config error: erc4337_bundler_alt_mempool_ids is set without specifying an IPFS gateway")
	}

	// Return Values
	privateKey := viper.GetString("erc4337_bundler_private_key")
	ethClientUrl := viper.GetString("erc4337_bundler_eth_client_url")
	port := viper.GetInt("erc4337_bundler_port")
	dataDirectory := viper.GetString("erc4337_bundler_data_directory")
	supportedEntryPoints := envArrayToAddressSlice(viper.GetString("erc4337_bundler_supported_entry_points"))
	beneficiary := viper.GetString("erc4337_bundler_beneficiary")
	nativeBundlerCollectorTracer := viper.GetString("erc4337_bundler_native_bundler_collector_tracer")
	nativeBundlerExecutorTracer := viper.GetString("erc4337_bundler_native_bundler_executor_tracer")
	maxVerificationGas := big.NewInt(int64(viper.GetInt("erc4337_bundler_max_verification_gas")))
	maxBatchGasLimit := big.NewInt(int64(viper.GetInt("erc4337_bundler_max_batch_gas_limit")))
	maxOpTTL := time.Second * viper.GetDuration("erc4337_bundler_max_op_ttl_seconds")
	opLookupLimit := viper.GetUint64("erc4337_bundler_op_lookup_limit")
	ethBuilderUrls := envArrayToStringSlice(viper.GetString("erc4337_bundler_eth_builder_urls"))
	blocksInTheFuture := viper.GetInt("erc4337_bundler_blocks_in_the_future")
	otelServiceName := viper.GetString("erc4337_bundler_otel_service_name")
	otelCollectorHeader := envKeyValStringToMap(viper.GetString("erc4337_bundler_otel_collector_headers"))
	otelCollectorUrl := viper.GetString("erc4337_bundler_otel_collector_url")
	otelInsecureMode := viper.GetBool("erc4337_bundler_otel_insecure_mode")
	altMempoolIPFSGateway := viper.GetString("erc4337_bundler_alt_mempool_ipfs_gateway")
	altMempoolIds := envArrayToStringSlice(viper.GetString("erc4337_bundler_alt_mempool_ids"))
	isOpStackNetwork := viper.GetBool("erc4337_bundler_is_op_stack_network")
	isArbStackNetwork := viper.GetBool("erc4337_bundler_is_arb_stack_network")
	isRIP7212Supported := viper.GetBool("erc4337_bundler_is_rip7212_supported")
	debugMode := viper.GetBool("erc4337_bundler_debug_mode")
	ginMode := viper.GetString("erc4337_bundler_gin_mode")
	storeBackend := viper.GetString("erc4337_bundler_store_backend")
	storeURL := viper.GetString("erc4337_bundler_store_url")
	storeNamespace := viper.GetString("erc4337_bundler_store_namespace")
	storeUsername := viper.GetString("erc4337_bundler_store_username")
	storePassword := viper.GetString("erc4337_bundler_store_password")
	storeDB := viper.GetInt("erc4337_bundler_store_db")
	storePoolSize := viper.GetInt("erc4337_bundler_store_pool_size")
	storeAddrs := envArrayToStringSlice(viper.GetString("erc4337_bundler_store_addrs"))
	storeMasterName := viper.GetString("erc4337_bundler_store_master_name")
	storeOpTTL := time.Second * viper.GetDuration("erc4337_bundler_store_op_ttl_seconds")
	storeOpTTLSet := viper.IsSet("erc4337_bundler_store_op_ttl_seconds")

	entryPointVersionStr := strings.ToLower(strings.TrimSpace(viper.GetString("erc4337_bundler_entrypoint_version")))
	var entryPointVersion userop.Version
	if entryPointVersionStr == "" || entryPointVersionStr == "auto" {
		if len(supportedEntryPoints) > 0 && supportedEntryPoints[0] != (common.Address{}) {
			entryPointVersion = userop.DetectVersionFromEntryPoint(supportedEntryPoints[0])
		} else {
			entryPointVersion = userop.V0_9
		}
	} else {
		if !userop.IsVersionSupported(entryPointVersionStr) {
			panic(fmt.Sprintf("Fatal config error: unsupported entrypoint version %q", entryPointVersionStr))
		}
		entryPointVersion = userop.Version(entryPointVersionStr)
		// Pin the explicit version for every configured EntryPoint so
		// address-based detection honors the override for non-canonical
		// deployments too.
		for _, ep := range supportedEntryPoints {
			userop.RegisterEntryPointVersion(ep, entryPointVersion)
		}
	}

	// Build the deduped set of versions the instance must serve, one per
	// configured EntryPoint (after any explicit override above). Falls back to
	// the preferred version when no EntryPoints are configured.
	entryPointVersions := dedupeVersions(supportedEntryPoints, entryPointVersion)

	return &Values{
		PrivateKey:                   privateKey,
		EthClientUrl:                 ethClientUrl,
		Port:                         port,
		DataDirectory:                dataDirectory,
		SupportedEntryPoints:         supportedEntryPoints,
		EntryPointVersion:            entryPointVersion,
		EntryPointVersions:           entryPointVersions,
		Beneficiary:                  beneficiary,
		NativeBundlerCollectorTracer: nativeBundlerCollectorTracer,
		NativeBundlerExecutorTracer:  nativeBundlerExecutorTracer,
		MaxVerificationGas:           maxVerificationGas,
		MaxBatchGasLimit:             maxBatchGasLimit,
		MaxOpTTL:                     maxOpTTL,
		OpLookupLimit:                opLookupLimit,
		ReputationConstants:          NewReputationConstantsFromEnv(),
		EthBuilderUrls:               ethBuilderUrls,
		BlocksInTheFuture:            blocksInTheFuture,
		OTELServiceName:              otelServiceName,
		OTELCollectorHeaders:         otelCollectorHeader,
		OTELCollectorUrl:             otelCollectorUrl,
		OTELInsecureMode:             otelInsecureMode,
		AltMempoolIPFSGateway:        altMempoolIPFSGateway,
		AltMempoolIds:                altMempoolIds,
		IsOpStackNetwork:             isOpStackNetwork,
		IsArbStackNetwork:            isArbStackNetwork,
		IsRIP7212Supported:           isRIP7212Supported,
		StoreBackend:                 storeBackend,
		StoreURL:                     storeURL,
		StoreNamespace:               storeNamespace,
		StoreUsername:                storeUsername,
		StorePassword:                storePassword,
		StoreDB:                      storeDB,
		StorePoolSize:                storePoolSize,
		StoreAddrs:                   storeAddrs,
		StoreMasterName:              storeMasterName,
		StoreOpTTL:                   storeOpTTL,
		StoreOpTTLExplicit:           storeOpTTLSet,
		DebugMode:                    debugMode,
		GinMode:                      ginMode,
	}
}
