import { startTransition, useDeferredValue, useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import {
  AlertTriangle,
  ArrowDownToLine,
  Bell,
  CircleEllipsis,
  FolderOpen,
  HardDrive,
  Plus,
  Settings2,
  Star,
} from 'lucide-react';
import type {
  FileTypeFilter,
  ImportDeviceSessionRecord,
  ImportDraftRecord,
  ImportReportSnapshot,
  ImportTargetEndpointRecord,
  IssueRecord,
  Library,
  MainView,
  NoticeJumpTargetKind,
  NoticeRecord,
  SettingsTab,
  Severity,
  StorageNode,
  TaskPriority,
  TaskRecord,
  TaskTab,
} from './data';
import { navigationItems as baseNavigationItems } from './data';
import {
  cloneSettingsRecord,
  resolveDefaultLibraryId,
  resolveStartupWorkspace,
  getDefaultPageSize,
  loadPersistedState,
  resolveThemeMode,
  STORAGE_KEY,
  type PersistedState,
} from './lib/clientState';
import { resolveImportEntrySignal } from './lib/importCenter';
import { createLibrary, loadLibraries } from './lib/librariesApi';
import { importsApi } from './lib/importsApi';
import { getRuntimeConfig } from './lib/runtimeConfig';
import { fetchSystemRuntimeStatus, type SystemRuntimeSummary } from './lib/systemRuntimeApi';
import {
  applyNoticeConsumptions,
  consumeReminderNoticeIds,
  getUnconsumedNoticeCount,
  markNoticeAfterJump,
  markNoticeAsRead,
  pruneNoticeConsumptions,
} from './lib/noticeCenter';
import {
  closeWorkspaceTab,
  DEFAULT_WORKSPACE_VIEW,
  insertWorkspaceTab,
  moveWorkspaceTab,
  reopenLastClosedWorkspace,
  reorderWorkspaceTab,
  type WorkspaceView,
} from './lib/workspaceTabs';
import {
  canDeleteFileCenterEndpoint,
  fileCenterApi,
  resolveFileCenterDeleteAvailability,
  resolveFileCenterSyncAvailability,
  type FileCenterColorLabel,
  type FileCenterDirectoryResult,
  type FileCenterEntry,
  type FileCenterIntegrationHealth,
  type FileCenterSortDirection,
  type FileCenterSortValue,
  type FileCenterStatusFilter,
  type FileCenterTagSuggestion,
} from './lib/fileCenterApi';
import { issuesApi } from './lib/issuesApi';
import { jobsApi, type JobStatus, type JobStreamEvent } from './lib/jobsApi';
import { notificationsApi } from './lib/notificationsApi';
import {
  storageNodesApi,
  type StorageHeartbeatPolicy,
  type StorageMountMode,
  type StorageNodesDashboard,
} from './lib/storageNodesApi';
import { ActionButton, IconButton, LibraryManagerSheet, Sheet } from './components/Shared';
import { WorkspaceTabBar } from './components/WorkspaceTabBar';
import { FileCenterPage } from './pages/FileCenterPage';
import { FileDetailSheet } from './pages/FileDetailSheet';
import { ImportCenterPage, type ImportBrowserNode } from './pages/ImportCenterPage';
import { IssuesPage, type IssueFocusRequest } from './pages/IssuesPage';
import { NotificationCenterSheet } from './pages/NotificationCenterSheet';
import { SettingsPage } from './pages/SettingsPage';
import {
  BackgroundTaskSettingsPanel,
  DependencyServicesSettingsPanel,
  ImportArchiveSettingsPanel,
  IssueGovernanceSettingsPanel,
  NotificationSettingsPanel,
  WorkspaceSettingsPanel,
} from './pages/SettingsPanels';
import { integrationsApi, type RuntimeComponentRecord } from './lib/integrationsApi';
import { StorageNodesPage } from './pages/StorageNodesPage';
import { TagManagementPage } from './pages/TagManagementPage';
import { TaskCenterWorkspace } from './pages/TaskCenterWorkspace';

export type PageSize = 10 | 20 | 50 | 100;
export type ContextMenuTarget =
  | { type: 'file'; item: FileCenterEntry; x: number; y: number }
  | { type: 'task'; item: TaskRecord; x: number; y: number }
  | { type: 'storage'; item: StorageNode; x: number; y: number }
  | null;

type FeedbackState = { message: string; tone: Severity } | null;
type IssueFocusState = IssueFocusRequest;
type PendingFileCenterJump = { libraryId: string; folderId: string | null; selectedIds: string[] } | null;
type StorageFocusState = { id?: string; label: string; path?: string } | null;
type PendingTaskFocus = {
  taskIds: string[];
  issueId?: string;
  taskItemId?: string;
  openIssuePopover?: boolean;
} | null;
type FileConfirmAction =
  | { kind: 'sync'; items: FileCenterEntry[]; endpointName: string; totalSelected: number; guidanceNotes?: string[] }
  | { kind: 'delete-asset'; items: FileCenterEntry[] }
  | {
      kind: 'delete-endpoint';
      items: FileCenterEntry[];
      endpointName: string;
      totalSelected: number;
      willDeleteAssetCount: number;
      guidanceNotes?: string[];
    }
  | {
      kind: 'delete-conflict';
      mode: 'asset' | 'endpoint';
      items: FileCenterEntry[];
      endpointName?: string;
      totalSelected: number;
      willDeleteAssetCount?: number;
      blockingTaskIds: string[];
      blockingTaskTitles: string[];
    };
type TagEditorState = {
  item: FileCenterEntry;
} | null;
type BatchAnnotationState = {
  items: FileCenterEntry[];
} | null;
type BatchTagState = {
  items: FileCenterEntry[];
} | null;
type BatchEndpointAction = {
  endpointName: string;
  enabled: boolean;
  reason?: string;
};
type RuntimeLightState = {
  ariaLabel: string;
  tooltip: string;
  tone: 'success' | 'warning' | 'critical';
} | null;
type ImportDashboardState = {
  libraries: Array<{ id: string; name: string }>;
  devices: ImportDeviceSessionRecord[];
  drafts: ImportDraftRecord[];
  reports: ImportReportSnapshot[];
  targetEndpoints: ImportTargetEndpointRecord[];
};

type ImportBrowserState = {
  sessionId: string;
  currentPath: string;
  items: ImportBrowserNode[];
  total: number;
  hasMore: boolean;
} | null;

const EMPTY_IMPORT_DASHBOARD: ImportDashboardState = {
  libraries: [],
  devices: [],
  drafts: [],
  reports: [],
  targetEndpoints: [],
};

function createEmptyLibraryMountDraft(): LibraryMountDraft {
  return {
    enabled: false,
    mountName: '',
    nodeId: '',
    relativePath: '',
    mountMode: '可写',
    heartbeatPolicy: '从不',
  };
}
type LibrarySourceType = '本地' | 'NAS' | '网盘';
type LibraryNodeOption = {
  id: string;
  label: string;
  sourceType: LibrarySourceType;
};
type LibraryMountDraft = {
  enabled: boolean;
  mountName: string;
  nodeId: string;
  relativePath: string;
  mountMode: StorageMountMode;
  heartbeatPolicy: StorageHeartbeatPolicy;
};
type LibraryCreateState = {
  name: string;
  loadingNodes: boolean;
  saving: boolean;
  errors: Partial<Record<string, string>>;
  mounts: Record<LibrarySourceType, LibraryMountDraft>;
} | null;

function resolveTaskStatusTone(status: string): Severity {
  if (status === '失败') return 'critical';
  if (status === '异常待处理') return 'critical';
  if (status === '运行中') return 'warning';
  if (status === '等待确认') return 'info';
  if (status === '已暂停') return 'info';
  if (status === '已完成') return 'success';
  if (status === '已取消') return 'info';
  if (status === '部分成功') return 'warning';
  return 'info';
}

function applyTaskStatusChange(task: TaskRecord, action: 'pause' | 'resume' | 'retry' | 'cancel'): TaskRecord {
  if (action === 'pause') {
    return {
      ...task,
      status: '已暂停',
      statusTone: resolveTaskStatusTone('已暂停'),
      speed: '—',
      eta: '等待继续',
      updatedAt: '刚刚',
    };
  }

  if (action === 'resume') {
    return {
      ...task,
      status: '运行中',
      statusTone: resolveTaskStatusTone('运行中'),
      eta: task.eta === '等待继续' ? '继续处理中' : task.eta,
      updatedAt: '刚刚',
    };
  }

  if (action === 'retry') {
    return {
      ...task,
      status: '运行中',
      statusTone: resolveTaskStatusTone('运行中'),
      progress: Math.min(task.progress, 18),
      eta: '重新进入队列',
      updatedAt: '刚刚',
    };
  }

  return {
    ...task,
    status: '已取消',
    statusTone: resolveTaskStatusTone('已取消'),
    speed: '—',
    eta: '已取消',
    updatedAt: '刚刚',
  };
}

void applyTaskStatusChange;

function resolveSyncLinkTypeFromTarget(target?: string) {
  if (!target) {
    return 'COPY' as const;
  }
  return target.includes('115') || target.includes('云') ? ('UPLOAD' as const) : ('COPY' as const);
}
void resolveSyncLinkTypeFromTarget;

function resolveTaskItemStatusTone(status: string): Severity {
  if (['失败', '已取消'].includes(status)) return 'critical';
  if (['传输中', '导入中', '校验中', '提交中', '运行中', '扫描中', '解析中', '删除中', '清理中'].includes(status)) return 'warning';
  if (['已暂停', '待执行', '已排队', '待导入', '等待确认', '等待清理'].includes(status)) return 'info';
  if (['已完成', '可执行'].includes(status)) return 'success';
  return 'info';
}

function applyTaskItemStatusChange(item: PersistedState['taskItemRecords'][number], action: 'pause' | 'resume' | 'cancel') {
  if (action === 'pause') {
    return {
      ...item,
      status: '已暂停',
      phase: '已暂停',
      statusTone: resolveTaskItemStatusTone('已暂停'),
      speed: '—',
    };
  }

  if (action === 'resume') {
    const nextStatus = item.kind === 'file' || item.kind === 'folder' ? '传输中' : '传输中';
    return {
      ...item,
      status: nextStatus,
      phase: nextStatus,
      statusTone: resolveTaskItemStatusTone(nextStatus),
    };
  }

  return {
    ...item,
    status: '已取消',
    phase: '已取消',
    statusTone: resolveTaskItemStatusTone('已取消'),
    speed: '—',
  };
}

function applyTaskItemStatusChangeForTask(
  item: PersistedState['taskItemRecords'][number],
  task: TaskRecord,
  action: 'pause' | 'resume' | 'retry' | 'cancel',
) {
  if (task.kind === 'transfer') {
    return applyTaskItemStatusChange(item, action === 'retry' ? 'resume' : action);
  }

  if (action === 'pause') {
    return {
      ...item,
      status: '已暂停',
      phase: '已暂停',
      statusTone: resolveTaskItemStatusTone('已暂停'),
      speed: '—',
    };
  }

  if (action === 'resume' || action === 'retry') {
    const nextPhase = action === 'retry' ? '重新进入队列' : item.phase === '等待时间窗' ? '快速校验' : item.phase ?? '运行中';
    return {
      ...item,
      status: '运行中',
      phase: nextPhase,
      statusTone: resolveTaskItemStatusTone('运行中'),
    };
  }

  return {
    ...item,
    status: '已取消',
    phase: '已取消',
    statusTone: resolveTaskItemStatusTone('已取消'),
    speed: '—',
  };
}

function applyTaskPriorityChange(task: TaskRecord, priority: TaskPriority): TaskRecord {
  return { ...task, priority, updatedAt: '刚刚' };
}

void applyTaskItemStatusChangeForTask;
void applyTaskPriorityChange;

function parseTaskSizeLabel(value: string): number {
  const normalized = value.trim().toUpperCase();
  const numeric = Number.parseFloat(normalized.replace(/[^\d.]/g, ''));
  if (!Number.isFinite(numeric)) {
    return 0;
  }
  if (normalized.includes('TB')) return numeric * 1024 * 1024 * 1024 * 1024;
  if (normalized.includes('GB')) return numeric * 1024 * 1024 * 1024;
  if (normalized.includes('MB')) return numeric * 1024 * 1024;
  if (normalized.includes('KB')) return numeric * 1024;
  return numeric;
}

function formatTaskSizeLabel(bytes: number): string {
  if (bytes >= 1024 * 1024 * 1024) {
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(1).replace(/\.0$/, '')} GB`;
  }
  if (bytes >= 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(1).replace(/\.0$/, '')} MB`;
  }
  if (bytes >= 1024) {
    return `${Math.round(bytes / 1024)} KB`;
  }
  return `${bytes} B`;
}

function isTrackedTransferTaskItem(item: PersistedState['taskItemRecords'][number]) {
  return (item.kind === 'file' || item.kind === undefined) && item.status !== '已取消';
}

function resolveTaskItemSizeLabel(
  item: PersistedState['taskItemRecords'][number],
  fileNodes: PersistedState['fileNodes'],
) {
  if (item.fileNodeId) {
    return fileNodes.find((node) => node.id === item.fileNodeId)?.size ?? item.size ?? '0 B';
  }
  return item.size ?? '0 B';
}

function resolveTaskTotalSizeBytes(task: TaskRecord) {
  if (typeof task.pendingTotalSizeBytes === 'number') {
    return task.pendingTotalSizeBytes;
  }
  if (typeof task.totalSizeBytes === 'number') {
    return task.totalSizeBytes;
  }
  if (task.totalSize) {
    return parseTaskSizeLabel(task.totalSize);
  }
  return 0;
}

function scheduleTransferTaskSizeAdjustment(
  sourceState: PersistedState,
  nextState: PersistedState,
  taskIds: string[],
  taskItemIds: string[],
): PersistedState {
  const canceledTaskIds = new Set(taskIds);
  const canceledTaskItemIds = new Set(taskItemIds);
  const adjustmentByTask = new Map<string, number>();

  sourceState.taskRecords.forEach((task) => {
    if (task.kind !== 'transfer') {
      return;
    }

    const trackedItems = sourceState.taskItemRecords.filter(
      (item) => item.taskId === task.id && isTrackedTransferTaskItem(item),
    );

    if (trackedItems.length === 0) {
      return;
    }

    const bytesToSubtract = canceledTaskIds.has(task.id)
      ? trackedItems.reduce((sum, item) => sum + parseTaskSizeLabel(resolveTaskItemSizeLabel(item, sourceState.fileNodes)), 0)
      : trackedItems
          .filter((item) => canceledTaskItemIds.has(item.id))
          .reduce((sum, item) => sum + parseTaskSizeLabel(resolveTaskItemSizeLabel(item, sourceState.fileNodes)), 0);

    if (bytesToSubtract <= 0) {
      return;
    }

    adjustmentByTask.set(task.id, bytesToSubtract);
  });

  if (adjustmentByTask.size === 0) {
    return nextState;
  }

  return {
    ...nextState,
    taskRecords: nextState.taskRecords.map((task) => {
      const bytesToSubtract = adjustmentByTask.get(task.id);
      if (!bytesToSubtract) {
        return task;
      }

      const nextBytes = Math.max(resolveTaskTotalSizeBytes(task) - bytesToSubtract, 0);
      return {
        ...task,
        totalSize: undefined,
        totalSizeBytes: resolveTaskTotalSizeBytes(task),
        pendingTotalSizeBytes: nextBytes,
      };
    }),
  };
}

function markTransferTasksForFullSizeRecompute(current: PersistedState, taskIds: string[]): PersistedState {
  if (taskIds.length === 0) {
    return current;
  }

  const targetIds = new Set(taskIds);
  return {
    ...current,
    taskRecords: current.taskRecords.map((task) =>
      task.kind === 'transfer' && targetIds.has(task.id)
        ? {
            ...task,
            totalSize: undefined,
            totalSizeBytes: undefined,
            pendingTotalSizeBytes: undefined,
          }
        : task,
    ),
  };
}

void markTransferTasksForFullSizeRecompute;
void scheduleTransferTaskSizeAdjustment;

function isBlockingTransferTaskStatus(status: string) {
  return !['已完成', '已取消', '失败', '部分成功'].includes(status);
}
void isBlockingTransferTaskStatus;

const ACTIVE_TASK_BADGE_STATUSES = new Set<JobStatus>(['RUNNING', 'PAUSED', 'WAITING_CONFIRMATION']);

function shouldShowTaskCenterBadge(status: JobStatus) {
  return ACTIVE_TASK_BADGE_STATUSES.has(status);
}

function shouldShowIssueCenterBadge(status: IssueRecord['status']) {
  return status === '待处理';
}

async function loadAllTaskBadgeJobs() {
  const items: Array<{ status: JobStatus }> = [];
  let page = 1;

  while (true) {
    const result = await jobsApi.list({ page, pageSize: 100 });
    items.push(...result.items);
    if (items.length >= result.total || result.items.length === 0) {
      return items;
    }
    page += 1;
  }
}

function resolveIssueActionFeedback(action: 'retry' | 'confirm' | 'postpone' | 'ignore' | 'refresh' | 'archive', count: number): FeedbackState {
  const quantity = count > 1 ? `${count} 条异常` : '当前异常';

  if (action === 'retry') {
    return { message: `已重新发起 ${quantity} 的处理流程`, tone: 'warning' };
  }
  if (action === 'confirm') {
    return { message: `已标记 ${quantity} 为已确认`, tone: 'success' };
  }
  if (action === 'postpone') {
    return { message: `已延后 ${quantity}`, tone: 'info' };
  }
  if (action === 'ignore') {
    return { message: `已忽略 ${quantity}`, tone: 'info' };
  }
  if (action === 'refresh') {
    return { message: `已刷新 ${quantity} 的检测状态`, tone: 'info' };
  }
  return { message: `已归档 ${quantity}`, tone: 'success' };
}

export default function App() {
  const runtimeConfig = useMemo(() => getRuntimeConfig(), []);
  const [persisted, setPersisted] = useState<PersistedState>(loadPersistedState);
  const [issueRecords, setIssueRecords] = useState<IssueRecord[]>([]);
  const [issuesHydrated, setIssuesHydrated] = useState(false);
  const [remoteNoticeRecords, setRemoteNoticeRecords] = useState<NoticeRecord[]>([]);
  const [libraries, setLibraries] = useState<Library[]>([]);
  const [importDashboard, setImportDashboard] = useState<ImportDashboardState>(EMPTY_IMPORT_DASHBOARD);
  const [importBrowserState, setImportBrowserState] = useState<ImportBrowserState>(null);
  const [importBrowserLoading, setImportBrowserLoading] = useState(false);
  const startupWorkspaceViewRef = useRef<WorkspaceView>(resolveStartupWorkspace(persisted.settings));
  const [openWorkspaceViews, setOpenWorkspaceViews] = useState<WorkspaceView[]>([startupWorkspaceViewRef.current]);
  const [mountedWorkspaceViews, setMountedWorkspaceViews] = useState<WorkspaceView[]>([startupWorkspaceViewRef.current]);
  const [activeWorkspaceView, setActiveWorkspaceView] = useState<WorkspaceView>(startupWorkspaceViewRef.current);
  const [recentlyClosedWorkspaceViews, setRecentlyClosedWorkspaceViews] = useState<WorkspaceView[]>([]);
  const [activeLibraryId, setActiveLibraryId] = useState('');
  const [taskTab, setTaskTab] = useState<TaskTab>('transfer');
  const [settingsTab, setSettingsTab] = useState<SettingsTab>('general');
  const [fileTypeFilter, setFileTypeFilter] = useState<FileTypeFilter>('全部');
  const [fileStatusFilter, setFileStatusFilter] = useState<FileCenterStatusFilter>('全部');
  const [partialSyncEndpointNames, setPartialSyncEndpointNames] = useState<string[]>([]);
  const [taskStatusFilter, setTaskStatusFilter] = useState('活跃中');
  const [issueFocusRequest, setIssueFocusRequest] = useState<IssueFocusState>(null);
  const [storageFocus, setStorageFocus] = useState<StorageFocusState>(null);
  const [searchText, setSearchText] = useState('');
  const [fileSort, setFileSort] = useState<FileCenterSortValue>('修改时间');
  const [fileSortDirection, setFileSortDirection] = useState<FileCenterSortDirection>('desc');
  const deferredSearchText = useDeferredValue(searchText);
  const [pageSize, setPageSize] = useState<PageSize>(() => getDefaultPageSize(persisted.settings));
  const [currentPage, setCurrentPage] = useState(1);
  const [settingsDraft, setSettingsDraft] = useState(() => cloneSettingsRecord(persisted.settings));
  const [cd2GatewayDraft, setCd2GatewayDraft] = useState({
    baseUrl: 'http://localhost:29798',
    username: '',
    password: '',
    enabled: true,
    hasPassword: false,
    runtimeStatus: 'UNKNOWN',
    saving: false,
    testing: false,
  });
  const [cd2Runtime, setCd2Runtime] = useState<RuntimeComponentRecord | null>(null);
  const [aria2Runtime, setAria2Runtime] = useState<RuntimeComponentRecord | null>(null);
  const [currentFolderId, setCurrentFolderId] = useState<string | null>(null);
  const [folderHistory, setFolderHistory] = useState<Array<string | null>>([null]);
  const [historyIndex, setHistoryIndex] = useState(0);
  const [selectedFileIds, setSelectedFileIds] = useState<string[]>([]);
  const [feedback, setFeedback] = useState<FeedbackState>(null);
  const issueRefreshTimeoutRef = useRef<number | null>(null);
  const issueRefreshInFlightRef = useRef(false);
  const issueRefreshDirtyRef = useRef(true);
  const taskBadgeRefreshTimeoutRef = useRef<number | null>(null);
  const fileCenterRefreshTimeoutRef = useRef<number | null>(null);
  const lastIssueSignatureRef = useRef('');
  const [notificationsOpen, setNotificationsOpen] = useState(false);
  const [runtimeLightState, setRuntimeLightState] = useState<RuntimeLightState>(null);
  const [libraryMenuOpen, setLibraryMenuOpen] = useState(false);
  const [managedLibrary, setManagedLibrary] = useState<Library | null>(null);
  const [libraryCreateState, setLibraryCreateState] = useState<LibraryCreateState>(null);
  const [libraryCreateSources, setLibraryCreateSources] = useState<StorageNodesDashboard | null>(null);
  const [storageNodesDashboard, setStorageNodesDashboard] = useState<StorageNodesDashboard | null>(null);
  const [fileDetail, setFileDetail] = useState<FileCenterEntry | null>(null);
  const [fileCenterState, setFileCenterState] = useState<FileCenterDirectoryResult>({
    breadcrumbs: [],
    items: [],
    total: 0,
    currentPathChildren: 0,
    endpointNames: [],
  });
  const [fileCenterLoading, setFileCenterLoading] = useState(true);
  const [fileCenterRefreshing, setFileCenterRefreshing] = useState(false);
  const [loadedFileCenterLocationKey, setLoadedFileCenterLocationKey] = useState<string | null>(null);
  const [fileCenterVersion, setFileCenterVersion] = useState(0);
  const [availableTags, setAvailableTags] = useState<FileCenterTagSuggestion[]>([]);
  const [pendingAction, setPendingAction] = useState<FileConfirmAction | null>(null);
  const [tagEditorState, setTagEditorState] = useState<TagEditorState>(null);
  const [batchAnnotationState, setBatchAnnotationState] = useState<BatchAnnotationState>(null);
  const [batchTagState, setBatchTagState] = useState<BatchTagState>(null);
  const [selectedFileEntries, setSelectedFileEntries] = useState<FileCenterEntry[]>([]);
  const [folderDraft, setFolderDraft] = useState<string | null>(null);
  const [pendingFileCenterJump, setPendingFileCenterJump] = useState<PendingFileCenterJump>(null);
  const [pendingTaskSelection, setPendingTaskSelection] = useState<PendingTaskFocus>(null);
  const [activeTaskBadgeCount, setActiveTaskBadgeCount] = useState(0);
  const [workspaceRefreshTokens, setWorkspaceRefreshTokens] = useState<Record<WorkspaceView, number>>({
    'file-center': 0,
    'import-center': 0,
    'task-center': 0,
    issues: 0,
    'storage-nodes': 0,
    settings: 0,
  });

  const shouldKeepIssuesFresh = true;

  useEffect(() => {
    let cancelled = false;

    const refreshTaskBadgeCount = async () => {
      try {
        const items = await loadAllTaskBadgeJobs();
        if (cancelled) {
          return;
        }
        setActiveTaskBadgeCount(items.filter((item) => shouldShowTaskCenterBadge(item.status)).length);
      } catch (error) {
        if (!cancelled) {
          console.error('load task badge count failed', error);
          setActiveTaskBadgeCount(0);
        }
      }
    };

    void refreshTaskBadgeCount();

    const unsubscribe = jobsApi.subscribe(() => {
      if (taskBadgeRefreshTimeoutRef.current !== null) {
        window.clearTimeout(taskBadgeRefreshTimeoutRef.current);
      }
      taskBadgeRefreshTimeoutRef.current = window.setTimeout(() => {
        taskBadgeRefreshTimeoutRef.current = null;
        void refreshTaskBadgeCount();
      }, 180);
    });

    return () => {
      cancelled = true;
      unsubscribe();
      if (taskBadgeRefreshTimeoutRef.current !== null) {
        window.clearTimeout(taskBadgeRefreshTimeoutRef.current);
      }
    };
  }, []);

  const loadImportDashboard = async (mode: 'load' | 'refresh' = 'load') => {
    if (import.meta.env.MODE === 'test') {
      return null;
    }
    try {
      const next = mode === 'refresh' ? await importsApi.refreshDashboard() : await importsApi.loadDashboard();
      setImportDashboard({
        libraries: next.libraries ?? [],
        devices: next.devices ?? [],
        drafts: next.drafts ?? [],
        reports: next.reports ?? [],
        targetEndpoints: next.targetEndpoints ?? [],
      });
      return next;
    } catch (error) {
      console.error(`load import dashboard failed`, error);
      setImportDashboard(EMPTY_IMPORT_DASHBOARD);
      return null;
    }
  };

  const loadImportBrowser = async (sessionId: string, path?: string) => {
    if (import.meta.env.MODE === 'test') {
      return null;
    }
    try {
      setImportBrowserLoading(true);
      const next = await importsApi.browseSession(sessionId, { path, limit: 200, offset: 0 });
      setImportBrowserState({
        sessionId: next.sessionId,
        currentPath: next.currentPath,
        items: next.items ?? [],
        total: next.total,
        hasMore: next.hasMore,
      });
      return next;
    } catch (error) {
      console.error(`load import browser failed`, error);
      setImportBrowserState(null);
      return null;
    } finally {
      setImportBrowserLoading(false);
    }
  };

  useEffect(() => {
    if (import.meta.env.MODE === 'test') {
      return undefined;
    }

    let disposed = false;

    const syncImportDashboard = async (mode: 'load' | 'refresh' = 'load') => {
      try {
        const next = mode === 'refresh' ? await importsApi.refreshDashboard() : await importsApi.loadDashboard();
        if (disposed) {
          return;
        }
        setImportDashboard({
          libraries: next.libraries ?? [],
          devices: next.devices ?? [],
          drafts: next.drafts ?? [],
          reports: next.reports ?? [],
          targetEndpoints: next.targetEndpoints ?? [],
        });
      } catch (error) {
        if (!disposed) {
          console.error(`refresh import dashboard failed`, error);
          setImportDashboard(EMPTY_IMPORT_DASHBOARD);
        }
      }
    };

    void syncImportDashboard();
    const timerId = window.setInterval(() => {
      void syncImportDashboard('refresh');
    }, 15000);

    return () => {
      disposed = true;
      window.clearInterval(timerId);
    };
  }, []);

  const computeIssueSignature = (items: IssueRecord[]) =>
    items.map((item) => `${item.id}:${item.status}:${item.updatedAt}:${item.histories.length}`).join('|');
  const activeIssueIds = useMemo(
    () =>
      issuesHydrated
        ? issueRecords
            .filter((item) => item.status === '待处理' || item.status === '待确认' || item.status === '处理中')
            .map((item) => item.id)
        : undefined,
    [issueRecords, issuesHydrated],
  );
  const noticeRecords = useMemo(
    () => applyNoticeConsumptions(remoteNoticeRecords, persisted.noticeConsumptions, { activeIssueIds }),
    [activeIssueIds, persisted.noticeConsumptions, remoteNoticeRecords],
  );

  const isIssueRelevantJobEvent = (event: JobStreamEvent) =>
    [
      'JOB_RETRIED',
      'JOB_FAILED',
      'JOB_PARTIAL_SUCCESS',
      'JOB_COMPLETED',
      'JOB_CANCELED',
      'JOB_ITEM_FAILED',
      'JOB_ITEM_COMPLETED',
      'JOB_ITEM_CANCELED',
    ].includes(event.eventType);

  const isFileCenterRelevantJobEvent = (event: JobStreamEvent) =>
    [
      'JOB_COMPLETED',
      'JOB_FAILED',
      'JOB_PARTIAL_SUCCESS',
      'JOB_CANCELED',
      'JOB_ITEM_COMPLETED',
      'JOB_ITEM_FAILED',
      'JOB_ITEM_CANCELED',
    ].includes(event.eventType);

  const refreshIssues = async () => {
    issueRefreshDirtyRef.current = false;
    if (issueRefreshInFlightRef.current) {
      issueRefreshDirtyRef.current = true;
      return;
    }

    issueRefreshInFlightRef.current = true;
    try {
      const items = await issuesApi.listAll();
      const nextSignature = computeIssueSignature(items);
      if (lastIssueSignatureRef.current !== nextSignature) {
        lastIssueSignatureRef.current = nextSignature;
        startTransition(() => {
          setIssueRecords(items);
          setIssuesHydrated(true);
        });
      }
    } catch (error) {
      issueRefreshDirtyRef.current = true;
      if (import.meta.env.MODE !== 'test') {
        console.error('load issues failed', error);
      }
    } finally {
      issueRefreshInFlightRef.current = false;
      if (issueRefreshDirtyRef.current && shouldKeepIssuesFresh) {
        if (issueRefreshTimeoutRef.current !== null) {
          window.clearTimeout(issueRefreshTimeoutRef.current);
        }
        issueRefreshTimeoutRef.current = window.setTimeout(() => {
          issueRefreshTimeoutRef.current = null;
          void refreshIssues();
        }, 240);
      }
    }
  };
  const refreshNotifications = async () => {
    try {
      const items = await notificationsApi.listAll();
      startTransition(() => {
        setRemoteNoticeRecords(items);
      });
      setPersisted((current) => ({
        ...current,
        noticeConsumptions: pruneNoticeConsumptions(current.noticeConsumptions, items.map((item) => item.id)),
      }));
    } catch (error) {
      if (import.meta.env.MODE !== 'test') {
        console.error('load notifications failed', error);
      }
    }
  };

  useEffect(() => {
    let disposed = false;

    void loadLibraries()
      .then((items) => {
        if (disposed) {
          return;
        }
        setLibraries(items);
      })
      .catch(() => {
        if (disposed) {
          return;
        }
        setLibraries([]);
        setFeedback({ message: '加载资产库失败，请稍后重试', tone: 'critical' });
      });

    return () => {
      disposed = true;
    };
  }, []);
  useEffect(() => {
    let disposed = false;
    void refreshNotifications();
    const unsubscribe = notificationsApi.subscribe(() => {
      if (!disposed) {
        void refreshNotifications();
      }
    });

    return () => {
      disposed = true;
      unsubscribe();
    };
  }, []);

  useEffect(() => {
    if (libraries.length === 0) {
      return;
    }

    if (activeLibraryId !== '' && libraries.some((item) => item.id === activeLibraryId)) {
      return;
    }

    setActiveLibraryId(resolveDefaultLibraryId(persisted.settings, libraries, persisted.lastSelectedLibraryId));
  }, [activeLibraryId, libraries, persisted.lastSelectedLibraryId, persisted.settings]);
  const pendingTaskSizeCalcIdsRef = useRef<Set<string>>(new Set());
  const workspaceStageRef = useRef<HTMLDivElement | null>(null);
  const fileCenterScanSequenceRef = useRef(0);
  const workspaceContainersRef = useRef<Partial<Record<WorkspaceView, HTMLDivElement>>>({});
  const activeView: MainView = activeWorkspaceView;
  const unreadNotificationCount = useMemo(() => getUnconsumedNoticeCount(noticeRecords), [noticeRecords]);
  const navigationItems = useMemo(() => {
    const pendingIssueCount = issueRecords.filter((issue) => shouldShowIssueCenterBadge(issue.status)).length;

    return baseNavigationItems.map((item) => {
      if (item.id === 'task-center') {
        return { ...item, badge: activeTaskBadgeCount > 0 ? String(activeTaskBadgeCount) : undefined };
      }
      if (item.id === 'issues') {
        return { ...item, badge: pendingIssueCount > 0 ? String(pendingIssueCount) : undefined };
      }
      if (item.id === 'storage-nodes') {
        return { ...item, badge: undefined };
      }
      return { ...item, badge: undefined };
    });
  }, [activeTaskBadgeCount, issueRecords]);
  const importEntrySignal = useMemo(() => resolveImportEntrySignal(importDashboard.devices), [importDashboard.devices]);
  const workspaceLabels = useMemo(
    () => ({
      'file-center': { title: '文件中心', icon: navIcons['file-center'] },
      'import-center': { title: '导入中心', icon: navIcons['import-center'] },
      'task-center': { title: '任务中心', icon: navIcons['task-center'] },
      issues: { title: '异常中心', icon: navIcons.issues },
      'storage-nodes': { title: '存储节点', icon: navIcons['storage-nodes'] },
      settings: { title: '设置', icon: navIcons.settings },
    }) as Record<WorkspaceView, { icon: React.ReactNode; title: string }>,
      [],
    );

  useEffect(() => {
    if (!runtimeConfig.runtimeStatusEnabled) {
      return undefined;
    }

    let disposed = false;

    const syncRuntimeStatus = async () => {
      const summary = await fetchSystemRuntimeStatus(runtimeConfig.centerBaseUrl);
      if (disposed) {
        return;
      }
      setRuntimeLightState(resolveRuntimeLightState(summary));
    };

    void syncRuntimeStatus();
    const timerId = window.setInterval(() => {
      void syncRuntimeStatus();
    }, runtimeConfig.runtimeStatusPollMs);

    return () => {
      disposed = true;
      window.clearInterval(timerId);
    };
  }, [runtimeConfig]);
  const settingsCustomContent = useMemo(() => {
    if (settingsTab === 'tag-management') {
      return <TagManagementPage libraries={libraries} onFeedback={setFeedback} />;
    }

    if (settingsTab === 'workspace') {
      return (
        <WorkspaceSettingsPanel
          activeViewLabel={workspaceLabels[activeWorkspaceView].title}
          openViews={openWorkspaceViews.map((view) => workspaceLabels[view].title)}
          sections={settingsDraft.workspace}
          onChangeSetting={(sectionId, rowId, value) =>
            setSettingsDraft((current) => ({
              ...current,
              workspace: current.workspace.map((section) =>
                section.id === sectionId
                  ? { ...section, rows: section.rows.map((row) => (row.id === rowId ? { ...row, value } : row)) }
                  : section,
              ),
            }))
          }
        />
      );
    }

    if (settingsTab === 'import-archive') {
      return (
        <ImportArchiveSettingsPanel
          deviceSessions={importDashboard.devices}
          reports={importDashboard.reports}
          sections={settingsDraft['import-archive']}
          onChangeSetting={(sectionId, rowId, value) =>
            setSettingsDraft((current) => ({
              ...current,
              'import-archive': current['import-archive'].map((section) =>
                section.id === sectionId
                  ? { ...section, rows: section.rows.map((row) => (row.id === rowId ? { ...row, value } : row)) }
                  : section,
              ),
            }))
          }
        />
      );
    }

    if (settingsTab === 'dependency-services') {
      return (
        <DependencyServicesSettingsPanel
          aria2Runtime={aria2Runtime}
          cd2Gateway={cd2GatewayDraft}
          cd2Runtime={cd2Runtime}
          onChangeCD2Gateway={(field, value) =>
            setCd2GatewayDraft((current) => ({
              ...current,
              [field]: value,
            }))
          }
          onSaveCD2Gateway={() => {
            setCd2GatewayDraft((current) => ({ ...current, saving: true }));
            void integrationsApi
              .saveCD2Gateway({
                baseUrl: cd2GatewayDraft.baseUrl.trim(),
                username: cd2GatewayDraft.username.trim(),
                password: cd2GatewayDraft.password,
                enabled: true,
              })
              .then(async (result) => {
                try {
                  const verified = await integrationsApi.testCD2Gateway({
                    baseUrl: cd2GatewayDraft.baseUrl.trim(),
                    username: cd2GatewayDraft.username.trim(),
                    password: cd2GatewayDraft.password,
                    enabled: true,
                  });
                  setCd2GatewayDraft((current) => ({
                    ...current,
                    runtimeStatus: verified.record.runtimeStatus,
                    password: '',
                    hasPassword: verified.record.hasPassword,
                    saving: false,
                  }));
                  setCd2Runtime({
                    name: 'CloudDrive2',
                    status: verified.record.runtimeStatus,
                    message:
                      verified.record.runtimeStatus === 'ONLINE'
                        ? 'CloudDrive2 连接正常'
                        : verified.record.lastErrorMessage || 'CloudDrive2 连接异常',
                    lastCheckedAt: verified.record.lastTestAt,
                    lastErrorCode: verified.record.lastErrorCode,
                    lastErrorMessage: verified.record.lastErrorMessage,
                  });
                  setFeedback({ message: `${result.message}，并已确认连接正常`, tone: 'success' });
                } catch (verifyError) {
                  setCd2GatewayDraft((current) => ({
                    ...current,
                    runtimeStatus: result.record.runtimeStatus,
                    password: '',
                    hasPassword: result.record.hasPassword,
                    saving: false,
                  }));
                  setCd2Runtime({
                    name: 'CloudDrive2',
                    status: 'ERROR',
                    message: verifyError instanceof Error ? verifyError.message : 'CloudDrive2 连接异常',
                    lastCheckedAt: result.record.lastTestAt,
                    lastErrorCode: result.record.lastErrorCode,
                    lastErrorMessage: verifyError instanceof Error ? verifyError.message : result.record.lastErrorMessage,
                  });
                  setFeedback({
                    message: verifyError instanceof Error ? `配置已保存，但连接确认失败：${verifyError.message}` : '配置已保存，但连接确认失败',
                    tone: 'critical',
                  });
                }
              })
              .catch((error) => {
                setCd2GatewayDraft((current) => ({ ...current, saving: false }));
                setFeedback({ message: error instanceof Error ? error.message : 'CloudDrive2 保存失败', tone: 'critical' });
              });
          }}
          onTestCD2Gateway={() => {
            setCd2GatewayDraft((current) => ({ ...current, testing: true }));
            void integrationsApi
              .testCD2Gateway({
                baseUrl: cd2GatewayDraft.baseUrl.trim(),
                username: cd2GatewayDraft.username.trim(),
                password: cd2GatewayDraft.password,
                enabled: true,
              })
              .then((result) => {
                setCd2GatewayDraft((current) => ({
                  ...current,
                  runtimeStatus: result.record.runtimeStatus,
                  hasPassword: result.record.hasPassword,
                  testing: false,
                }));
                setCd2Runtime({
                  name: 'CloudDrive2',
                  status: result.record.runtimeStatus,
                  message:
                    result.record.runtimeStatus === 'ONLINE'
                      ? 'CloudDrive2 连接正常'
                      : result.record.lastErrorMessage || 'CloudDrive2 连接异常',
                  lastCheckedAt: result.record.lastTestAt,
                  lastErrorCode: result.record.lastErrorCode,
                  lastErrorMessage: result.record.lastErrorMessage,
                });
                setFeedback({ message: result.message, tone: 'success' });
              })
              .catch((error) => {
                setCd2GatewayDraft((current) => ({ ...current, testing: false }));
                setFeedback({ message: error instanceof Error ? error.message : 'CloudDrive2 测试失败', tone: 'critical' });
              });
          }}
        />
      );
    }

    if (settingsTab === 'notifications') {
      return (
        <NotificationSettingsPanel
          notices={noticeRecords}
          sections={settingsDraft.notifications}
          onChangeSetting={(sectionId, rowId, value) =>
            setSettingsDraft((current) => ({
              ...current,
              notifications: current.notifications.map((section) =>
                section.id === sectionId
                  ? { ...section, rows: section.rows.map((row) => (row.id === rowId ? { ...row, value } : row)) }
                  : section,
              ),
            }))
          }
        />
      );
    }

    if (settingsTab === 'issue-governance') {
      return (
        <IssueGovernanceSettingsPanel
          issues={issueRecords}
          sections={settingsDraft['issue-governance']}
          onChangeSetting={(sectionId, rowId, value) =>
            setSettingsDraft((current) => ({
              ...current,
              'issue-governance': current['issue-governance'].map((section) =>
                section.id === sectionId
                  ? { ...section, rows: section.rows.map((row) => (row.id === rowId ? { ...row, value } : row)) }
                  : section,
              ),
            }))
          }
        />
      );
    }

    if (settingsTab === 'background-tasks') {
      return (
        <BackgroundTaskSettingsPanel
          sections={settingsDraft['background-tasks']}
          tasks={persisted.taskRecords}
          onChangeSetting={(sectionId, rowId, value) =>
            setSettingsDraft((current) => ({
              ...current,
              'background-tasks': current['background-tasks'].map((section) =>
                section.id === sectionId
                  ? { ...section, rows: section.rows.map((row) => (row.id === rowId ? { ...row, value } : row)) }
                  : section,
              ),
            }))
          }
        />
      );
    }

    return null;
  }, [
    activeWorkspaceView,
    openWorkspaceViews,
    importDashboard.devices,
    importDashboard.reports,
    issueRecords,
    libraries,
    noticeRecords,
    persisted.taskRecords,
    settingsDraft,
    settingsTab,
    workspaceLabels,
    aria2Runtime,
    cd2GatewayDraft,
    cd2Runtime,
  ]);

  useEffect(() => {
    if (!mountedWorkspaceViews.includes('settings') && !mountedWorkspaceViews.includes('file-center')) {
      return;
    }
    let cancelled = false;
    const loadDependencyServices = async () => {
      try {
        const [gateways, runtime] = await Promise.all([integrationsApi.loadGateways(), integrationsApi.loadRuntime()]);
        if (cancelled) return;
        const gateway = gateways.find((item) => item.gatewayType === 'CD2');
        const runtimeItem = runtime.find((item) => item.name === 'CloudDrive2') ?? null;
        const aria2RuntimeItem = runtime.find((item) => item.name.toLowerCase() === 'aria2') ?? null;
        setCd2Runtime(runtimeItem);
        setAria2Runtime(aria2RuntimeItem);
        if (gateway) {
          setCd2GatewayDraft((current) => ({
            ...current,
            baseUrl: gateway.baseUrl,
            username: gateway.username ?? '',
            enabled: true,
            hasPassword: gateway.hasPassword,
            runtimeStatus: gateway.runtimeStatus,
          }));
        }
      } catch {
      }
    };
    void loadDependencyServices();
    return () => {
      cancelled = true;
    };
  }, [mountedWorkspaceViews]);

  useEffect(() => {
    if (!mountedWorkspaceViews.includes('file-center')) {
      return;
    }
    let cancelled = false;
    void storageNodesApi
      .loadDashboard()
      .then((dashboard) => {
        if (!cancelled) {
          setStorageNodesDashboard(dashboard);
        }
      })
      .catch(() => {
      });
    return () => {
      cancelled = true;
    };
  }, [mountedWorkspaceViews]);

  useEffect(() => {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(persisted));
  }, [persisted]);

  useEffect(() => {
    if (!activeLibraryId || persisted.lastSelectedLibraryId === activeLibraryId) {
      return;
    }
    setPersisted((current) => ({
      ...current,
      lastSelectedLibraryId: activeLibraryId,
    }));
  }, [activeLibraryId, persisted.lastSelectedLibraryId]);

  useEffect(() => {
    setSettingsDraft(cloneSettingsRecord(persisted.settings));
    setPageSize(getDefaultPageSize(persisted.settings));
  }, [persisted.settings]);

  useEffect(() => {
    if (shouldKeepIssuesFresh && issueRefreshDirtyRef.current) {
      void refreshIssues();
    }

    const unsubscribe = jobsApi.subscribe((event: JobStreamEvent) => {
      if (!isIssueRelevantJobEvent(event)) {
        return;
      }

      issueRefreshDirtyRef.current = true;
      if (issueRefreshTimeoutRef.current !== null) {
        window.clearTimeout(issueRefreshTimeoutRef.current);
      }
      issueRefreshTimeoutRef.current = window.setTimeout(() => {
        issueRefreshTimeoutRef.current = null;
        void refreshIssues();
      }, 240);
    });

    return () => {
      unsubscribe();
      if (issueRefreshTimeoutRef.current !== null) {
        window.clearTimeout(issueRefreshTimeoutRef.current);
      }
    };
  }, [shouldKeepIssuesFresh]);

  useEffect(() => {
    if (!feedback) return;
    const timer = window.setTimeout(() => setFeedback(null), 3000);
    return () => window.clearTimeout(timer);
  }, [feedback]);

  useEffect(() => {
    const transferTasksNeedingSize = persisted.taskRecords.filter(
      (task) => task.kind === 'transfer' && !task.totalSize,
    );
    if (transferTasksNeedingSize.length === 0) {
      return;
    }

    const timers = transferTasksNeedingSize
      .filter((task) => !pendingTaskSizeCalcIdsRef.current.has(task.id))
      .map((task) => {
        pendingTaskSizeCalcIdsRef.current.add(task.id);
        return window.setTimeout(() => {
          setPersisted((current) => ({
            ...current,
            taskRecords: current.taskRecords.map((record) => {
              if (record.id !== task.id || record.totalSize) {
                return record;
              }

              const bytes =
                typeof record.pendingTotalSizeBytes === 'number'
                  ? record.pendingTotalSizeBytes
                  : current.taskItemRecords
                      .filter((item) => item.taskId === task.id && isTrackedTransferTaskItem(item))
                      .reduce((sum, item) => sum + parseTaskSizeLabel(resolveTaskItemSizeLabel(item, current.fileNodes)), 0);

              return {
                ...record,
                totalSize: formatTaskSizeLabel(bytes),
                totalSizeBytes: bytes,
                pendingTotalSizeBytes: undefined,
              };
            }),
          }));
          pendingTaskSizeCalcIdsRef.current.delete(task.id);
        }, 220);
      });

    return () => {
      timers.forEach((timer) => window.clearTimeout(timer));
    };
  }, [persisted.taskItemRecords, persisted.taskRecords]);

  useEffect(() => {
    setCurrentPage(1);
  }, [currentFolderId, deferredSearchText, fileSort, fileStatusFilter, fileTypeFilter, pageSize, partialSyncEndpointNames]);

  useEffect(() => {
    setSelectedFileIds([]);
  }, [activeLibraryId, currentFolderId]);

  useEffect(() => {
    if (!pendingFileCenterJump) {
      return;
    }
    if (activeView !== 'file-center') {
      return;
    }
    if (fileCenterLoading) {
      return;
    }
    if (activeLibraryId !== pendingFileCenterJump.libraryId) {
      return;
    }
    if (currentFolderId !== pendingFileCenterJump.folderId) {
      return;
    }
    setSelectedFileIds(pendingFileCenterJump.selectedIds);
    setPendingFileCenterJump(null);
  }, [activeLibraryId, activeView, currentFolderId, fileCenterLoading, pendingFileCenterJump]);

  useEffect(() => {
    setPartialSyncEndpointNames([]);
  }, [activeLibraryId]);

  useEffect(() => {
    if (selectedFileIds.length === 0) {
      setSelectedFileEntries([]);
      return;
    }

    let disposed = false;
    const visibleMap = new Map(
      fileCenterState.items
        .filter((item) => selectedFileIds.includes(item.id))
        .map((item) => [item.id, item]),
    );

    setSelectedFileEntries((current) => {
      const next = selectedFileIds
        .map((id) => visibleMap.get(id) ?? current.find((item) => item.id === id))
        .filter((item): item is FileCenterEntry => Boolean(item));
      return next;
    });

    void Promise.all(selectedFileIds.map((id) => fileCenterApi.loadEntryDetail(id))).then((items) => {
      if (disposed) {
        return;
      }
      setSelectedFileEntries(items.filter((item): item is FileCenterEntry => Boolean(item)));
    });

    return () => {
      disposed = true;
    };
  }, [fileCenterState.items, fileCenterVersion, selectedFileIds]);

  useEffect(() => {
    if (!activeLibraryId) {
      setAvailableTags([]);
      return;
    }
    void fileCenterApi.loadTagSuggestions('', activeLibraryId).then(setAvailableTags).catch(() => {
      setAvailableTags([]);
    });
  }, [activeLibraryId, fileCenterVersion]);

  const theme = useMemo(() => resolveThemeMode(persisted.settings), [persisted.settings]);
  const currentLibrary = useMemo(
    () =>
      libraries.find((item) => item.id === activeLibraryId) ??
      libraries[0] ?? {
        id: '',
        name: '资产库',
        rootLabel: '/',
        itemCount: '0',
        health: '—',
        storagePolicy: '—',
      },
    [activeLibraryId, libraries],
  );
  const statusFilterEndpointNames = useMemo(
    () => fileCenterState.endpointNames ?? fileCenterApi.listLibraryEndpointNames(activeLibraryId),
    [activeLibraryId, fileCenterState.endpointNames],
  );
  const currentSettingsSections = useMemo(() => {
    const sections = settingsDraft[settingsTab];
    if (settingsTab !== 'general') {
      return sections;
    }
    return sections.map((section) =>
      section.id !== 'launch'
        ? section
        : {
            ...section,
            rows: section.rows.map((row) =>
              row.id !== 'default-library'
                ? row
                : {
                    ...row,
                    options: ['上次使用', ...libraries.map((library) => library.name)],
                  },
            ),
          },
    );
  }, [libraries, settingsDraft, settingsTab]);
  const libraryCreateNodeOptions = useMemo<LibraryNodeOption[]>(
    () =>
      libraryCreateSources
        ? [
            ...libraryCreateSources.localNodes.map((node) => ({
              id: node.id,
              label: `本地 · ${node.name}`,
              sourceType: '本地' as const,
            })),
            ...libraryCreateSources.nasNodes.map((node) => ({
              id: node.id,
              label: `NAS · ${node.name}`,
              sourceType: 'NAS' as const,
            })),
            ...libraryCreateSources.cloudNodes.map((node) => ({
              id: node.id,
              label: `网盘 · ${node.name}`,
              sourceType: '网盘' as const,
            })),
          ]
        : [],
    [libraryCreateSources],
  );
  const breadcrumbs = fileCenterState.breadcrumbs.length > 0
    ? fileCenterState.breadcrumbs
    : [{ id: null as string | null, label: currentLibrary.name }];
  const pageCount = Math.max(1, Math.ceil(fileCenterState.total / pageSize));
  const currentFileCenterLocationKey = `${activeLibraryId}:${currentFolderId ?? 'root'}`;
  const isFileCenterBackgroundLoading =
    fileCenterLoading && loadedFileCenterLocationKey === currentFileCenterLocationKey;

  useEffect(() => {
    if (currentPage > pageCount) {
      setCurrentPage(pageCount);
    }
  }, [currentPage, pageCount]);

  useEffect(() => {
    let disposed = false;
    const requestLocationKey = `${activeLibraryId}:${currentFolderId ?? 'root'}`;
    setFileCenterLoading(true);

    if (!activeLibraryId) {
      setFileCenterState({
        breadcrumbs: [],
        items: [],
        total: 0,
        currentPathChildren: 0,
        endpointNames: [],
      });
      setLoadedFileCenterLocationKey(null);
      setFileCenterLoading(false);
      return () => {
        disposed = true;
      };
    }

    void fileCenterApi
      .loadDirectory({
        libraryId: activeLibraryId,
        parentId: currentFolderId,
        page: currentPage,
        pageSize,
        searchText: deferredSearchText,
        fileTypeFilter,
        statusFilter: fileStatusFilter,
        sortValue: fileSort,
        sortDirection: fileSortDirection,
        partialSyncEndpointNames,
      })
      .then((result) => {
        if (disposed) {
          return;
        }
        setFileCenterState(result);
        setLoadedFileCenterLocationKey(requestLocationKey);
      })
      .catch(() => {
        if (disposed) {
          return;
        }
        setFileCenterState({
          breadcrumbs: [{ id: null, label: currentLibrary.name }],
          items: [],
          total: 0,
          currentPathChildren: 0,
          endpointNames: [],
        });
        setLoadedFileCenterLocationKey(requestLocationKey);
        setFeedback({ message: '加载文件中心失败，请稍后重试', tone: 'critical' });
      })
      .finally(() => {
        if (!disposed) {
          setFileCenterLoading(false);
        }
      });

    return () => {
      disposed = true;
    };
  }, [
    activeLibraryId,
    currentFolderId,
    currentLibrary.name,
    currentPage,
    deferredSearchText,
    fileCenterVersion,
    fileSort,
    fileSortDirection,
    fileStatusFilter,
    fileTypeFilter,
    partialSyncEndpointNames,
    pageSize,
  ]);

  useEffect(() => {
    if (activeView !== 'file-center' || !activeLibraryId) {
      setFileCenterRefreshing(false);
      return;
    }

    const scanSequence = fileCenterScanSequenceRef.current + 1;
    fileCenterScanSequenceRef.current = scanSequence;
    setFileCenterRefreshing(true);

    void fileCenterApi
      .scanDirectory({
        libraryId: activeLibraryId,
        parentId: currentFolderId,
      })
      .then(() => {
        if (fileCenterScanSequenceRef.current !== scanSequence) {
          return;
        }
        setFileCenterVersion((current) => current + 1);
      })
      .catch(() => {
        if (fileCenterScanSequenceRef.current !== scanSequence) {
          return;
        }
        setFeedback({ message: '当前目录自动扫描失败，请稍后重试', tone: 'warning' });
      })
      .finally(() => {
        if (fileCenterScanSequenceRef.current === scanSequence) {
          setFileCenterRefreshing(false);
        }
      });
  }, [activeLibraryId, activeView, currentFolderId]);

  useEffect(() => {
    let disposed = false;
    const unsubscribe = fileCenterApi.subscribe(() => {
      setFileCenterVersion((current) => current + 1);

      if (!fileDetail?.id) {
        return;
      }

      void fileCenterApi.loadEntryDetail(fileDetail.id).then((detail) => {
        if (!disposed) {
          setFileDetail(detail);
        }
      });
    });

    return () => {
      disposed = true;
      unsubscribe();
    };
  }, [fileDetail?.id]);

  useEffect(() => {
    if (activeView !== 'file-center' || !activeLibraryId) {
      if (fileCenterRefreshTimeoutRef.current !== null) {
        window.clearTimeout(fileCenterRefreshTimeoutRef.current);
        fileCenterRefreshTimeoutRef.current = null;
      }
      return;
    }

    let disposed = false;
    const detailId = fileDetail?.id;
    const unsubscribe = jobsApi.subscribe((event: JobStreamEvent) => {
      if (!isFileCenterRelevantJobEvent(event)) {
        return;
      }

      if (fileCenterRefreshTimeoutRef.current !== null) {
        window.clearTimeout(fileCenterRefreshTimeoutRef.current);
      }
      fileCenterRefreshTimeoutRef.current = window.setTimeout(() => {
        fileCenterRefreshTimeoutRef.current = null;
        setFileCenterVersion((current) => current + 1);

        if (!detailId) {
          return;
        }

        void fileCenterApi.loadEntryDetail(detailId).then((detail) => {
          if (!disposed) {
            setFileDetail((current) => (current?.id === detailId ? detail : current));
          }
        });
      }, 240);
    });

    return () => {
      disposed = true;
      unsubscribe();
      if (fileCenterRefreshTimeoutRef.current !== null) {
        window.clearTimeout(fileCenterRefreshTimeoutRef.current);
        fileCenterRefreshTimeoutRef.current = null;
      }
    };
  }, [activeLibraryId, activeView, fileDetail?.id]);
  const commitState = (updater: (current: PersistedState) => PersistedState, nextFeedback?: FeedbackState) => {
    setPersisted((current) => {
      return updater(current);
    });
    if (nextFeedback) setFeedback(nextFeedback);
  };

  const openIssueCenterForIssue = (issue: IssueRecord) => {
    setIssueFocusRequest({
      issueId: issue.id,
      taskId: issue.source.taskId ?? issue.taskId,
      sourceDomain: issue.sourceDomain,
      libraryId: issue.libraryId,
      endpointId: issue.source.endpointId,
      fileNodeId: issue.source.fileNodeId,
      path: issue.source.path,
      label:
        issue.source.taskTitle
          ? `按任务查看异常：${issue.source.taskTitle}`
          : issue.source.endpointLabel
            ? `按来源查看异常：${issue.source.endpointLabel}`
            : `定位异常：${issue.title}`,
    });
    activateWorkspace('issues');
  };

  const openIssueCenterForTask = (task: TaskRecord) => {
    setIssueFocusRequest({
      taskId: task.id,
      sourceDomain: task.kind === 'transfer' ? '传输任务' : '其他任务',
      libraryId: task.libraryId,
      label: `按任务查看异常：${task.title}`,
    });
    activateWorkspace('issues');
  };

  void openIssueCenterForIssue;
  void openIssueCenterForTask;

  const openIssueCenterForIds = (issueIds: string[]) => {
    const issue = issueIds
      .map((issueId) => issueRecords.find((item) => item.id === issueId) ?? null)
      .find((item): item is IssueRecord => Boolean(item));

    if (!issue) {
      activateWorkspace('issues');
      return;
    }

    setIssueFocusRequest({
      issueId: issue.id,
      taskId: issue.source.taskId ?? issue.taskId,
      sourceDomain: issue.sourceDomain,
      libraryId: issue.libraryId,
      endpointId: issue.source.endpointId,
      fileNodeId: issue.source.fileNodeId,
      path: issue.source.path,
      label:
        issue.source.taskTitle
          ? `按任务查看异常：${issue.source.taskTitle}`
          : issue.source.endpointLabel
            ? `按来源查看异常：${issue.source.endpointLabel}`
            : `定位异常：${issue.title}`,
    });
    activateWorkspace('issues');
  };

  const openTaskCenterForIssue = (issue: IssueRecord) => {
    const taskId = issue.source.taskId ?? issue.taskId;
    if (taskId) {
      setTaskTab(issue.sourceDomain === '传输任务' ? 'transfer' : 'other');
      setPendingTaskSelection({
        taskIds: [taskId],
        issueId: issue.id,
        taskItemId: issue.source.taskItemId ?? issue.taskItemId,
        openIssuePopover: true,
      });
      setTaskStatusFilter('全部');
    }
    activateWorkspace('task-center');
  };

  const openTaskCenterById = (taskId?: string) => {
    if (!taskId) {
      activateWorkspace('task-center');
      return;
    }

    setTaskTab('other');
    setPendingTaskSelection({ taskIds: [taskId] });
    setTaskStatusFilter('全部');
    activateWorkspace('task-center');
  };

  const openFileCenterForIssue = async (issue: IssueRecord) => {
    let folderId: string | null = null;
    let selectedIds: string[] = [];

    if (issue.source.fileNodeId) {
      const detail = await fileCenterApi.loadEntryDetail(issue.source.fileNodeId);
      if (detail) {
        folderId = detail.type === 'folder' ? detail.id : detail.parentId;
        selectedIds = [detail.id];
      }
    }

    setActiveLibraryId(issue.libraryId);
    setCurrentFolderId(folderId);
    setFolderHistory([folderId]);
    setHistoryIndex(0);
    setSelectedFileIds(selectedIds);
    setPendingFileCenterJump({
      libraryId: issue.libraryId,
      folderId,
      selectedIds,
    });
    activateWorkspace('file-center');
  };

  const openStorageNodesForIssue = (issue?: IssueRecord) => {
    if (issue?.source.endpointId || issue?.source.path) {
      setStorageFocus({
        id: issue.source.endpointId,
        label: issue.source.endpointLabel ?? issue.title,
        path: issue.source.path,
      });
    }
    activateWorkspace('storage-nodes');
  };

  const openImportCenter = () => {
    activateWorkspace('import-center');
  };

  const openDependencySettings = () => {
    setSettingsTab('dependency-services');
    activateWorkspace('settings');
  };

  const openNotificationCenter = () => {
    const consumedAt = new Date().toISOString();
    setPersisted((current) => ({
      ...current,
      noticeConsumptions: consumeReminderNoticeIds(current.noticeConsumptions, noticeRecords, consumedAt),
    }));
    setNotificationsOpen(true);
  };

  const handleHeaderSignalClick = () => {
    openImportCenter();
  };

  const findNoticeIssue = (notice: NoticeRecord) =>
    notice.issueId ? issueRecords.find((issue) => issue.id === notice.issueId) ?? null : null;

  const handleNoticeMarkRead = (noticeId: string) => {
    const consumedAt = new Date().toISOString();
    setPersisted((current) => ({
      ...current,
      noticeConsumptions: markNoticeAsRead(current.noticeConsumptions, noticeId, consumedAt),
    }));
  };

  const handleNoticeOpenTarget = async (notice: NoticeRecord, targetKind?: NoticeJumpTargetKind) => {
    const issue = findNoticeIssue(notice);
    const nextTargetKind = targetKind ?? notice.jumpParams.kind;
    const consumedAt = new Date().toISOString();

    setPersisted((current) => ({
      ...current,
      noticeConsumptions: markNoticeAfterJump(current.noticeConsumptions, notice, consumedAt),
    }));
    setNotificationsOpen(false);

    if (nextTargetKind === 'import-center') {
      openImportCenter();
      return;
    }

    if (nextTargetKind === 'settings') {
      openDependencySettings();
      return;
    }

    if (nextTargetKind === 'issues') {
      if (issue) {
        setIssueFocusRequest({
          taskId: issue.source.taskId ?? issue.taskId,
          sourceDomain: issue.sourceDomain,
          libraryId: issue.libraryId,
          endpointId: issue.source.endpointId,
          fileNodeId: issue.source.fileNodeId,
          path: issue.source.path,
          label:
            notice.jumpParams.label ??
            (issue.source.taskTitle
              ? `按任务查看异常：${issue.source.taskTitle}`
              : issue.source.endpointLabel
                ? `按来源查看异常：${issue.source.endpointLabel}`
                : `定位异常：${issue.title}`),
        });
        activateWorkspace('issues');
        return;
      }

      setIssueFocusRequest({
        issueId: notice.issueId,
        taskId: notice.jumpParams.taskId,
        sourceDomain: notice.jumpParams.sourceDomain,
        libraryId: notice.jumpParams.libraryId,
        endpointId: notice.jumpParams.endpointId,
        fileNodeId: notice.jumpParams.fileNodeId,
        path: notice.jumpParams.path,
        label: notice.jumpParams.label ?? `定位异常：${notice.title}`,
      });
      activateWorkspace('issues');
      return;
    }

    if (nextTargetKind === 'task-center') {
      if (issue) {
        openTaskCenterForIssue(issue);
      } else {
        if (notice.jumpParams.taskId) {
          setPendingTaskSelection({
            taskIds: [notice.jumpParams.taskId],
            issueId: notice.issueId,
            taskItemId: notice.jumpParams.taskItemId,
          });
          setTaskStatusFilter('全部');
        }
        activateWorkspace('task-center');
      }
      return;
    }

    if (nextTargetKind === 'file-center') {
      if (issue) {
        await openFileCenterForIssue(issue);
      } else {
        let folderId: string | null = null;
        let selectedIds: string[] = [];
        if (notice.jumpParams.fileNodeId) {
          const detail = await fileCenterApi.loadEntryDetail(notice.jumpParams.fileNodeId);
          if (detail) {
            folderId = detail.type === 'folder' ? detail.id : detail.parentId;
            selectedIds = [detail.id];
          } else {
            selectedIds = [notice.jumpParams.fileNodeId];
          }
        }
        if (notice.jumpParams.libraryId) {
          setActiveLibraryId(notice.jumpParams.libraryId);
        }
        setCurrentFolderId(folderId);
        setFolderHistory([folderId]);
        setHistoryIndex(0);
        setSelectedFileIds(selectedIds);
        setPendingFileCenterJump({
          libraryId: notice.jumpParams.libraryId ?? activeLibraryId,
          folderId,
          selectedIds,
        });
        activateWorkspace('file-center');
      }
      return;
    }

    if (nextTargetKind === 'storage-nodes') {
      if (issue) {
        openStorageNodesForIssue(issue);
      } else {
        setStorageFocus({
          id: notice.jumpParams.endpointId,
          label: notice.jumpParams.label ?? notice.title,
          path: notice.jumpParams.path,
        });
        activateWorkspace('storage-nodes');
      }
      return;
    }

    activateWorkspace('issues');
  };

  const rememberClosedWorkspaces = (views: WorkspaceView[]) => {
    if (views.length === 0) {
      return;
    }

    setRecentlyClosedWorkspaceViews((current) => {
      const next = [...views, ...current.filter((view) => !views.includes(view))];
      return next.slice(0, 8);
    });
  };

  const ensureWorkspaceMounted = (view: WorkspaceView) => {
    setMountedWorkspaceViews((current) => (current.includes(view) ? current : [...current, view]));
  };

  const activateWorkspace = (view: WorkspaceView) => {
    ensureWorkspaceMounted(view);
    setOpenWorkspaceViews((current) =>
      current.includes(view) ? current : insertWorkspaceTab(current, activeWorkspaceView, view),
    );
    setRecentlyClosedWorkspaceViews((current) => current.filter((item) => item !== view));
    setActiveWorkspaceView(view);
  };

  const closeWorkspace = (view: WorkspaceView) => {
    if (openWorkspaceViews.length === 1 && openWorkspaceViews[0] === DEFAULT_WORKSPACE_VIEW && view === DEFAULT_WORKSPACE_VIEW) {
      setActiveWorkspaceView(DEFAULT_WORKSPACE_VIEW);
      return;
    }

    const result = closeWorkspaceTab(openWorkspaceViews, activeWorkspaceView, view);
    const nextOrder = result.nextOrder.length > 0 ? result.nextOrder : [DEFAULT_WORKSPACE_VIEW];
    const nextActive = result.nextActive ?? DEFAULT_WORKSPACE_VIEW;

    setOpenWorkspaceViews(nextOrder);
    ensureWorkspaceMounted(nextActive);
    rememberClosedWorkspaces([view]);
    setActiveWorkspaceView(nextActive);
  };

  const closeOtherWorkspaces = (view: WorkspaceView) => {
    const closingViews = openWorkspaceViews.filter((item) => item !== view);
    if (closingViews.length === 0) {
      return;
    }

    setOpenWorkspaceViews([view]);
    rememberClosedWorkspaces([...closingViews].reverse());
    setActiveWorkspaceView(view);
  };

  const closeWorkspaceSide = (view: WorkspaceView, direction: 'left' | 'right') => {
    const currentIndex = openWorkspaceViews.indexOf(view);
    if (currentIndex === -1) {
      return;
    }

    const closingViews =
      direction === 'left'
        ? openWorkspaceViews.slice(0, currentIndex)
        : openWorkspaceViews.slice(currentIndex + 1);

    if (closingViews.length === 0) {
      return;
    }

    setOpenWorkspaceViews(openWorkspaceViews.filter((item) => !closingViews.includes(item)));
    rememberClosedWorkspaces(direction === 'left' ? [...closingViews].reverse() : closingViews);
    if (closingViews.includes(activeWorkspaceView)) {
      setActiveWorkspaceView(view);
    }
  };

  const moveWorkspace = (view: WorkspaceView, direction: 'left' | 'right') => {
    setOpenWorkspaceViews((current) => moveWorkspaceTab(current, view, direction));
  };

  const reopenLastClosedWorkspaceTab = () => {
    const result = reopenLastClosedWorkspace(openWorkspaceViews, recentlyClosedWorkspaceViews);
    if (!result.reopened || !result.nextActive) {
      return;
    }

    ensureWorkspaceMounted(result.reopened);
    setOpenWorkspaceViews(result.nextOrder);
    setRecentlyClosedWorkspaceViews(result.nextClosedStack);
    setActiveWorkspaceView(result.nextActive);
  };

  const reorderWorkspace = (source: WorkspaceView, target: WorkspaceView) => {
    setOpenWorkspaceViews((current) => reorderWorkspaceTab(current, source, target));
  };

  const refreshWorkspace = (view: WorkspaceView) => {
    if (view === 'file-center') {
      setFileCenterVersion((current) => current + 1);
    }

    setWorkspaceRefreshTokens((current) => ({
      ...current,
      [view]: current[view] + 1,
    }));
  };

  const getWorkspaceContainer = (view: WorkspaceView) => {
    const existing = workspaceContainersRef.current[view];
    if (existing) {
      return existing;
    }

    const container = document.createElement('div');
    container.className = 'workspace-view';
    container.dataset.workspaceView = view;
    workspaceContainersRef.current[view] = container;
    return container;
  };

  useEffect(() => {
    const stage = workspaceStageRef.current;
    if (!stage) {
      return;
    }

    stage.replaceChildren();
    stage.appendChild(getWorkspaceContainer(activeWorkspaceView));
  }, [activeWorkspaceView, mountedWorkspaceViews]);

  const openFolder = (folderId: string | null, pushHistory = true) => {
    setCurrentFolderId(folderId);
    if (!pushHistory) return;
    setFolderHistory((current) => {
      const next = current.slice(0, historyIndex + 1);
      next.push(folderId);
      setHistoryIndex(next.length - 1);
      return next;
    });
  };

  const switchLibrary = (libraryId: string) => {
    setActiveLibraryId(libraryId);
    setCurrentFolderId(null);
    setFolderHistory([null]);
    setHistoryIndex(0);
    setSelectedFileIds([]);
    setLibraryMenuOpen(false);
  };

  const openCreateLibraryDialog = () => {
    setLibraryMenuOpen(false);
    setLibraryCreateState({
      name: '',
      loadingNodes: true,
      saving: false,
      errors: {},
      mounts: {
        本地: createEmptyLibraryMountDraft(),
        NAS: createEmptyLibraryMountDraft(),
        网盘: createEmptyLibraryMountDraft(),
      },
    });

    void storageNodesApi
      .loadDashboard()
      .then((dashboard) => {
        setLibraryCreateSources(dashboard);
        setLibraryCreateState((current) =>
          current
            ? {
                ...current,
                loadingNodes: false,
                mounts: {
                  本地: { ...current.mounts['本地'], nodeId: dashboard.localNodes[0]?.id ?? '' },
                  NAS: { ...current.mounts['NAS'], nodeId: dashboard.nasNodes[0]?.id ?? '' },
                  网盘: { ...current.mounts['网盘'], nodeId: dashboard.cloudNodes[0]?.id ?? '' },
                },
              }
            : current,
        );
      })
      .catch((error) => {
        setLibraryCreateSources({
          localNodes: [],
          nasNodes: [],
          cloudNodes: [],
          mounts: [],
          mountFolders: [],
        });
        setLibraryCreateState((current) => (current ? { ...current, loadingNodes: false } : current));
        setFeedback({ message: error instanceof Error ? error.message : '加载存储节点失败', tone: 'critical' });
      });
  };

  const saveCreatedLibrary = async () => {
    if (!libraryCreateState) {
      return;
    }

    const errors: Partial<Record<string, string>> = {};
    if (!libraryCreateState.name.trim()) errors.name = '请输入资产库名称';
    (['本地', 'NAS', '网盘'] as LibrarySourceType[]).forEach((sourceType) => {
      const mount = libraryCreateState.mounts[sourceType];
      if (!mount.enabled) {
        return;
      }
      if (!mount.mountName.trim()) errors[`mountName-${sourceType}`] = '请输入挂载名称';
      if (!mount.nodeId.trim()) errors[`nodeId-${sourceType}`] = '请选择所属节点';
      if (!mount.relativePath.trim()) errors[`relativePath-${sourceType}`] = '请输入挂载子目录';
    });
    if (Object.keys(errors).length > 0) {
      setLibraryCreateState((current) => (current ? { ...current, errors } : current));
      return;
    }

    setLibraryCreateState((current) => (current ? { ...current, saving: true, errors: {} } : current));

    try {
      const created = await createLibrary(libraryCreateState.name.trim());
      const enabledMounts = (['本地', 'NAS', '网盘'] as LibrarySourceType[])
        .map((sourceType) => ({ sourceType, mount: libraryCreateState.mounts[sourceType] }))
        .filter((item) => item.mount.enabled);

      const createdMounts = await Promise.all(
        enabledMounts.map(({ sourceType, mount }) =>
          storageNodesApi.saveMount({
            name: mount.mountName.trim(),
            libraryId: created.library.id,
            libraryName: created.library.name,
            nodeId: mount.nodeId,
            mountMode: mount.mountMode,
            heartbeatPolicy: mount.heartbeatPolicy,
            relativePath: mount.relativePath.trim(),
            notes: '',
            folderType: sourceType,
          }),
        ),
      );
      if (createdMounts.length > 0) {
        await storageNodesApi.runMountScan(createdMounts.map((item) => item.record.id));
      }

      const nextLibraries = await loadLibraries();
      setLibraries(nextLibraries);
      setLibraryCreateState(null);
      setLibraryCreateSources(null);
      switchLibrary(created.library.id);
      activateWorkspace('file-center');
      setFeedback({ message: '资产库与挂载已创建', tone: 'success' });
    } catch (error) {
      setLibraryCreateState((current) => (current ? { ...current, saving: false } : current));
      setFeedback({ message: error instanceof Error ? error.message : '创建资产库失败', tone: 'critical' });
    }
  };

  const getManagedReplicaCount = (item: FileCenterEntry, excludingEndpointName?: string) =>
    item.endpoints.filter(
      (endpoint) =>
        endpoint.endpointType !== 'removable' &&
        endpoint.name !== excludingEndpointName &&
        canDeleteFileCenterEndpoint(endpoint),
    ).length;

  const fileCenterIntegrationHealth = useMemo<FileCenterIntegrationHealth>(() => {
    const healthyCloudNode = storageNodesDashboard?.cloudNodes.find(
      (node) => node.vendor === '115' && node.tone === 'success' && node.status === '鉴权正常',
    );
    const cloudAuthMessage = storageNodesDashboard
      ? healthyCloudNode
        ? undefined
        : '115 节点鉴权未就绪，请先前往存储节点页处理'
      : undefined;
    return {
      cd2Online: isRuntimeComponentOnline(cd2Runtime),
      aria2Online: isRuntimeComponentOnline(aria2Runtime),
      cloudAuthReady: Boolean(healthyCloudNode),
      cd2Message: cd2Runtime?.message,
      aria2Message: aria2Runtime?.message,
      cloudAuthMessage,
    };
  }, [aria2Runtime, cd2Runtime, storageNodesDashboard]);

  const effectiveFileCenterIntegrationHealth = useMemo<FileCenterIntegrationHealth>(() => {
    if (fileCenterIntegrationHealth.cloudAuthReady) {
      return fileCenterIntegrationHealth;
    }

    const cloudNodes = storageNodesDashboard?.cloudNodes ?? [];
    const hasUsable115Node = cloudNodes.some(
      (node) =>
        node.vendor === '115' &&
        (node.tone === 'success' || node.tokenStatus === '已配置' || node.status.includes('鉴权')),
    );

    if (hasUsable115Node || storageNodesDashboard == null) {
      return {
        ...fileCenterIntegrationHealth,
        cloudAuthReady: true,
        cloudAuthMessage: undefined,
      };
    }

    return fileCenterIntegrationHealth;
  }, [fileCenterIntegrationHealth, storageNodesDashboard]);

  const fileCenterCloudActionNotice = useMemo(() => {
    const issues: string[] = [];
    const actions = new Set<'settings' | 'storage-nodes'>();

    if (!effectiveFileCenterIntegrationHealth.cd2Online) {
      issues.push('CloudDrive2 当前不可用');
      actions.add('settings');
    }
    if (!effectiveFileCenterIntegrationHealth.aria2Online) {
      issues.push('aria2 当前不可用，云端下载会受影响');
      actions.add('settings');
    }
    if (!effectiveFileCenterIntegrationHealth.cloudAuthReady) {
      issues.push(effectiveFileCenterIntegrationHealth.cloudAuthMessage ?? '115 节点鉴权未就绪');
      actions.add('storage-nodes');
    }

    if (issues.length === 0) {
      return null;
    }

    return {
      tone: 'warning' as const,
      message: `115 云端动作暂不可用：${issues.join('；')}。上传和删除副本走 CloudDrive2，下载走 aria2，云端传输支持断点续传。`,
      actions: Array.from(actions),
    };
  }, [fileCenterIntegrationHealth]);

  const batchEndpointActions = useMemo(() => {
    const sourceItems = selectedFileEntries.length > 0
      ? selectedFileEntries
      : fileCenterState.items.filter((item) => selectedFileIds.includes(item.id));
    const endpointNames = Array.from(
      new Set(sourceItems.flatMap((item) => item.endpoints.map((endpoint) => endpoint.name))),
    );

    const syncActions: BatchEndpointAction[] = endpointNames.map((endpointName) => {
      const results = sourceItems.map((item) =>
        resolveFileCenterSyncAvailability(item, endpointName, effectiveFileCenterIntegrationHealth),
      );
      const enabled = results.some((result) => result.enabled);
      return {
        endpointName,
        enabled,
        reason: enabled ? undefined : results.find((result) => result.reason)?.reason,
      };
    });

    const deleteActions: BatchEndpointAction[] = endpointNames.map((endpointName) => {
      const results = sourceItems.map((item) =>
          resolveFileCenterDeleteAvailability(item, endpointName, effectiveFileCenterIntegrationHealth),
      );
      const enabled = results.some((result) => result.enabled);
      return {
        endpointName,
        enabled,
        reason: enabled ? undefined : results.find((result) => result.reason)?.reason,
      };
    });

    return {
      syncActions,
      deleteActions,
    };
  }, [effectiveFileCenterIntegrationHealth, fileCenterState.items, selectedFileEntries, selectedFileIds]);

  const selectedActionItems = useMemo(
    () =>
      selectedFileEntries.length > 0
        ? selectedFileEntries
        : fileCenterState.items.filter((item) => selectedFileIds.includes(item.id)),
    [fileCenterState.items, selectedFileEntries, selectedFileIds],
  );

  const requestDeleteAssets = async (ids: string[]) => {
    const targets = (await Promise.all(ids.map((id) => fileCenterApi.loadEntryDetail(id)))).filter(
      (item): item is FileCenterEntry => Boolean(item),
    );
    if (targets.length === 0) {
      setFeedback({ message: '请先选择要删除的资产', tone: 'info' });
      return;
    }
    setPendingAction({ kind: 'delete-asset', items: targets });
  };

  const performDeleteAssets = async (items: FileCenterEntry[], cancelTaskIds: string[] = []) => {
    void cancelTaskIds;
    const result = await fileCenterApi.deleteAssets(items.map((item) => item.id));
    const shouldRefreshDetail = items.some((item) => item.id === fileDetail?.id);
    const updatedItem = shouldRefreshDetail && fileDetail ? await fileCenterApi.loadEntryDetail(fileDetail.id) : null;
    setFeedback({ message: result.message, tone: 'info' });
    setSelectedFileIds([]);
    if (updatedItem) {
      setFileDetail(updatedItem);
    } else if (shouldRefreshDetail) {
      setFileDetail(null);
    }
    setFileCenterVersion((current) => current + 1);
  };

  const requestDeleteEndpoint = (item: FileCenterEntry, endpointName: string) => {
    requestBatchDeleteEndpoint(endpointName, [item]);
  };

  const requestBatchDeleteEndpoint = (endpointName: string, sourceItems = selectedActionItems) => {
    const deleteEvaluations = sourceItems.map((item) => ({
      item,
      availability: resolveFileCenterDeleteAvailability(item, endpointName, effectiveFileCenterIntegrationHealth),
    }));
    const eligibleItems = deleteEvaluations.filter((entry) => entry.availability.enabled).map((entry) => entry.item);

    if (eligibleItems.length === 0) {
      setFeedback({
        message:
          deleteEvaluations.find((entry) => entry.availability.reason)?.availability.reason ??
          `当前所选资产在 ${endpointName} 上没有可删除的副本`,
        tone: 'info',
      });
      return;
    }

    const guidanceNotes = Array.from(
      new Set(
        deleteEvaluations
          .filter((entry) => entry.availability.enabled)
          .map((entry) => entry.availability.execution?.summary)
          .filter((value): value is string => Boolean(value)),
      ),
    );

    setPendingAction({
      kind: 'delete-endpoint',
      endpointName,
      items: eligibleItems,
      totalSelected: sourceItems.length,
      willDeleteAssetCount: eligibleItems.filter((item) => getManagedReplicaCount(item, endpointName) === 0).length,
      guidanceNotes,
    });
  };

  const performDeleteFromEndpoint = async (
    items: FileCenterEntry[],
    endpointName: string,
    cancelTaskIds: string[] = [],
  ) => {
    void cancelTaskIds;
    const shouldRefreshDetail = items.some((item) => item.id === fileDetail?.id);
    const result = await fileCenterApi.deleteFromEndpoint({
      entryIds: items.map((item) => item.id),
      endpointName,
    });
    const updatedItem = shouldRefreshDetail && fileDetail ? await fileCenterApi.loadEntryDetail(fileDetail.id) : null;
    setFeedback({ message: result.message, tone: 'info' });
    setSelectedFileIds([]);
    if (updatedItem) {
      setFileDetail(updatedItem);
    } else if (shouldRefreshDetail) {
      setFileDetail(null);
    }
    setFileCenterVersion((current) => current + 1);
  };

  const requestSyncToEndpoint = (item: FileCenterEntry, endpointName: string) => {
    requestBatchSyncEndpoint(endpointName, [item]);
  };

  const requestBatchSyncEndpoint = (endpointName: string, sourceItems = selectedActionItems) => {
    const syncEvaluations = sourceItems.map((item) => ({
      item,
      availability: resolveFileCenterSyncAvailability(item, endpointName, effectiveFileCenterIntegrationHealth),
    }));
    const eligibleItems = syncEvaluations.filter((entry) => entry.availability.enabled).map((entry) => entry.item);

    if (eligibleItems.length === 0) {
      setFeedback({
        message:
          syncEvaluations.find((entry) => entry.availability.reason)?.availability.reason ??
          `当前所选资产在 ${endpointName} 上没有可同步的副本`,
        tone: 'info',
      });
      return;
    }

    const guidanceNotes = Array.from(
      new Set(
        syncEvaluations
          .filter((entry) => entry.availability.enabled)
          .map((entry) => entry.availability.execution?.summary)
          .filter((value): value is string => Boolean(value)),
      ),
    );

    setPendingAction({
      kind: 'sync',
      items: eligibleItems,
      endpointName,
      totalSelected: sourceItems.length,
      guidanceNotes,
    });
  };

  const performSyncToEndpoint = async (items: FileCenterEntry[], endpointName: string) => {
    const shouldRefreshDetail = items.some((item) => item.id === fileDetail?.id);
    const result = await fileCenterApi.syncToEndpoint({
      entryIds: items.map((item) => item.id),
      endpointName,
    });
    const updatedItem = shouldRefreshDetail && fileDetail ? await fileCenterApi.loadEntryDetail(fileDetail.id) : null;
    setFeedback({ message: result.message, tone: 'info' });
    if (updatedItem) {
      setFileDetail(updatedItem);
    }
    setFileCenterVersion((current) => current + 1);
  };

  const confirmPendingAction = async () => {
    if (!pendingAction) {
      return;
    }

    if (pendingAction.kind === 'sync') {
      await performSyncToEndpoint(pendingAction.items, pendingAction.endpointName);
      setPendingAction(null);
      return;
    }

    if (pendingAction.kind === 'delete-endpoint') {
      await performDeleteFromEndpoint(pendingAction.items, pendingAction.endpointName);
      setPendingAction(null);
      return;
    }

    if (pendingAction.kind === 'delete-conflict') {
      if (pendingAction.mode === 'endpoint' && pendingAction.endpointName) {
        await performDeleteFromEndpoint(pendingAction.items, pendingAction.endpointName, pendingAction.blockingTaskIds);
      } else {
        await performDeleteAssets(pendingAction.items, pendingAction.blockingTaskIds);
      }
      setPendingAction(null);
      return;
    }

    await performDeleteAssets(pendingAction.items);
    setPendingAction(null);
  };

  const saveAnnotations = async (input: {
    id: string;
    rating: number;
    colorLabel: FileCenterColorLabel;
    tags: string[];
  }) => {
    const shouldRefreshDetail = fileDetail?.id === input.id;
    const result = await fileCenterApi.updateAnnotations(input.id, input);
    const updatedItem = await fileCenterApi.loadEntryDetail(input.id);
    setFeedback({ message: result.message, tone: 'success' });
    setAvailableTags(await fileCenterApi.loadTagSuggestions('', activeLibraryId));
    if (updatedItem && shouldRefreshDetail) {
      setFileDetail(updatedItem);
    }
    setFileCenterVersion((current) => current + 1);
  };

  const saveTags = async (item: FileCenterEntry, tags: string[]) => {
    await saveAnnotations({
      id: item.id,
      rating: item.rating,
      colorLabel: item.colorLabel,
      tags,
    });
    setTagEditorState(null);
  };

  const saveBatchAnnotations = async (input: {
    rating: number | null;
    colorLabel: FileCenterColorLabel | null;
  }) => {
    if (!batchAnnotationState || batchAnnotationState.items.length === 0) {
      setFeedback({ message: '请先选择要批量标记的资产', tone: 'info' });
      return;
    }

    if (input.rating === null && input.colorLabel === null) {
      setFeedback({ message: '请至少选择一种要更新的标记', tone: 'warning' });
      return;
    }

    await Promise.all(
      batchAnnotationState.items.map((item) =>
        fileCenterApi.updateAnnotations(item.id, {
          rating: input.rating ?? item.rating,
          colorLabel: input.colorLabel ?? item.colorLabel,
          tags: item.tags,
        }),
      ),
    );

    const shouldRefreshDetail = batchAnnotationState.items.some((item) => item.id === fileDetail?.id);
    const updatedItem = shouldRefreshDetail && fileDetail ? await fileCenterApi.loadEntryDetail(fileDetail.id) : null;
    setFeedback({ message: `已更新 ${batchAnnotationState.items.length} 项资产的标记`, tone: 'success' });
    setAvailableTags(await fileCenterApi.loadTagSuggestions('', activeLibraryId));
    if (updatedItem) {
      setFileDetail(updatedItem);
    }
    setBatchAnnotationState(null);
    setFileCenterVersion((current) => current + 1);
  };

  const saveBatchTags = async (tags: string[]) => {
    if (!batchTagState || batchTagState.items.length === 0) {
      setFeedback({ message: '请先选择要批量设置标签的条目', tone: 'info' });
      return;
    }

    const existingTagUniverse = resolveBatchTagUnion(batchTagState.items);
    const selectedTagSet = new Set(tags);
    const removableTags = existingTagUniverse.filter((tag) => !selectedTagSet.has(tag));

    await Promise.all(
      batchTagState.items.map((item) =>
        fileCenterApi.updateAnnotations(item.id, {
          rating: item.rating,
          colorLabel: item.colorLabel,
          tags: Array.from(
            new Set([
              ...item.tags.filter((tag) => !removableTags.includes(tag)),
              ...tags,
            ]),
          ),
        }),
      ),
    );

    const shouldRefreshDetail = batchTagState.items.some((item) => item.id === fileDetail?.id);
    const updatedItem = shouldRefreshDetail && fileDetail ? await fileCenterApi.loadEntryDetail(fileDetail.id) : null;
    setFeedback({ message: `已更新 ${batchTagState.items.length} 项条目的标签`, tone: 'success' });
    setAvailableTags(await fileCenterApi.loadTagSuggestions('', activeLibraryId));
    if (updatedItem) {
      setFileDetail(updatedItem);
    }
    setBatchTagState(null);
    setFileCenterVersion((current) => current + 1);
  };

  const handleUploadSelection = async (mode: 'files' | 'folder', files: File[]) => {
    try {
      const result = await fileCenterApi.uploadSelection({
        libraryId: activeLibraryId,
        parentId: currentFolderId,
        mode,
        items: files.map((file) => ({
          file,
          name: file.name,
          size: file.size,
          relativePath:
            mode === 'folder'
              ? ((file as File & { webkitRelativePath?: string }).webkitRelativePath || file.name)
              : file.name,
        })),
      });
      setFeedback({ message: result.message, tone: 'success' });
      setFileCenterVersion((current) => current + 1);
    } catch {
      setFeedback({ message: '上传失败，请稍后重试', tone: 'critical' });
    }
  };

  const openEntry = async (item: FileCenterEntry) => {
    if (item.type === 'folder') {
      openFolder(item.id);
      return;
    }
    const detail = await fileCenterApi.loadEntryDetail(item.id);
    if (detail) {
      setFileDetail(detail);
    }
  };

  const openEntryDetail = async (item: FileCenterEntry) => {
    const detail = await fileCenterApi.loadEntryDetail(item.id);
    if (detail) {
      setFileDetail(detail);
    }
  };

  return (
    <div className={`app-shell theme-${theme}`}>
      <aside className="sidebar">
        <div className="brand-area">
          <div className="brand-mark" aria-hidden="true">
            <svg viewBox="0 0 64 64" className="mare-mark">
              <circle cx="32" cy="32" r="30" className="mare-ring" />
              <circle cx="32" cy="32" r="27.6" className="mare-core" />
              <circle cx="45" cy="18" r="7" className="mare-sun top" />
              <circle cx="41.5" cy="47" r="7.5" className="mare-sun bottom" />
              <path className="mare-horizon" d="M8 33c11 0 20-3 28-10 7-6 13-8 20-8v39H8z" />
              <path className="mare-cloud-band" d="M37 23c3-2 7-3 12-3 4 0 6 1 9 3-2 0-4 1-5 3 2 0 4 1 5 3-4-1-8-1-11 0 1-2 0-3-2-4-2-1-4-2-8-2z" />
              <path className="mare-cloud-pattern" d="M40 31c2-1 4-1 6-1 4 0 8 1 12 3-4 0-7 1-9 2 2 0 4 1 5 3-3-1-6-1-9-1-3 0-6 0-9 1 1-2 0-4-2-5 2-1 4-2 6-2z" />
              <path className="mare-mountain" d="M12 46c4-6 7-11 10-15 3 4 5 8 8 15H12z" />
              <path className="mare-wave-deep" d="M8 38c4-7 10-12 18-14 4 0 6 1 8 4 1 1 2 3 2 5 1 4 2 7 5 9 2 2 5 3 9 3 4 0 7-1 10-3v13H8z" />
              <path className="mare-wave-main" d="M10 42c4-5 8-8 13-8 5 0 8 2 10 8 2-10 7-18 15-18 4 0 6 1 8 4-3-1-5 0-6 3-1 2-3 4-6 4-3 0-4-2-5-5-1-2-2-4-4-5 2 0 4-1 5-3 1 2 2 5 3 8 1 4 3 6 6 7v11H10z" />
              <path className="mare-wave-crest" d="M16 24c5-1 10 1 13 7 2-5 5-8 10-8 3 0 6 1 8 4-2 0-3 1-4 3-2-1-4-2-6-2-3 0-5 2-7 6-2 4-4 6-8 6-3 0-5-2-6-5-1-2-2-4-5-4 2-1 4-3 5-7z" />
              <path className="mare-wave-foam" d="M17 26c2 1 4 3 5 6 1 3 3 4 5 4 3 0 4-2 6-7 2-6 6-9 11-9 2 0 4 0 6 1-1 1-2 2-2 4-2 0-3-1-5-1-3 0-5 2-7 6-2 4-4 6-8 6-3 0-5-2-6-5-1-2-2-4-5-4 2 0 4-1 5-3z" />
            </svg>
          </div>
          <div className="brand-copy">
            <strong className="mare-wordmark">MARE</strong>
          </div>
        </div>

        <div className="library-anchor">
          <button
            aria-expanded={libraryMenuOpen}
            aria-haspopup="menu"
            className="library-trigger"
            type="button"
            onClick={() => setLibraryMenuOpen((value) => !value)}
          >
            <div className="library-trigger-copy">
              <strong>{currentLibrary.name}</strong>
            </div>
          </button>
          {libraryMenuOpen ? (
            <div className="library-menu" role="menu">
              {libraries.map((library) => (
                <div className={`library-option${library.id === currentLibrary.id ? ' active' : ''}`} key={library.id}>
                  <button className="library-option-main" type="button" onClick={() => switchLibrary(library.id)}>
                    <div>
                      <strong>{library.name}</strong>
                      <span>{library.itemCount} 项 · {library.storagePolicy}</span>
                    </div>
                  </button>
                  <IconButton ariaLabel={`管理 ${library.name}`} onClick={() => setManagedLibrary(library)}>
                    <Settings2 size={15} />
                  </IconButton>
                </div>
              ))}
              <div className="library-menu-footer">
                <button aria-label="添加资产库" className="library-create-button" type="button" onClick={openCreateLibraryDialog}>
                  <Plus size={15} />
                  <span>添加资产库</span>
                </button>
              </div>
            </div>
          ) : null}
        </div>

        <nav className="sidebar-nav" aria-label="主导航">
          {navigationItems.map((item) => (
            <button
              key={item.id}
              aria-label={item.label}
              className={`nav-item${item.id === activeWorkspaceView ? ' active' : ''}`}
              type="button"
              onClick={() =>
                startTransition(() => {
                  if (item.id === 'issues') {
                    setIssueFocusRequest(null);
                  }
                  activateWorkspace(item.id);
                })
              }
            >
              <span className="nav-icon">{navIcons[item.id]}</span>
              <span className="nav-label">{item.label}</span>
              {item.badge ? <span className="nav-badge">{item.badge}</span> : null}
            </button>
          ))}
          </nav>
        </aside>

      <main className="content-shell">
        <header className="page-header workspace-page-header">
          <div className="page-header-main">
            <div className={`page-header-feedback${feedback ? ` ${feedback.tone}` : ''}`}>
              {feedback ? feedback.message : null}
            </div>
            <WorkspaceTabBar
              activeTab={activeWorkspaceView}
              canReopenClosed={recentlyClosedWorkspaceViews.length > 0}
              labels={workspaceLabels}
              tabs={openWorkspaceViews}
              onActivate={activateWorkspace}
              onClose={closeWorkspace}
              onCloseLeft={(view) => closeWorkspaceSide(view, 'left')}
              onCloseOthers={closeOtherWorkspaces}
              onCloseRight={(view) => closeWorkspaceSide(view, 'right')}
              onMoveLeft={(view) => moveWorkspace(view, 'left')}
              onMoveRight={(view) => moveWorkspace(view, 'right')}
              onRefresh={refreshWorkspace}
              onReopenLastClosed={reopenLastClosedWorkspaceTab}
              onReorder={reorderWorkspace}
            />
          </div>
          <div className="page-header-actions">
            {runtimeLightState ? (
              <span
                aria-label={runtimeLightState.ariaLabel}
                className={`system-runtime-indicator ${runtimeLightState.tone} has-status-tooltip`}
                data-tooltip={runtimeLightState.tooltip}
                data-testid="system-runtime-indicator"
                role="img"
              />
            ) : null}
            <button
              aria-label={importEntrySignal.label}
              className={`import-entry-chip ${importEntrySignal.tone}`}
              type="button"
              onClick={handleHeaderSignalClick}
            >
              <span className="import-entry-dot" aria-hidden="true" />
              <span className="import-entry-text">{importEntrySignal.label}</span>
            </button>
            <div className="notification-trigger">
              <IconButton ariaLabel="通知" onClick={openNotificationCenter}>
                <Bell size={16} />
              </IconButton>
              {unreadNotificationCount > 0 ? (
                <span className="notification-badge" aria-label={`未消费通知 ${unreadNotificationCount} 条`}>
                  {unreadNotificationCount > 99 ? '99+' : unreadNotificationCount}
                </span>
              ) : null}
            </div>
          </div>
        </header>

        <div className="workspace-stage" ref={workspaceStageRef} />

        {mountedWorkspaceViews.includes('file-center') ? (
          createPortal(
            <FileCenterPage
              key={`file-center-${workspaceRefreshTokens['file-center']}`}
            breadcrumbs={breadcrumbs}
            canGoBack={historyIndex > 0}
            canGoForward={historyIndex < folderHistory.length - 1}
            currentEntries={fileCenterState.items}
            currentPage={currentPage}
            currentPathChildren={fileCenterState.currentPathChildren}
            fileTypeFilter={fileTypeFilter}
            loading={!isFileCenterBackgroundLoading && fileCenterLoading}
            pageCount={pageCount}
            pageSize={pageSize}
            partialSyncEndpointNames={partialSyncEndpointNames}
            refreshing={fileCenterRefreshing || isFileCenterBackgroundLoading}
            searchText={searchText}
            selectedIds={selectedFileIds}
            sortValue={fileSort}
            sortDirection={fileSortDirection}
            statusFilterEndpointNames={statusFilterEndpointNames}
            statusFilter={fileStatusFilter}
            theme={theme}
            total={fileCenterState.total}
            cloudActionNotice={fileCenterCloudActionNotice}
            integrationHealth={fileCenterIntegrationHealth}
            onChangeSort={setFileSort}
            onToggleSortDirection={() =>
              setFileSortDirection((current) => (current === 'desc' ? 'asc' : 'desc'))
            }
            batchDeleteEndpointActions={batchEndpointActions.deleteActions}
            batchSyncEndpointActions={batchEndpointActions.syncActions}
            onCreateFolder={() => setFolderDraft('')}
            onDeleteAssetDirect={(item) => void requestDeleteAssets([item.id])}
            onGoBack={() => {
              const nextIndex = Math.max(0, historyIndex - 1);
              setHistoryIndex(nextIndex);
              setCurrentFolderId(folderHistory[nextIndex] ?? null);
            }}
            onGoForward={() => {
              const nextIndex = Math.min(folderHistory.length - 1, historyIndex + 1);
              setHistoryIndex(nextIndex);
              setCurrentFolderId(folderHistory[nextIndex] ?? null);
            }}
            onNavigateBreadcrumb={(index) => openFolder(breadcrumbs[index]?.id ?? null)}
            onOpenItem={(item) => void openEntry(item)}
            onOpenItemDetail={(item) => void openEntryDetail(item)}
            onOpenBatchAnnotationEditor={() => setBatchAnnotationState({ items: selectedActionItems })}
            onOpenBatchTagEditor={() => setBatchTagState({ items: selectedActionItems })}
            onOpenCloudDependencySettings={() => {
              setSettingsTab('dependency-services');
              activateWorkspace('settings');
            }}
            onOpenStorageNodes={() => activateWorkspace('storage-nodes')}
            onOpenTagEditor={(item) => setTagEditorState({ item })}
            onDeleteSelected={() => void requestDeleteAssets(selectedFileIds)}
            onRefreshIndex={() => {
              if (!activeLibraryId) {
                return;
              }
              const scanSequence = fileCenterScanSequenceRef.current + 1;
              fileCenterScanSequenceRef.current = scanSequence;
              setFileCenterRefreshing(true);
              void fileCenterApi
                .scanDirectory({
                  libraryId: activeLibraryId,
                  parentId: currentFolderId,
                })
                .then((result) => {
                  if (fileCenterScanSequenceRef.current !== scanSequence) {
                    return;
                  }
                  setFeedback({ message: result.message, tone: 'info' });
                  setFileCenterVersion((current) => current + 1);
                })
                .catch(() => {
                  if (fileCenterScanSequenceRef.current !== scanSequence) {
                    return;
                  }
                  setFeedback({ message: '索引刷新失败，请稍后重试', tone: 'critical' });
                })
                .finally(() => {
                  if (fileCenterScanSequenceRef.current === scanSequence) {
                    setFileCenterRefreshing(false);
                  }
                });
            }}
            onSetCurrentPage={setCurrentPage}
            onSetFileTypeFilter={setFileTypeFilter}
            onSetPageSize={setPageSize}
            onSetSearchText={(value) => startTransition(() => setSearchText(value))}
            onSetStatusFilter={(value) => {
              setFileStatusFilter(value);
              if (value !== '部分同步') {
                setPartialSyncEndpointNames([]);
              }
            }}
            onClearPartialSyncEndpoints={() => setPartialSyncEndpointNames([])}
            onTogglePartialSyncEndpoint={(endpointName) =>
              setPartialSyncEndpointNames((current) =>
                current.includes(endpointName)
                  ? current.filter((item) => item !== endpointName)
                  : [...current, endpointName],
              )
            }
            onUploadFiles={(files) => void handleUploadSelection('files', files)}
            onUploadFolder={(files) => void handleUploadSelection('folder', files)}
            onRequestBatchDeleteEndpoint={requestBatchDeleteEndpoint}
            onRequestBatchSyncEndpoint={requestBatchSyncEndpoint}
            onRequestDeleteEndpoint={requestDeleteEndpoint}
            onRequestSyncEndpoint={requestSyncToEndpoint}
            onClearSelection={() => setSelectedFileIds([])}
            onToggleSelect={(id) =>
              setSelectedFileIds((current) =>
                current.includes(id) ? current.filter((item) => item !== id) : [...current, id],
              )
            }
            onToggleSelectVisible={() => {
              const ids = fileCenterState.items.map((item) => item.id);
              const allSelected = ids.every((id) => selectedFileIds.includes(id));
              setSelectedFileIds((current) =>
                allSelected ? current.filter((id) => !ids.includes(id)) : Array.from(new Set([...current, ...ids])),
              );
            }}
            />,
            getWorkspaceContainer('file-center'),
          )
        ) : null}

        {mountedWorkspaceViews.includes('import-center')
          ? createPortal(
              <ImportCenterPage
                key={`import-center-${workspaceRefreshTokens['import-center']}`}
                libraries={importDashboard.libraries}
                devices={importDashboard.devices}
                drafts={importDashboard.drafts}
                issues={issueRecords}
                reports={importDashboard.reports}
                targetEndpoints={importDashboard.targetEndpoints}
                browserState={importBrowserState}
                browserLoading={importBrowserLoading}
                onBrowseSession={(deviceSessionId, path) => void loadImportBrowser(deviceSessionId, path)}
                onOpenFolder={(deviceSessionId, path) => void loadImportBrowser(deviceSessionId, path)}
                onGoToParentFolder={(deviceSessionId, currentPath) => {
                  const normalized = currentPath === '/' ? '' : currentPath.replace(/^\/+/, '');
                  const segments = normalized.split('/').filter(Boolean);
                  segments.pop();
                  void loadImportBrowser(deviceSessionId, segments.join('/'));
                }}
                onOpenFileCenter={(libraryId) => {
                  setActiveLibraryId(libraryId);
                  setCurrentFolderId(null);
                  setFolderHistory([null]);
                  setHistoryIndex(0);
                  setSelectedFileIds([]);
                  activateWorkspace('file-center');
                }}
                onOpenIssueCenter={openIssueCenterForIds}
                onOpenStorageNodes={() => activateWorkspace('storage-nodes')}
                onOpenTaskCenter={openTaskCenterById}
                onRefreshDevices={() =>
                  void loadImportDashboard('refresh').then(() => setFeedback({ message: '设备池已刷新', tone: 'info' }))
                }
                onSelectLibrary={(draftId, libraryId) =>
                  void importsApi
                    .setDraftLibrary(draftId, libraryId)
                    .then(async (result) => {
                      await loadImportDashboard();
                      const draft = importDashboard.drafts.find((item) => item.id === draftId);
                      if (draft) {
                        await loadImportBrowser(draft.deviceSessionId);
                      }
                      setFeedback({ message: result.message, tone: 'success' });
                    })
                    .catch((error) => setFeedback({ message: error instanceof Error ? error.message : '切换资产库失败', tone: 'warning' }))
                }
                onRefreshPrecheck={(draftId) =>
                  void importsApi
                    .refreshPrecheck(draftId)
                    .then(async (result) => {
                      await loadImportDashboard();
                      setFeedback({ message: result.message, tone: 'info' });
                    })
                    .catch((error) => setFeedback({ message: error instanceof Error ? error.message : '预检刷新失败', tone: 'warning' }))
                }
                onSaveSelectionTargets={(deviceSessionId, payload) =>
                  void importsApi
                    .saveSelectionTargets(deviceSessionId, payload)
                    .then(async () => {
                      await loadImportDashboard();
                      const currentPath =
                        importBrowserState?.sessionId === deviceSessionId && importBrowserState.currentPath !== '/'
                          ? importBrowserState.currentPath.replace(/^\/+/, '')
                          : undefined;
                      await loadImportBrowser(deviceSessionId, currentPath);
                    })
                    .catch((error) => setFeedback({ message: error instanceof Error ? error.message : '更新导入对象目标端失败', tone: 'warning' }))
                }
                onSubmitImport={(deviceSessionId) =>
                  void importsApi
                    .submit(deviceSessionId)
                    .then(async (result) => {
                      await loadImportDashboard();
                      await loadImportBrowser(deviceSessionId);
                      setFeedback({ message: result.message, tone: 'success' });
                      openTaskCenterById(result.report.taskId);
                    })
                    .catch((error) => setFeedback({ message: error instanceof Error ? error.message : '提交导入失败', tone: 'warning' }))
                }
              />,
              getWorkspaceContainer('import-center'),
            )
          : null}

        {mountedWorkspaceViews.includes('task-center') ? (
          createPortal(
            <TaskCenterWorkspace
              key={`task-center-${workspaceRefreshTokens['task-center']}`}
              activeTab={taskTab}
              visible={activeWorkspaceView === 'task-center'}
              fileNodes={persisted.fileNodes}
              issues={issueRecords}
              libraries={libraries}
              statusFilter={taskStatusFilter}
              preselectedTaskIds={pendingTaskSelection}
              onConsumePreselectedTaskIds={() => setPendingTaskSelection(null)}
              onFeedback={(value) => setFeedback(value)}
              onOpenFileCenterForTask={(task) => {
                setActiveLibraryId(task.libraryId);
                setCurrentFolderId(null);
                setFolderHistory([null]);
                setHistoryIndex(0);
                setSelectedFileIds([]);
                setPendingFileCenterJump({
                  libraryId: task.libraryId,
                  folderId: null,
                  selectedIds: [],
                });
                activateWorkspace('file-center');
              }}
              onOpenIssueCenterForIssue={openIssueCenterForIssue}
              onOpenIssueCenterForTask={openIssueCenterForTask}
              onOpenStorageNodesForTask={() => {
                activateWorkspace('storage-nodes');
              }}
              onSetActiveTab={setTaskTab}
              onSetTaskStatusFilter={setTaskStatusFilter}
            />,
            getWorkspaceContainer('task-center'),
          )
        ) : null}

        {mountedWorkspaceViews.includes('issues') ? (
          createPortal(
            <IssuesPage
              key={`issues-${workspaceRefreshTokens.issues}`}
              issues={issueRecords}
              libraries={libraries}
              focusRequest={issueFocusRequest}
              onClearFocusRequest={() => setIssueFocusRequest(null)}
              onConsumeFocusRequest={() => setIssueFocusRequest(null)}
              onIssueAction={(ids, action) =>
                void issuesApi
                  .applyAction(ids, action)
                  .then(() => {
                    void refreshIssues();
                    void refreshNotifications();
                    setFeedback(resolveIssueActionFeedback(action, ids.length));
                  })
                  .catch((error) =>
                    setFeedback({
                      message: error instanceof Error ? error.message : '异常处理失败',
                      tone: 'warning',
                    }),
                  )
              }
              onClearHistory={(ids) =>
                void issuesApi
                  .clearHistory(ids)
                  .then(() => {
                    void refreshIssues();
                    void refreshNotifications();
                    setFeedback({ message: `已清理 ${ids.length} 条历史异常`, tone: 'success' });
                  })
                  .catch((error) =>
                    setFeedback({
                      message: error instanceof Error ? error.message : '清理历史异常失败',
                      tone: 'warning',
                    }),
                  )
              }
              onOpenTaskCenter={openTaskCenterForIssue}
              onOpenFileCenter={openFileCenterForIssue}
              onOpenSettings={() => openDependencySettings()}
              onOpenStorageNodes={openStorageNodesForIssue}
            />,
            getWorkspaceContainer('issues'),
          )
        ) : null}

        {mountedWorkspaceViews.includes('storage-nodes') ? (
          createPortal(
            <StorageNodesPage
              key={`storage-nodes-${workspaceRefreshTokens['storage-nodes']}`}
              libraries={libraries}
              focusRequest={storageFocus}
              onConsumeFocusRequest={() => setStorageFocus(null)}
              onFeedback={setFeedback}
              onOpenIssueCenter={(context) => {
                setIssueFocusRequest({
                  sourceDomain: '存储节点',
                  endpointId: context.id,
                  path: context.path,
                  label: `按存储节点查看异常：${context.label}`,
                });
                activateWorkspace('issues');
              }}
              onOpenTaskCenter={() => activateWorkspace('task-center')}
            />,
            getWorkspaceContainer('storage-nodes'),
          )
        ) : null}

        {mountedWorkspaceViews.includes('settings') ? (
          createPortal(
            <SettingsPage
              key={`settings-${workspaceRefreshTokens.settings}`}
              customContent={settingsCustomContent}
              sections={currentSettingsSections}
              settingsTab={settingsTab}
              setSettingsTab={setSettingsTab}
              onChangeSetting={(sectionId, rowId, value) =>
                setSettingsDraft((current) => ({
                  ...current,
                  [settingsTab]: current[settingsTab].map((section) =>
                    section.id === sectionId
                      ? { ...section, rows: section.rows.map((row) => (row.id === rowId ? { ...row, value } : row)) }
                      : section,
                  ),
                }))
              }
              onResetSettings={() => setSettingsDraft(cloneSettingsRecord(loadPersistedState().settings))}
              onSaveSettings={() =>
                commitState((current) => ({ ...current, settings: cloneSettingsRecord(settingsDraft) }), {
                  message: '设置已保存',
                  tone: 'success',
                })
              }
            />,
            getWorkspaceContainer('settings'),
          )
        ) : null}
      </main>

      {activeView === 'file-center' && fileDetail ? (
        <FileDetailSheet
          item={fileDetail}
          onClose={() => setFileDetail(null)}
          onSaveAnnotations={saveAnnotations}
        />
      ) : null}

      {activeView === 'file-center' && tagEditorState ? (
        <TagEditorDialog
          availableTags={availableTags}
          item={tagEditorState.item}
          onClose={() => setTagEditorState(null)}
          onSave={(tags) => void saveTags(tagEditorState.item, tags)}
        />
      ) : null}

      {activeView === 'file-center' && batchAnnotationState ? (
        <BatchAnnotationDialog
          count={batchAnnotationState.items.length}
          onClose={() => setBatchAnnotationState(null)}
          onSave={(input) => void saveBatchAnnotations(input)}
        />
      ) : null}

      {activeView === 'file-center' && batchTagState ? (
        <BatchTagDialog
          availableTags={availableTags}
          items={batchTagState.items}
          onClose={() => setBatchTagState(null)}
          onSave={(tags) => void saveBatchTags(tags)}
        />
      ) : null}

      {libraryCreateState ? (
        <div className="dialog-backdrop" role="presentation" onClick={() => { setLibraryCreateState(null); setLibraryCreateSources(null); }}>
          <section
            aria-label="新增资产库"
            className="dialog-panel"
            role="dialog"
            onClick={(event) => event.stopPropagation()}
          >
            <div className="sheet-header">
              <strong>新增资产库</strong>
              <IconButton ariaLabel="关闭" onClick={() => { setLibraryCreateState(null); setLibraryCreateSources(null); }}>
                <CircleEllipsis size={15} />
              </IconButton>
            </div>
            <div className="sheet-section">
              <label className="form-field">
                <span>资产库名称</span>
                <input
                  aria-label="资产库名称"
                  value={libraryCreateState.name}
                  onChange={(event) =>
                    setLibraryCreateState((current) =>
                      current ? { ...current, name: event.target.value, errors: { ...current.errors, name: undefined } } : current,
                    )
                  }
                />
                {libraryCreateState.errors.name ? <small className="field-error">{libraryCreateState.errors.name}</small> : null}
              </label>
            </div>
            <div className="sheet-section">
              <strong>挂载文件夹配置</strong>
              <p className="muted-paragraph">可分别为本地、NAS、网盘配置挂载，也可以只配置其中一部分。</p>
            </div>
            {(['本地', 'NAS', '网盘'] as LibrarySourceType[]).map((sourceType) => {
              const mount = libraryCreateState.mounts[sourceType];
              const options = libraryCreateNodeOptions.filter((item) => item.sourceType === sourceType);
              return (
                <div className="sheet-section" key={sourceType}>
                  <div className="row-actions">
                    <strong>{sourceType} 挂载</strong>
                    <ActionButton
                      onClick={() =>
                        setLibraryCreateState((current) =>
                          current
                            ? {
                                ...current,
                                mounts: {
                                  ...current.mounts,
                                  [sourceType]: {
                                    ...current.mounts[sourceType],
                                    enabled: !current.mounts[sourceType].enabled,
                                  },
                                },
                              }
                            : current,
                        )
                      }
                    >
                      {mount.enabled ? '已启用' : '不配置'}
                    </ActionButton>
                  </div>
                  {mount.enabled ? (
                    <div className="sheet-form">
                      <label className="form-field">
                        <span>挂载名称</span>
                        <input
                          aria-label={`${sourceType}挂载名称`}
                          value={mount.mountName}
                          onChange={(event) =>
                            setLibraryCreateState((current) =>
                              current
                                ? {
                                    ...current,
                                    mounts: {
                                      ...current.mounts,
                                      [sourceType]: { ...current.mounts[sourceType], mountName: event.target.value },
                                    },
                                    errors: { ...current.errors, [`mountName-${sourceType}`]: undefined },
                                  }
                                : current,
                            )
                          }
                        />
                        {libraryCreateState.errors[`mountName-${sourceType}`] ? (
                          <small className="field-error">{libraryCreateState.errors[`mountName-${sourceType}`]}</small>
                        ) : null}
                      </label>
                      <label className="form-field">
                        <span>所属节点</span>
                        <select
                          aria-label={`${sourceType}所属节点`}
                          disabled={libraryCreateState.loadingNodes || options.length === 0}
                          value={mount.nodeId}
                          onChange={(event) =>
                            setLibraryCreateState((current) =>
                              current
                                ? {
                                    ...current,
                                    mounts: {
                                      ...current.mounts,
                                      [sourceType]: { ...current.mounts[sourceType], nodeId: event.target.value },
                                    },
                                    errors: { ...current.errors, [`nodeId-${sourceType}`]: undefined },
                                  }
                                : current,
                            )
                          }
                        >
                          {options.length === 0 ? (
                            <option value="">暂无可用节点</option>
                          ) : (
                            options.map((option) => (
                              <option key={option.id} value={option.id}>
                                {option.label}
                              </option>
                            ))
                          )}
                        </select>
                        {libraryCreateState.errors[`nodeId-${sourceType}`] ? (
                          <small className="field-error">{libraryCreateState.errors[`nodeId-${sourceType}`]}</small>
                        ) : null}
                      </label>
                      <label className="form-field">
                        <span>挂载子目录</span>
                        <input
                          aria-label={`${sourceType}挂载子目录`}
                          value={mount.relativePath}
                          onChange={(event) =>
                            setLibraryCreateState((current) =>
                              current
                                ? {
                                    ...current,
                                    mounts: {
                                      ...current.mounts,
                                      [sourceType]: { ...current.mounts[sourceType], relativePath: event.target.value },
                                    },
                                    errors: { ...current.errors, [`relativePath-${sourceType}`]: undefined },
                                  }
                                : current,
                            )
                          }
                        />
                        {libraryCreateState.errors[`relativePath-${sourceType}`] ? (
                          <small className="field-error">{libraryCreateState.errors[`relativePath-${sourceType}`]}</small>
                        ) : null}
                      </label>
                      <label className="form-field">
                        <span>挂载模式</span>
                        <select
                          aria-label={`${sourceType}挂载模式`}
                          value={mount.mountMode}
                          onChange={(event) =>
                            setLibraryCreateState((current) =>
                              current
                                ? {
                                    ...current,
                                    mounts: {
                                      ...current.mounts,
                                      [sourceType]: { ...current.mounts[sourceType], mountMode: event.target.value as StorageMountMode },
                                    },
                                  }
                                : current,
                            )
                          }
                        >
                          {(['可写', '只读'] as StorageMountMode[]).map((mode) => (
                            <option key={mode} value={mode}>
                              {mode}
                            </option>
                          ))}
                        </select>
                      </label>
                      <label className="form-field">
                        <span>心跳策略</span>
                        <select
                          aria-label={`${sourceType}心跳策略`}
                          value={mount.heartbeatPolicy}
                          onChange={(event) =>
                            setLibraryCreateState((current) =>
                              current
                                ? {
                                    ...current,
                                    mounts: {
                                      ...current.mounts,
                                      [sourceType]: { ...current.mounts[sourceType], heartbeatPolicy: event.target.value as StorageHeartbeatPolicy },
                                    },
                                  }
                                : current,
                            )
                          }
                        >
                          {(['从不', '每周（深夜）', '每日（深夜）', '每小时'] as StorageHeartbeatPolicy[]).map((policy) => (
                            <option key={policy} value={policy}>
                              {policy}
                            </option>
                          ))}
                        </select>
                      </label>
                    </div>
                  ) : null}
                </div>
              );
            })}
            <div className="sheet-actions right">
              <ActionButton onClick={() => { setLibraryCreateState(null); setLibraryCreateSources(null); }}>取消</ActionButton>
              <ActionButton tone="primary" onClick={() => void saveCreatedLibrary()}>
                {libraryCreateState.saving ? '创建中...' : '创建资产库'}
              </ActionButton>
            </div>
          </section>
        </div>
      ) : null}

      {managedLibrary ? <LibraryManagerSheet library={managedLibrary} onClose={() => setManagedLibrary(null)} /> : null}

      {notificationsOpen ? (
        <Sheet onClose={() => setNotificationsOpen(false)} title="通知中心">
          <NotificationCenterSheet
            noticeRecords={noticeRecords}
            onMarkRead={handleNoticeMarkRead}
            onOpenTarget={handleNoticeOpenTarget}
          />
          <div className="workspace-card compact-list inner-list" hidden>
            {false ? (
              <p className="muted-text">还没有通知。</p>
            ) : (
              ([] as Array<{ id: string; title: string; detail: string; createdAt: string; read: boolean }>).map((item) => (
                <article className="notice-card" key={item.id}>
                  <div>
                    <strong>{item.title}</strong>
                    <p>{item.detail}</p>
                  </div>
                  <div className="notice-meta">
                    <span>{item.createdAt}</span>
                    {item.read === false ? (
                      <ActionButton onClick={() => undefined}>
                        标记已读
                      </ActionButton>
                    ) : (
                      <span className="muted-text">已读</span>
                    )}
                  </div>
                </article>
              ))
            )}
          </div>
        </Sheet>
      ) : null}

      {activeView === 'file-center' && pendingAction ? (
        <FileActionDialog
          action={pendingAction}
          onCancel={() => setPendingAction(null)}
          onConfirm={() => void confirmPendingAction()}
          onInspectConflict={() => {
            if (pendingAction.kind !== 'delete-conflict') {
              return;
            }
            setPendingAction(null);
            setTaskTab('transfer');
            setPendingTaskSelection({ taskIds: pendingAction.blockingTaskIds });
            activateWorkspace('task-center');
          }}
        />
      ) : null}

      {activeView === 'file-center' && folderDraft !== null ? (
        <Sheet onClose={() => setFolderDraft(null)} title="新建目录">
          <div className="sheet-form">
            <label className="form-field">
              <span>目录名称</span>
              <input aria-label="目录名称" value={folderDraft} onChange={(event) => setFolderDraft(event.target.value)} />
            </label>
          </div>
          <div className="sheet-actions right">
            <ActionButton onClick={() => setFolderDraft(null)}>取消</ActionButton>
            <ActionButton
              tone="primary"
              onClick={() => {
                if (!folderDraft.trim()) {
                  setFeedback({ message: '目录名称不能为空', tone: 'warning' });
                  return;
                }
                void fileCenterApi
                  .createFolder({
                    libraryId: activeLibraryId,
                    parentId: currentFolderId,
                    name: folderDraft.trim(),
                  })
                  .then((result) => {
                    setFeedback({ message: result.message, tone: 'success' });
                    setFileCenterVersion((current) => current + 1);
                    setFolderDraft(null);
                  })
                  .catch(() => {
                    setFeedback({ message: '目录创建失败，请稍后重试', tone: 'critical' });
                  });
              }}
            >
              创建
            </ActionButton>
          </div>
        </Sheet>
      ) : null}
    </div>
  );
}

function FileActionDialog({
  action,
  onCancel,
  onConfirm,
  onInspectConflict,
}: {
  action: FileConfirmAction;
  onCancel: () => void;
  onConfirm: () => void;
  onInspectConflict: () => void;
}) {
  const isDeleteAsset = action.kind === 'delete-asset';
  const skippedCount =
    action.kind === 'delete-asset'
      ? 0
      : action.kind === 'delete-conflict'
        ? action.totalSelected - action.items.length
        : action.totalSelected - action.items.length;

  const title =
    action.kind === 'sync'
      ? '确认同步'
      : action.kind === 'delete-endpoint'
        ? '确认删除副本'
        : action.kind === 'delete-conflict'
          ? '存在运行中任务'
          : '确认删除资产';

  const description =
    action.kind === 'sync'
      ? action.totalSelected === 1
        ? `是否将“${action.items[0].name}”同步到 ${action.endpointName}？`
        : `是否将选中的 ${action.totalSelected} 项资产同步到 ${action.endpointName}？`
      : action.kind === 'delete-endpoint'
        ? action.totalSelected === 1
          ? `是否删除“${action.items[0].name}”在 ${action.endpointName} 上的副本？`
          : `是否删除选中的 ${action.totalSelected} 项资产在 ${action.endpointName} 上的副本？`
        : action.kind === 'delete-conflict'
          ? `当前选中的资产仍有关联传输任务在运行。若继续删除，将先取消相关任务，再执行删除。`
        : action.items.length === 1
          ? `是否删除资产“${action.items[0].name}”？`
          : `是否删除这 ${action.items.length} 个资产？`;

  const notes: Array<{ text: string; critical?: boolean }> = [];

  if (action.kind === 'sync' && skippedCount > 0) {
    notes.push({ text: `其中 ${skippedCount} 项在该端点上已同步，将自动跳过。` });
  }

  if (action.kind === 'delete-endpoint') {
    if (skippedCount > 0) {
      notes.push({ text: `其中 ${skippedCount} 项在该端点上已不可删除，将自动跳过。` });
    }
    action.guidanceNotes?.forEach((note) => {
      notes.push({ text: note });
    });
    if (action.willDeleteAssetCount > 0) {
      notes.push({ text: `其中 ${action.willDeleteAssetCount} 项会移除最后一个受管副本，资产将被删除。`, critical: true });
    } else {
      notes.push({ text: '删除后可在任务中心查看执行状态。' });
    }
  }

  if (action.kind === 'sync') {
    action.guidanceNotes?.forEach((note) => {
      notes.push({ text: note });
    });
  }

  if (isDeleteAsset) {
    notes.push({ text: '删除后可在任务中心查看执行状态。', critical: true });
  }

  if (action.kind === 'delete-conflict') {
    notes.push({
      text: `关联任务：${action.blockingTaskTitles.join('、')}`,
      critical: true,
    });
  }

  return (
    <div className="dialog-backdrop" role="presentation" onClick={onCancel}>
      <section className="dialog-panel compact-confirm-dialog" role="dialog" aria-label={title} onClick={(event) => event.stopPropagation()}>
        <div className="sheet-header">
          <strong>{title}</strong>
        </div>
        <div className="dialog-card">
          <p className="muted-paragraph">{description}</p>
          {notes.map((note) => (
            <p key={note.text} className={note.critical ? 'confirm-warning critical' : 'confirm-warning'}>
              {note.text}
            </p>
          ))}
        </div>
        <div className="sheet-actions right">
          <ActionButton onClick={onCancel}>取消</ActionButton>
          {action.kind === 'delete-conflict' ? (
            <ActionButton onClick={onInspectConflict}>查看任务</ActionButton>
          ) : null}
          <ActionButton tone={action.kind === 'sync' ? 'primary' : 'danger'} onClick={onConfirm}>
            {action.kind === 'sync' ? '确认同步' : action.kind === 'delete-conflict' ? '取消任务并删除' : '确认删除'}
          </ActionButton>
        </div>
      </section>
    </div>
  );
}

function BatchAnnotationDialog({
  count,
  onClose,
  onSave,
}: {
  count: number;
  onClose: () => void;
  onSave: (input: { rating: number | null; colorLabel: FileCenterColorLabel | null }) => void;
}) {
  const [rating, setRating] = useState<number | null>(null);
  const [colorLabel, setColorLabel] = useState<FileCenterColorLabel | null>(null);

  return (
    <div className="dialog-backdrop" role="presentation" onClick={onClose}>
      <section className="dialog-panel batch-annotation-dialog" role="dialog" aria-label="批量标记" onClick={(event) => event.stopPropagation()}>
        <div className="sheet-header">
          <strong>批量标记</strong>
        </div>
        <div className="dialog-card batch-annotation-card">
          <p className="muted-paragraph">将为选中的 {count} 项资产统一设置星级和色标。未选择的项会保持原值。</p>

          <div className="annotation-choice-group">
            <strong>星级</strong>
            <div className="annotation-choice-grid" role="group" aria-label="批量星级">
              <button className={rating === null ? 'active' : ''} type="button" onClick={() => setRating(null)}>
                保持不变
              </button>
              <button className={rating === 0 ? 'active' : ''} type="button" onClick={() => setRating(0)}>
                无评级
              </button>
              {Array.from({ length: 5 }, (_, index) => {
                const value = index + 1;
                return (
                  <button
                    key={value}
                    aria-label={`${value} 星`}
                    className={rating === value ? 'active' : ''}
                    type="button"
                    onClick={() => setRating(value)}
                  >
                    <span className="file-rating">
                      {Array.from({ length: value }, (_, starIndex) => (
                        <Star key={`${value}-${starIndex}`} size={12} fill="currentColor" />
                      ))}
                    </span>
                  </button>
                );
              })}
            </div>
          </div>

          <div className="annotation-choice-group">
            <strong>色标</strong>
            <div className="annotation-choice-grid" role="group" aria-label="批量色标">
              <button className={colorLabel === null ? 'active' : ''} type="button" onClick={() => setColorLabel(null)}>
                保持不变
              </button>
              {(['无', '红标', '黄标', '绿标', '蓝标', '紫标'] as FileCenterColorLabel[]).map((option) => (
                <button
                  key={option}
                  aria-label={option}
                  className={colorLabel === option ? 'active' : ''}
                  type="button"
                  onClick={() => setColorLabel(option)}
                >
                  <span className={`color-label-dot ${resolveBatchColorClass(option)}`} />
                  <span>{option}</span>
                </button>
              ))}
            </div>
          </div>
        </div>
        <div className="sheet-actions right">
          <ActionButton onClick={onClose}>取消</ActionButton>
          <ActionButton tone="primary" onClick={() => onSave({ rating, colorLabel })}>
            保存标记
          </ActionButton>
        </div>
      </section>
    </div>
  );
}

function TagEditorDialog({
  availableTags,
  item,
  onClose,
  onSave,
}: {
  availableTags: FileCenterTagSuggestion[];
  item: FileCenterEntry;
  onClose: () => void;
  onSave: (tags: string[]) => void;
}) {
  const [searchText, setSearchText] = useState('');
  const [selectedTags, setSelectedTags] = useState<string[]>(item.tags);

  useEffect(() => {
    setSelectedTags(item.tags);
    setSearchText('');
  }, [item]);

  const filteredTags = useMemo(() => {
    const keyword = searchText.trim().toLowerCase();
    return availableTags.filter((tag) => (keyword ? tag.name.toLowerCase().includes(keyword) : true)).slice(0, 18);
  }, [availableTags, searchText]);

  function toggleTag(tag: string) {
    setSelectedTags((current) =>
      current.includes(tag) ? current.filter((itemTag) => itemTag !== tag) : [...current, tag],
    );
  }

  function addCurrentInput() {
    const next = searchText.trim();
    if (!next) {
      return;
    }
    setSelectedTags((current) => (current.includes(next) ? current : [...current, next]));
  }

  return (
    <div className="dialog-backdrop" role="presentation" onClick={onClose}>
      <section className="dialog-panel tag-editor-dialog" role="dialog" aria-label="标签编辑" onClick={(event) => event.stopPropagation()}>
        <div className="sheet-header">
          <strong>标签</strong>
        </div>
        <div className="tag-editor-toolbar">
          <div className="tag-editor-search">
            <input
              aria-label="标签搜索"
              placeholder="搜索标签"
              value={searchText}
              onChange={(event) => setSearchText(event.target.value)}
            />
            <ActionButton onClick={() => setSearchText(searchText.trim())}>搜索</ActionButton>
          </div>
          <ActionButton tone="primary" onClick={addCurrentInput}>
            新增标签
          </ActionButton>
        </div>
        <div className="sheet-section">
          <div className="endpoint-row">
            {selectedTags.map((tag) => (
              <button key={tag} className="tag-chip-button" type="button" onClick={() => toggleTag(tag)}>
                {tag}
              </button>
            ))}
          </div>
        </div>
        <div className="sheet-section">
          <strong>常用标签</strong>
          <div className="tag-suggestion-list">
            {filteredTags.map((tag) => (
              <button
                aria-label={`${tag.name} ${tag.count} 次使用`}
                key={tag.name}
                className={selectedTags.includes(tag.name) ? 'active' : ''}
                type="button"
                onClick={() => toggleTag(tag.name)}
              >
                <span>{tag.name}</span>
                <span>{tag.count} 次使用</span>
              </button>
            ))}
          </div>
        </div>
        <div className="sheet-actions right">
          <ActionButton onClick={onClose}>取消</ActionButton>
          <ActionButton tone="primary" onClick={() => onSave(selectedTags)}>
            保存标签
          </ActionButton>
        </div>
      </section>
    </div>
  );
}

function BatchTagDialog({
  availableTags,
  items,
  onClose,
  onSave,
}: {
  availableTags: FileCenterTagSuggestion[];
  items: FileCenterEntry[];
  onClose: () => void;
  onSave: (tags: string[]) => void;
}) {
  const [searchText, setSearchText] = useState('');
  const [selectedTags, setSelectedTags] = useState<string[]>(resolveBatchTagUnion(items));

  useEffect(() => {
    setSelectedTags(resolveBatchTagUnion(items));
    setSearchText('');
  }, [items]);

  const filteredTags = useMemo(() => {
    const keyword = searchText.trim().toLowerCase();
    return availableTags.filter((tag) => (keyword ? tag.name.toLowerCase().includes(keyword) : true)).slice(0, 18);
  }, [availableTags, searchText]);

  function toggleTag(tag: string) {
    setSelectedTags((current) =>
      current.includes(tag) ? current.filter((itemTag) => itemTag !== tag) : [...current, tag],
    );
  }

  function addCurrentInput() {
    const next = searchText.trim();
    if (!next) {
      return;
    }
    setSelectedTags((current) => (current.includes(next) ? current : [...current, next]));
  }

  return (
    <div className="dialog-backdrop" role="presentation" onClick={onClose}>
      <section className="dialog-panel tag-editor-dialog" role="dialog" aria-label="批量标签" onClick={(event) => event.stopPropagation()}>
        <div className="sheet-header">
          <strong>批量标签</strong>
        </div>
        <div className="dialog-card">
          <p className="muted-paragraph">将为选中的 {items.length} 项条目批量调整标签。已存在于任一条目上的标签都可直接取消，系统会自动跳过原本没有该标签的条目。</p>
        </div>
        <div className="tag-editor-toolbar">
          <div className="tag-editor-search">
            <input
              aria-label="批量标签搜索"
              placeholder="搜索标签"
              value={searchText}
              onChange={(event) => setSearchText(event.target.value)}
            />
            <ActionButton onClick={() => setSearchText(searchText.trim())}>搜索</ActionButton>
          </div>
          <ActionButton tone="primary" onClick={addCurrentInput}>
            新增标签
          </ActionButton>
        </div>
        <div className="sheet-section">
          <div className="endpoint-row">
            {selectedTags.map((tag) => (
              <button key={tag} className="tag-chip-button" type="button" onClick={() => toggleTag(tag)}>
                {tag}
              </button>
            ))}
          </div>
        </div>
        <div className="sheet-section">
          <strong>常用标签</strong>
          <div className="tag-suggestion-list">
            {filteredTags.map((tag) => (
              <button
                aria-label={`${tag.name} ${tag.count} 次使用`}
                key={tag.name}
                className={selectedTags.includes(tag.name) ? 'active' : ''}
                type="button"
                onClick={() => toggleTag(tag.name)}
              >
                <span>{tag.name}</span>
                <span>{tag.count} 次使用</span>
              </button>
            ))}
          </div>
        </div>
        <div className="sheet-actions right">
          <ActionButton onClick={onClose}>取消</ActionButton>
          <ActionButton tone="primary" onClick={() => onSave(selectedTags)}>
            保存标签
          </ActionButton>
        </div>
      </section>
    </div>
  );
}

function resolveBatchTagUnion(items: FileCenterEntry[]) {
  return Array.from(new Set(items.flatMap((item) => item.tags)));
}

function resolveBatchColorClass(colorLabel: FileCenterColorLabel) {
  if (colorLabel === '红标') return 'red';
  if (colorLabel === '黄标') return 'yellow';
  if (colorLabel === '绿标') return 'green';
  if (colorLabel === '蓝标') return 'blue';
  if (colorLabel === '紫标') return 'purple';
  return 'none';
}

function isRuntimeComponentOnline(component: RuntimeComponentRecord | null) {
  if (!component) {
    return false;
  }
  return component.status === 'ONLINE' || component.status === 'READY';
}

const navIcons: Record<MainView, React.ReactNode> = {
  'file-center': <FolderOpen size={16} />,
  'import-center': <ArrowDownToLine size={16} />,
  'task-center': <CircleEllipsis size={16} />,
  issues: <AlertTriangle size={16} />,
  'storage-nodes': <HardDrive size={16} />,
  settings: <Settings2 size={16} />,
};

function resolveRuntimeLightState(summary: SystemRuntimeSummary): RuntimeLightState {
  if (summary.centerStatus !== 'ready') {
    return {
      ariaLabel: '系统状态：中心服务异常',
      tooltip: '中心服务：异常 · 本地执行器：未知',
      tone: 'critical',
    };
  }

  if (summary.agentStatus !== 'online') {
    return {
      ariaLabel: '系统状态：中心服务可用，本地执行器异常',
      tooltip: '中心服务：正常 · 本地执行器：异常',
      tone: 'warning',
    };
  }

  return {
    ariaLabel: '系统状态：中心服务可用，本地执行器在线',
    tooltip: '中心服务：正常 · 本地执行器：在线',
    tone: 'success',
  };
}
