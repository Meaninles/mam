package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"mare/services/center/internal/agentregistry"
	"mare/services/center/internal/assets"
	"mare/services/center/internal/config"
	"mare/services/center/internal/db"
	httpapi "mare/services/center/internal/http"
	"mare/services/center/internal/importing"
	"mare/services/center/internal/integration"
	"mare/services/center/internal/issues"
	"mare/services/center/internal/jobs"
	"mare/services/center/internal/logging"
	"mare/services/center/internal/notifications"
	"mare/services/center/internal/runtime"
	"mare/services/center/internal/storage"
	"mare/services/center/internal/tags"
	jobdto "mare/shared/contracts/dto/job"
)

type ServerApplication struct {
	config     config.Config
	logger     *slog.Logger
	httpServer *http.Server
	jobService interface {
		Start(context.Context)
	}
	dbPool interface {
		Close()
	}
}

func NewServer(ctx context.Context, cfg config.Config) (*ServerApplication, error) {
	logger := logging.New(cfg.LogLevel).With(
		slog.String("service", cfg.ServiceName),
		slog.String("version", cfg.ServiceVersion),
	)

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	migrator := db.NewMigrator()
	if cfg.AutoMigrate {
		if _, err := migrator.Apply(ctx, pool); err != nil {
			pool.Close()
			return nil, err
		}
	}

	agentService := agentregistry.NewService(pool)
	assetService := assets.NewService(pool)
	tagService := tags.NewService(pool)
	localFolderService := storage.NewLocalFolderService(pool)
	localFolderService.SetAssetService(assetService)
	nasNodeService := storage.NewNASNodeService(pool)
	jobService := jobs.NewService(pool)
	integrationService := integration.NewService(pool)
	integrationService.RegisterProvider(integration.NewCD2115Driver(integrationService))
	integrationService.RegisterDownloader(integration.NewAria2Manager())
	localFolderService.SetIntegrationService(integrationService)
	assetService.SetCloudResolver(integrationService)
	assetService.SetJobRuntime(jobService)
	cloudNodeService := storage.NewCloudNodeService(pool, integrationService)
	importService := importing.NewService(pool, importing.NewHTTPAgentBridge(30*time.Second), jobService, assetService)
	issueService := issues.NewService(pool, jobService)
	notificationService := notifications.NewService(pool)
	jobService.SetIssueSynchronizer(issueService)
	jobService.SetNotificationSynchronizer(notificationService)
	issueService.SetNotificationSynchronizer(notificationService)
	jobService.RegisterExecutor(jobs.JobIntentScanDirectory, func(ctx context.Context, execution jobs.ExecutionContext) error {
		if execution.Job.SourceDomain == jobs.SourceDomainStorageNodes {
			mountID := findMountLinkID(execution.ItemLinks)
			if mountID == nil {
				return errors.New("挂载扫描作业缺少目标挂载关联")
			}
			return localFolderService.RunSingleMountScan(ctx, *mountID)
		}

		target := buildDirectoryScanTarget(execution)
		if target == nil {
			return errors.New("目录扫描作业缺少目录或挂载关联")
		}
		return assetService.ExecuteDirectoryScanTarget(ctx, *target)
	})
	jobService.RegisterExecutor(jobs.JobIntentReplicate, func(ctx context.Context, execution jobs.ExecutionContext) error {
		sourceReplicaID := findReplicaLinkID(execution.ItemLinks)
		targetMountID := findTargetMountLinkID(execution.ItemLinks)
		if sourceReplicaID == nil || targetMountID == nil {
			return errors.New("同步作业缺少源副本或目标挂载关联")
		}
		return assetService.ExecuteReplicaSyncTask(ctx, execution.Job.ID, execution.Item.ID, *sourceReplicaID, *targetMountID)
	})
	jobService.RegisterExecutor(jobs.JobIntentDeleteReplica, func(ctx context.Context, execution jobs.ExecutionContext) error {
		replicaID := findReplicaLinkID(execution.ItemLinks)
		if replicaID == nil {
			return errors.New("副本删除作业缺少副本关联")
		}
		return assetService.ExecuteReplicaDeletionTask(ctx, execution.Job.ID, execution.Item.ID, *replicaID)
	})
	jobService.RegisterExecutor(jobs.JobIntentDeleteAsset, func(ctx context.Context, execution jobs.ExecutionContext) error {
		assetID := findAssetLinkID(execution.ItemLinks)
		if assetID != nil {
			return assetService.ExecuteAssetDeletion(ctx, *assetID)
		}
		directoryID := findDirectoryLinkID(execution.ItemLinks)
		if directoryID != nil {
			return assetService.ExecuteDirectoryDeletion(ctx, *directoryID)
		}
		return errors.New("资产删除作业缺少资产或目录关联")
	})
	jobService.RegisterExecutor(jobs.JobIntentImport, importService.ExecuteImportJobItem)
	runtimeService := runtime.NewService(
		cfg.ServiceName,
		cfg.ServiceVersion,
		time.Now(),
		cfg.HeartbeatTimeout,
		pool,
		migrator,
		agentService,
	)

	router := httpapi.NewRouter(httpapi.Dependencies{
		Logger:        logger,
		Runtime:       runtimeService,
		Agents:        agentService,
		Jobs:          jobService,
		Issues:        issueService,
		Notifications: notificationService,
		Imports:       importService,
		Integrations:  integrationService,
		LocalNodes:    localFolderService,
		NasNodes:      nasNodeService,
		CloudNodes:    cloudNodeService,
		LocalFolders:  localFolderService,
		Assets:        assetService,
		Tags:          tagService,
	})

	return &ServerApplication{
		config: cfg,
		logger: logger,
		httpServer: &http.Server{
			Addr:              cfg.HTTPAddr,
			Handler:           router,
			ReadHeaderTimeout: 5 * time.Second,
		},
		jobService: multiStarter{jobService, integrationService},
		dbPool:     pool,
	}, nil
}

type multiStarter []interface{ Start(context.Context) }

func (m multiStarter) Start(ctx context.Context) {
	for _, starter := range m {
		starter.Start(ctx)
	}
}

func (a *ServerApplication) Run(ctx context.Context) error {
	serverErrors := make(chan error, 1)

	go func() {
		if a.jobService != nil {
			a.jobService.Start(ctx)
		}
		serverErrors <- a.httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return a.httpServer.Shutdown(shutdownCtx)
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (a *ServerApplication) Close(ctx context.Context) error {
	if a.dbPool != nil {
		a.dbPool.Close()
	}
	return nil
}

func RunMigrations(ctx context.Context, cfg config.Config) error {
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	migrator := db.NewMigrator()
	_, err = migrator.Apply(ctx, pool)
	return err
}

func findMountLinkID(links []jobdto.ObjectLinkRecord) *string {
	for _, link := range links {
		if link.ObjectType == jobs.ObjectTypeMount && link.MountID != nil {
			return link.MountID
		}
	}
	return nil
}

func findTargetMountLinkID(links []jobdto.ObjectLinkRecord) *string {
	for _, link := range links {
		if link.ObjectType == jobs.ObjectTypeMount && link.LinkRole == jobs.LinkRoleTargetMount && link.MountID != nil {
			return link.MountID
		}
	}
	return nil
}

func findReplicaLinkID(links []jobdto.ObjectLinkRecord) *string {
	for _, link := range links {
		if link.ObjectType == jobs.ObjectTypeAssetReplica && link.AssetReplicaID != nil {
			return link.AssetReplicaID
		}
	}
	return nil
}

func findAssetLinkID(links []jobdto.ObjectLinkRecord) *string {
	for _, link := range links {
		if link.ObjectType == jobs.ObjectTypeAsset && link.AssetID != nil {
			return link.AssetID
		}
	}
	return nil
}

func findDirectoryLinkID(links []jobdto.ObjectLinkRecord) *string {
	for _, link := range links {
		if link.ObjectType == jobs.ObjectTypeDirectory && link.DirectoryID != nil {
			return link.DirectoryID
		}
	}
	return nil
}

func buildDirectoryScanTarget(execution jobs.ExecutionContext) *assets.DirectoryScanTargetPlan {
	var target assets.DirectoryScanTargetPlan
	for _, link := range execution.ItemLinks {
		if link.ObjectType == jobs.ObjectTypeDirectory && link.DirectoryID != nil {
			target.DirectoryID = *link.DirectoryID
		}
		if link.ObjectType == jobs.ObjectTypeMount && link.MountID != nil {
			target.MountID = *link.MountID
		}
	}
	if target.DirectoryID == "" || target.MountID == "" {
		return nil
	}
	if execution.Job.LibraryID == nil {
		return nil
	}
	target.LibraryID = *execution.Job.LibraryID
	snapshot, ok := execution.Job.SourceSnapshot.(map[string]any)
	if !ok {
		return nil
	}
	libraryName, ok := snapshot["libraryName"].(string)
	if !ok || libraryName == "" {
		return nil
	}
	relativePath, ok := snapshot["relativePath"].(string)
	if !ok || relativePath == "" {
		return nil
	}
	target.LibraryName = libraryName
	target.RelativePath = relativePath
	if execution.Item.TargetPath != nil {
		target.PhysicalPath = *execution.Item.TargetPath
	}
	return &target
}
