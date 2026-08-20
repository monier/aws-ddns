// Composition root only — load configuration, wire dependencies, start the loop.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/route53"

	"github.com/monier/aws-ddns/apps/aws-ddns/internal/framework"
	"github.com/monier/aws-ddns/apps/aws-ddns/internal/process"
	"github.com/monier/aws-ddns/apps/aws-ddns/internal/repositories"
	"github.com/monier/aws-ddns/apps/aws-ddns/internal/services"
)

// version is stamped at build time via -ldflags="-X main.version=<v>".
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	dataDirFlag := flag.String("data-dir", "", "app data folder holding aws-ddns.ini and the state file (default: $DATA_DIR, then /var/lib/aws-ddns; environment variables override file values)")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}

	// Bootstrap logger — structured JSON on stdout from the very first step, so
	// every startup failure is visible in the container logs. Rebuilt with the
	// configured level once the configuration is loaded.
	logger := framework.NewLogger(slog.LevelInfo)
	logger.Info("starting aws-ddns",
		"version", version,
		"goVersion", runtime.Version(),
		"os", runtime.GOOS,
		"arch", runtime.GOARCH,
		"pid", os.Getpid(),
	)

	// Last-resort net: any panic that escapes is recorded in the logs with its
	// stack before the process exits, instead of dying silently.
	defer func() {
		if r := recover(); r != nil {
			logger.Error("fatal panic, shutting down", "panic", r, "stack", string(debug.Stack()))
			os.Exit(1)
		}
	}()

	dataDir, dataDirSource := framework.ResolveDataDir(*dataDirFlag)
	logger.Info("resolving app data folder", "dataDir", dataDir, "source", dataDirSource)
	if err := framework.EnsureDataDir(dataDir); err != nil {
		logger.Error("app data folder is not usable", "dataDir", dataDir, "error", err)
		os.Exit(2)
	}
	logger.Info("app data folder ready", "dataDir", dataDir)

	cfg, err := framework.LoadConfig(dataDir)
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(2)
	}
	logger = framework.NewLogger(cfg.LogLevel)
	logger.Info("configuration loaded",
		"iniFile", cfg.INIPath,
		"iniFound", cfg.INIFound,
		"record", cfg.RecordName,
		"hostedZone", cfg.HostedZoneID,
		"interval", cfg.Interval.String(),
		"ttl", cfg.TTL,
		"logLevel", cfg.LogLevel.String(),
		"stateFile", cfg.StateFile,
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Credentials from the INI file are handed to the SDK explicitly; otherwise
	// the SDK's default chain (environment variables, …) resolves them.
	credentialsSource := "SDK default provider chain"
	awsOptions := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(cfg.AWSRegion)}
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		credentialsSource = "static (INI/environment)"
		awsOptions = append(awsOptions, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsOptions...)
	if err != nil {
		logger.Error("failed to load AWS configuration", "error", err)
		os.Exit(1)
	}
	logger.Info("AWS configuration loaded", "region", cfg.AWSRegion, "credentials", credentialsSource)

	discoverer := repositories.NewHTTPIPDiscoverer(cfg.DiscoveryEndpoints, cfg.HTTPTimeout, logger)
	dnsRepository := repositories.NewRoute53DNSRepository(route53.NewFromConfig(awsCfg), cfg.HostedZoneID)
	stateStore := repositories.NewFileStateStore(cfg.StateFile)
	syncService := services.NewSyncService(discoverer, dnsRepository, stateStore, cfg.RecordName, cfg.TTL, logger)

	logger.Info("synchronization loop starting", "interval", cfg.Interval.String())
	process.NewRunner(syncService, cfg.Interval, logger).Run(ctx)

	logger.Info("aws-ddns stopped")
}
