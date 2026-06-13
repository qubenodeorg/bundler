package start

import (
	"context"
	"fmt"
	"log"

	badger "github.com/dgraph-io/badger/v3"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/zarbanio/bundler/internal/config"
	"github.com/zarbanio/bundler/internal/logger"
	"github.com/zarbanio/bundler/internal/o11y"
	"github.com/zarbanio/bundler/pkg/altmempools"
	"github.com/zarbanio/bundler/pkg/bundler"
	"github.com/zarbanio/bundler/pkg/client"
	"github.com/zarbanio/bundler/pkg/entrypoint/stake"
	"github.com/zarbanio/bundler/pkg/gas"
	"github.com/zarbanio/bundler/pkg/jsonrpc"
	"github.com/zarbanio/bundler/pkg/mempool"
	"github.com/zarbanio/bundler/pkg/modules/batch"
	"github.com/zarbanio/bundler/pkg/modules/checks"
	"github.com/zarbanio/bundler/pkg/modules/entities"
	"github.com/zarbanio/bundler/pkg/modules/expire"
	"github.com/zarbanio/bundler/pkg/modules/gasprice"
	"github.com/zarbanio/bundler/pkg/modules/relay"
	"github.com/zarbanio/bundler/pkg/signer"
	"github.com/zarbanio/bundler/pkg/tracer"

	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
)

func PrivateMode() {
	conf := config.GetValues()

	logr := logger.NewZeroLogr().
		WithName("qubenode_bundler").
		WithValues("bundler_mode", "private")

	logr.Info(
		"tracer configuration",
		"collector", tracer.CollectorName(conf.NativeBundlerCollectorTracer),
		"executor", tracer.ExecutorName(conf.NativeBundlerExecutorTracer),
	)

	eoa, err := signer.New(conf.PrivateKey)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("EOA Address: ", eoa.Address.Hex())
	beneficiary := common.HexToAddress(conf.Beneficiary)

	db, err := badger.Open(badger.DefaultOptions(conf.DataDirectory))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	runDBGarbageCollection(db)

	rpc, err := rpc.Dial(conf.EthClientUrl)
	if err != nil {
		log.Fatal(err)
	}

	eth := ethclient.NewClient(rpc)

	chain, err := eth.ChainID(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	if o11y.IsEnabled(conf.OTELServiceName) {
		o11yOpts := &o11y.Opts{
			ServiceName:     conf.OTELServiceName,
			CollectorHeader: conf.OTELCollectorHeaders,
			CollectorUrl:    conf.OTELCollectorUrl,
			InsecureMode:    conf.OTELInsecureMode,

			ChainID: chain,
			Address: eoa.Address,
		}

		tracerCleanup := o11y.InitTracer(o11yOpts)
		defer tracerCleanup()

		metricsCleanup := o11y.InitMetrics(o11yOpts)
		defer metricsCleanup()
	}

	ov := gas.NewDefaultOverheadForVersion(conf.EntryPointVersion)
	// The PVG estimate is computed for the op as submitted to estimateGas, but
	// the op later sent can differ slightly in byte content (e.g. a different
	// sender deployed in between), shifting the calldata cost by a few gas per
	// byte. A small buffer keeps returned estimates acceptable at send time.
	ov.SetPreVerificationGasBufferFactor(1)
	if conf.IsArbStackNetwork || config.ArbStackChains.Contains(chain.Uint64()) {
		ov.SetCalcPreVerificationGasFunc(gas.CalcArbitrumPVGWithEthClient(rpc, conf.SupportedEntryPoints[0]))
		ov.SetPreVerificationGasBufferFactor(16)
	}

	if conf.IsOpStackNetwork || config.OpStackChains.Contains(chain.Uint64()) {
		ov.SetCalcPreVerificationGasFunc(
			gas.CalcOptimismPVGWithEthClient(rpc, chain, conf.SupportedEntryPoints[0]),
		)
		ov.SetPreVerificationGasBufferFactor(1)
	}

	mem, err := mempool.New(db)
	if err != nil {
		log.Fatal(err)
	}

	alt, err := altmempools.NewFromIPFS(chain, conf.AltMempoolIPFSGateway, conf.AltMempoolIds)
	if err != nil {
		log.Fatal(err)
	}

	check := checks.New(
		db,
		rpc,
		ov,
		alt,
		conf.MaxVerificationGas,
		conf.MaxBatchGasLimit,
		conf.IsRIP7212Supported,
		conf.NativeBundlerCollectorTracer,
		conf.ReputationConstants,
	)

	exp := expire.New(conf.MaxOpTTL)

	relayer := relay.New(eoa, eth, chain, beneficiary, logr)
	relayer.SetRpc(rpc)

	rep := entities.New(db, eth, conf.ReputationConstants)

	// Init Client
	c := client.New(
		mem,
		ov,
		chain,
		conf.SupportedEntryPoints,
		conf.OpLookupLimit,
		eoa,
		eth,
		rpc,
	)
	c.SetGetUserOpReceiptFunc(client.GetUserOpReceiptWithEthClient(eth))
	c.SetGetGasPricesFunc(client.GetGasPricesWithEthClient(eth))
	c.SetGetGasEstimateFunc(
		client.GetGasEstimateWithEthClient(
			rpc,
			ov,
			chain,
			conf.MaxBatchGasLimit,
			conf.NativeBundlerExecutorTracer,
		),
	)
	c.SetGetUserOpByHashFunc(client.GetUserOpByHashWithEthClient(eth))
	c.SetGetStakeFunc(stake.GetStakeWithEthClient(eth))
	c.UseLogger(logr)
	c.UseModules(
		rep.CheckStatus(),
		rep.ValidateOpLimit(),
		check.ValidateOpValues(),
		check.SimulateOp(),
		rep.IncOpsSeen(),
	)

	// Init Bundler
	b := bundler.New(mem, chain, conf.SupportedEntryPoints)
	b.SetGetBaseFeeFunc(gasprice.GetBaseFeeWithEthClient(eth))
	b.SetGetGasTipFunc(gasprice.GetGasTipWithEthClient(eth))
	b.SetGetLegacyGasPriceFunc(gasprice.GetLegacyGasPriceWithEthClient(eth))
	b.UseLogger(logr)
	if err := b.UserMeter(otel.GetMeterProvider().Meter("bundler")); err != nil {
		log.Fatal(err)
	}
	b.UseModules(
		exp.DropExpired(),
		gasprice.SortByGasPrice(),
		gasprice.FilterUnderpriced(),
		batch.SortByNonce(),
		batch.MaintainGasLimit(conf.MaxBatchGasLimit),
		check.PaymasterDeposit(),
		check.SimulateBatch(),
		relayer.SendUserOperation(),
		rep.ProcessDrops(stake.GetStakeWithEthClient(eth)),
		rep.IncOpsIncluded(),
		check.Clean(),
	)
	if err := b.Run(); err != nil {
		log.Fatal(err)
	}
	// Stop the batch loop on shutdown before the DB is closed (deferred db.Close
	// runs after this, since defers unwind LIFO), letting any in-flight batch finish.
	defer b.Stop()

	// init Debug. debug_bundler_sendBundleNow must build one bundle from ALL
	// eligible mempool ops and only return once the bundle transaction is
	// mined, so neither the batch size nor the receipt wait is overridden.
	var d *client.Debug
	if conf.DebugMode {
		d = client.NewDebug(eoa, eth, mem, rep, b, chain, conf.SupportedEntryPoints[0], beneficiary)
	}

	// Init HTTP server
	gin.SetMode(conf.GinMode)
	r := gin.New()
	if err := r.SetTrustedProxies(nil); err != nil {
		log.Fatal(err)
	}
	if o11y.IsEnabled(conf.OTELServiceName) {
		r.Use(otelgin.Middleware(conf.OTELServiceName))
	}
	r.Use(
		cors.Default(),
		logger.WithLogr(logr),
		gin.Recovery(),
	)
	r.GET("/ping", livenessHandler())
	r.GET("/live", livenessHandler())
	r.GET("/ready", readinessHandler(eth, db, logr))
	handlers := []gin.HandlerFunc{
		jsonrpc.Controller(client.NewRpcAdapter(c, d, rpc)),
		jsonrpc.WithOTELTracerAttributes(),
	}
	r.POST("/", handlers...)
	r.POST("/rpc", handlers...)

	if err := serveHTTP(r, conf.Port, conf.DebugMode, logr); err != nil {
		log.Fatal(err)
	}
}
