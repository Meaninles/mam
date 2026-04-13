import { startTransition, useEffect, useMemo, useRef, useState } from 'react';
import type { FileNode, IssueRecord, Library, Severity, TaskPriority, TaskRecord, TaskItemRecord, TaskTab } from '../data';
import { EmptyState } from '../components/Shared';
import {
  jobsApi,
  type JobDetail,
  type JobItemRecord,
  type JobPriority,
  type JobRecord,
  type JobStatus,
  type JobStreamEvent,
} from '../lib/jobsApi';
import { TaskCenterPage, TaskDetailSheet } from './TaskCenterPage';

type PendingTaskFocus = {
  taskIds: string[];
  issueId?: string;
  taskItemId?: string;
  openIssuePopover?: boolean;
} | null;

type WorkspaceFeedback = {
  message: string;
  tone: Severity;
};

type TaskCenterWorkspaceProps = {
  activeTab: TaskTab;
  visible: boolean;
  fileNodes: FileNode[];
  issues: IssueRecord[];
  libraries: Library[];
  preselectedTaskIds: PendingTaskFocus;
  statusFilter: string;
  onConsumePreselectedTaskIds: () => void;
  onFeedback: (feedback: WorkspaceFeedback) => void;
  onOpenFileCenterForTask: (task: TaskRecord) => void;
  onOpenIssueCenterForIssue: (issue: IssueRecord) => void;
  onOpenIssueCenterForTask: (task: TaskRecord) => void;
  onOpenStorageNodesForTask: (task: TaskRecord) => void;
  onSetActiveTab: (value: TaskTab) => void;
  onSetTaskStatusFilter: (value: string) => void;
};

const PAGE_SIZE = 100;

export function TaskCenterWorkspace(props: TaskCenterWorkspaceProps) {
  const {
    activeTab,
    visible,
    fileNodes,
    issues,
    libraries,
    preselectedTaskIds,
    statusFilter,
    onConsumePreselectedTaskIds,
    onFeedback,
    onOpenFileCenterForTask,
    onOpenIssueCenterForIssue,
    onOpenIssueCenterForTask,
    onOpenStorageNodesForTask,
    onSetActiveTab,
    onSetTaskStatusFilter,
  } = props;

  const [jobDetails, setJobDetails] = useState<JobDetail[]>([]);
  const [loading, setLoading] = useState(true);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [taskDetailId, setTaskDetailId] = useState<string | null>(null);
  const refreshTimeoutRef = useRef<number | null>(null);
  const loadWorkspaceRef = useRef<(showLoading: boolean) => Promise<void>>(async () => {});
  const previousVisibleRef = useRef(visible);

  useEffect(() => {
    let cancelled = false;

    const loadWorkspace = async (showLoading: boolean) => {
      if (showLoading) {
        setLoading(true);
      }
      setErrorMessage(null);

      try {
        const jobs = await loadAllJobs();
        const details = await Promise.all(
          jobs.map(async (job) => {
            try {
              return await jobsApi.detail(job.id);
            } catch {
              return {
                job,
                items: [],
                links: [],
              } satisfies JobDetail;
            }
          }),
        );
        if (cancelled) {
          return;
        }
        startTransition(() => {
          setJobDetails(details);
          setLoading(false);
        });
      } catch (error) {
        if (cancelled) {
          return;
        }
        setLoading(false);
        setErrorMessage(error instanceof Error ? error.message : '任务中心加载失败');
      }
    };

    loadWorkspaceRef.current = loadWorkspace;

    void loadWorkspace(true);

    const unsubscribe = jobsApi.subscribe((_event: JobStreamEvent) => {
      if (refreshTimeoutRef.current !== null) {
        window.clearTimeout(refreshTimeoutRef.current);
      }
      refreshTimeoutRef.current = window.setTimeout(() => {
        refreshTimeoutRef.current = null;
        void loadWorkspace(false);
      }, 180);
    });

    return () => {
      cancelled = true;
      unsubscribe();
      if (refreshTimeoutRef.current !== null) {
        window.clearTimeout(refreshTimeoutRef.current);
      }
    };
  }, []);

  useEffect(() => {
    const wasVisible = previousVisibleRef.current;
    previousVisibleRef.current = visible;

    if (!visible || wasVisible === visible) {
      return;
    }

    void loadWorkspaceRef.current(false);
  }, [visible]);

  const workspace = useMemo(() => {
    const tasks = jobDetails.map((detail) => mapJobDetailToTask(detail));
    const taskItems = jobDetails.flatMap((detail) => mapJobItemsToTaskItems(detail));
    return {
      tasks,
      taskItems,
      taskById: new Map(tasks.map((task) => [task.id, task])),
      taskItemsByTaskId: taskItems.reduce<Record<string, TaskItemRecord[]>>((accumulator, item) => {
        if (!accumulator[item.taskId]) {
          accumulator[item.taskId] = [];
        }
        accumulator[item.taskId].push(item);
        return accumulator;
      }, {}),
    };
  }, [jobDetails]);

  const taskDetail = taskDetailId ? workspace.taskById.get(taskDetailId) ?? null : null;

  useEffect(() => {
    if (taskDetailId && !workspace.taskById.has(taskDetailId)) {
      setTaskDetailId(null);
    }
  }, [taskDetailId, workspace.taskById]);

  const refreshWorkspace = async () => {
    const jobs = await loadAllJobs();
    const details = await Promise.all(
      jobs.map(async (job) => {
        try {
          return await jobsApi.detail(job.id);
        } catch {
          return {
            job,
            items: [],
            links: [],
          } satisfies JobDetail;
        }
      }),
    );
    startTransition(() => {
      setJobDetails(details);
      setErrorMessage(null);
    });
  };

  const changeTaskPriority = async (taskIds: string[], priority: TaskPriority) => {
    if (taskIds.length === 0) {
      return;
    }
    await Promise.all(taskIds.map((taskId) => jobsApi.updatePriority(taskId, mapTaskPriorityToJobPriority(priority))));
    await refreshWorkspace();
    onFeedback({
      message: taskIds.length > 1 ? `已更新 ${taskIds.length} 个任务的优先级` : '任务优先级已更新',
      tone: 'success',
    });
  };

  const changeTaskStatus = async (taskIds: string[], action: 'pause' | 'resume' | 'retry' | 'cancel') => {
    if (taskIds.length === 0) {
      return;
    }
    await Promise.all(
      taskIds.map((taskId) => {
        if (action === 'pause') return jobsApi.pause(taskId);
        if (action === 'resume') return jobsApi.resume(taskId);
        if (action === 'retry') return jobsApi.retry(taskId);
        return jobsApi.cancel(taskId);
      }),
    );
    await refreshWorkspace();
    onFeedback({
      message: describeTaskActionFeedback(action, taskIds.length),
      tone: action === 'cancel' ? 'warning' : 'success',
    });
  };

  const changeTaskItemStatus = async (taskItemIds: string[], action: 'pause' | 'resume' | 'cancel') => {
    if (taskItemIds.length === 0) {
      return;
    }
    await Promise.all(
      taskItemIds.map((itemId) => {
        if (action === 'pause') return jobsApi.pauseItem(itemId);
        if (action === 'resume') return jobsApi.resumeItem(itemId);
        return jobsApi.cancelItem(itemId);
      }),
    );
    await refreshWorkspace();
    onFeedback({
      message: describeTaskItemActionFeedback(action, taskItemIds.length),
      tone: action === 'cancel' ? 'warning' : 'success',
    });
  };

  if (loading) {
    return (
      <section className="page-shell">
        <div className="workspace-card">
          <p className="muted-text">正在加载真实任务数据…</p>
        </div>
      </section>
    );
  }

  if (errorMessage) {
    return (
      <section className="page-shell">
        <div className="workspace-card">
          <EmptyState title="任务中心暂时不可用" description={errorMessage} />
        </div>
      </section>
    );
  }

  return (
    <>
      <TaskCenterPage
        activeTab={activeTab}
        fileNodes={fileNodes}
        issues={issues}
        libraries={libraries}
        preselectedTaskIds={preselectedTaskIds}
        statusFilter={statusFilter}
        taskItems={workspace.taskItems}
        tasks={workspace.tasks}
        onChangeTaskPriority={(taskIds, priority) => void changeTaskPriority(taskIds, priority)}
        onChangeTaskItemStatus={(taskItemIds, action) => void changeTaskItemStatus(taskItemIds, action)}
        onChangeTaskStatus={(taskIds, action) => void changeTaskStatus(taskIds, action)}
        onConsumePreselectedTaskIds={onConsumePreselectedTaskIds}
        onOpenIssueCenterForIssue={onOpenIssueCenterForIssue}
        onOpenIssueCenterForTask={onOpenIssueCenterForTask}
        onOpenTaskDetail={(value) => setTaskDetailId(value?.id ?? null)}
        onSetActiveTab={onSetActiveTab}
        onSetTaskStatusFilter={onSetTaskStatusFilter}
      />

      {taskDetail ? (
        <TaskDetailSheet
          fileNodes={fileNodes}
          item={taskDetail}
          issues={issues}
          items={workspace.taskItemsByTaskId[taskDetail.id] ?? []}
          onChangeTaskPriority={(taskIds, priority) => void changeTaskPriority(taskIds, priority)}
          onChangeTaskItemStatus={(taskItemIds, action) => void changeTaskItemStatus(taskItemIds, action)}
          onChangeTaskStatus={(taskIds, action) => void changeTaskStatus(taskIds, action)}
          onClose={() => setTaskDetailId(null)}
          onOpenFileCenterForTask={onOpenFileCenterForTask}
          onOpenIssueCenterForTask={onOpenIssueCenterForTask}
          onOpenStorageNodesForTask={onOpenStorageNodesForTask}
        />
      ) : null}
    </>
  );
}

async function loadAllJobs() {
  const items: JobRecord[] = [];
  let page = 1;

  while (true) {
    const result = await jobsApi.list({ page, pageSize: PAGE_SIZE });
    items.push(...result.items);
    if (items.length >= result.total || result.items.length === 0) {
      return items;
    }
    page += 1;
  }
}

function mapJobDetailToTask(detail: JobDetail): TaskRecord {
  const { job, items, links } = detail;
  const kind = resolveTaskKind(job);
  const firstItem = items[0];
  const externalItem = resolvePrimaryExternalTaskItem(items);
  const routeSummary = resolveRouteSummary(job, firstItem);
  const scopeLabel = firstItem?.sourcePath ?? firstItem?.targetPath ?? job.summary;
  const phaseLabel = resolveTaskPhaseLabel(job.status, items);
  const executorEngineLabel = resolveExecutorEngineLabel(externalItem?.externalTaskEngine, job.routeType);
  const externalTaskStatus = resolveExternalTaskStatusValue(externalItem);
  const failureLocation = resolveFailureLocation(job, externalItem);
  const rawErrorMessage = externalItem?.latestErrorMessage ?? job.latestErrorMessage;

  return {
    id: job.id,
    kind,
    title: job.title,
    type: resolveTaskType(job),
    otherTaskType: kind === 'other' ? resolveOtherTaskType(job) : undefined,
    backendTypes: [job.jobIntent],
    businessType: kind === 'transfer' ? resolveTransferBusinessType(job) : undefined,
    syncLinkType: kind === 'transfer' ? resolveTransferLinkType(job.routeType) : undefined,
    status: resolveJobStatusLabel(job.status),
    statusTone: resolveJobStatusTone(job.status),
    libraryId: job.libraryId ?? '',
    source: routeSummary.source,
    target: routeSummary.target,
    sourcePath: firstItem?.sourcePath,
    targetPath: firstItem?.targetPath,
    progress: Math.round(job.progressPercent),
    speed: formatSpeed(job.speedBps),
    eta: formatEta(job.etaSeconds, job.status),
    fileCount: job.totalItems,
    folderCount: 0,
    totalSize: `${job.totalItems} 项`,
    multiFile: job.totalItems > 1,
    updatedAt: formatDateTime(job.updatedAt),
    priority: resolveTaskPriority(job.priority),
    issueIds: [],
    creator: resolveCreatorLabel(job.createdByType),
    createdAt: formatDateTime(job.createdAt),
    startedAt: formatDateTime(job.startedAt),
    finishedAt: formatDateTime(job.finishedAt),
    phaseLabel,
    scopeLabel,
    endpointLabel: resolveEndpointLabel(links),
    resultSummary: job.outcomeSummary ?? job.latestErrorMessage ?? firstItem?.resultSummary,
    waitingReason: resolveWaitingReason(job.status),
    executorEngineLabel,
    externalTaskId: externalItem?.externalTaskId,
    externalTaskStatus,
    failureLocation,
    rawErrorMessage,
  };
}

function mapJobItemsToTaskItems(detail: JobDetail): TaskItemRecord[] {
  const priority = resolveTaskPriority(detail.job.priority);
  return detail.items.map((item) => ({
    id: item.id,
    taskId: detail.job.id,
    parentId: item.parentItemId,
    name: item.title,
    kind: resolveTaskItemKind(item),
    depth: item.parentItemId ? 2 : 1,
    phase: resolveItemPhaseLabel(item),
    status: resolveItemStatusLabel(item.status),
    statusTone: resolveItemStatusTone(item.status),
    priority,
    progress: Math.round(item.progressPercent),
    size: formatBytes(item.bytesTotal),
    speed: formatSpeed(item.speedBps),
    sourcePath: item.sourcePath,
    targetPath: item.targetPath,
    pathLabel: item.sourcePath ?? item.targetPath,
    resultLabel: item.latestErrorMessage ?? item.resultSummary,
    issueIds: [],
    executorEngineLabel: resolveExecutorEngineLabel(item.externalTaskEngine, detail.job.routeType),
    externalTaskId: item.externalTaskId,
    externalTaskStatus: resolveExternalTaskStatusValue(item),
    failureLocation: resolveFailureLocation(detail.job, item),
    rawErrorMessage: item.latestErrorMessage,
  }));
}

function resolvePrimaryExternalTaskItem(items: JobItemRecord[]) {
  const withExternal = items.filter((item) => item.externalTaskEngine || item.externalTaskId || item.externalTaskStatus);
  if (withExternal.length === 0) {
    return null;
  }

  const priorityOrder = ['RUNNING', 'FAILED', 'PAUSED', 'CANCELED', 'COMPLETED', 'QUEUED', 'PENDING'];
  return (
    withExternal.sort((left, right) => {
      const leftIndex = priorityOrder.indexOf(left.status);
      const rightIndex = priorityOrder.indexOf(right.status);
      return (leftIndex === -1 ? priorityOrder.length : leftIndex) - (rightIndex === -1 ? priorityOrder.length : rightIndex);
    })[0] ?? null
  );
}

function resolveTaskKind(job: JobRecord): TaskTab {
  if (job.jobFamily === 'TRANSFER' || job.jobIntent === 'IMPORT' || job.jobIntent === 'REPLICATE') {
    return 'transfer';
  }
  return 'other';
}

function resolveTaskType(job: JobRecord) {
  if (job.jobIntent === 'SCAN_DIRECTORY') return 'SCAN';
  if (job.jobIntent === 'EXTRACT_METADATA') return 'METADATA_EXTRACT';
  if (job.jobIntent === 'VERIFY_REPLICA' || job.jobIntent === 'VERIFY_ASSET') return 'VERIFY';
  if (job.jobIntent === 'DELETE_REPLICA' || job.jobIntent === 'DELETE_ASSET') return 'DELETE';
  if (job.jobIntent === 'IMPORT') return 'IMPORT';
  if (job.jobIntent === 'REPLICATE') return 'SYNC';
  return job.jobIntent;
}

function resolveOtherTaskType(job: JobRecord): TaskRecord['otherTaskType'] {
  if (job.jobIntent === 'SCAN_DIRECTORY') return 'SCAN';
  if (job.jobIntent === 'EXTRACT_METADATA') return 'METADATA_EXTRACT';
  if (job.jobIntent === 'VERIFY_REPLICA' || job.jobIntent === 'VERIFY_ASSET') return 'VERIFY';
  if (job.jobIntent === 'DELETE_REPLICA' || job.jobIntent === 'DELETE_ASSET') return 'DELETE_CLEANUP';
  return undefined;
}

function resolveTransferBusinessType(job: JobRecord): TaskRecord['businessType'] {
  return job.jobIntent === 'IMPORT' ? 'IMPORT' : 'SYNC';
}

function resolveTransferLinkType(routeType?: string): TaskRecord['syncLinkType'] {
  if (routeType === 'UPLOAD') return 'UPLOAD';
  if (routeType === 'DOWNLOAD') return 'DOWNLOAD';
  return 'COPY';
}

function resolveJobStatusLabel(status: JobStatus) {
  switch (status) {
    case 'RUNNING':
      return '运行中';
    case 'PAUSED':
      return '已暂停';
    case 'WAITING_CONFIRMATION':
      return '等待确认';
    case 'WAITING_RETRY':
    case 'PENDING':
    case 'QUEUED':
      return '待执行';
    case 'PARTIAL_SUCCESS':
      return '部分成功';
    case 'FAILED':
      return '失败';
    case 'COMPLETED':
      return '已完成';
    case 'CANCELED':
      return '已取消';
    default:
      return status;
  }
}

function resolveItemStatusLabel(status: string) {
  switch (status) {
    case 'RUNNING':
      return '运行中';
    case 'PAUSED':
      return '已暂停';
    case 'WAITING_RETRY':
    case 'PENDING':
    case 'QUEUED':
      return '待执行';
    case 'SKIPPED':
      return '已跳过';
    case 'FAILED':
      return '失败';
    case 'COMPLETED':
      return '已完成';
    case 'CANCELED':
      return '已取消';
    default:
      return status;
  }
}

function resolveJobStatusTone(status: JobStatus): Severity {
  switch (status) {
    case 'RUNNING':
    case 'PARTIAL_SUCCESS':
      return 'warning';
    case 'FAILED':
      return 'critical';
    case 'COMPLETED':
      return 'success';
    default:
      return 'info';
  }
}

function resolveItemStatusTone(status: string): Severity {
  switch (status) {
    case 'RUNNING':
      return 'warning';
    case 'FAILED':
      return 'critical';
    case 'COMPLETED':
      return 'success';
    default:
      return 'info';
  }
}

function resolveTaskPriority(priority: JobPriority): TaskPriority {
  if (priority === 'HIGH') return '高优先级';
  if (priority === 'LOW') return '低优先级';
  return '普通优先级';
}

function mapTaskPriorityToJobPriority(priority: TaskPriority): JobPriority {
  if (priority === '高优先级') return 'HIGH';
  if (priority === '低优先级') return 'LOW';
  return 'NORMAL';
}

function resolveTaskPhaseLabel(status: JobStatus, items: JobItemRecord[]) {
  const primaryItem =
    items.find((item) => item.status === 'RUNNING') ??
    items.find((item) => item.status === 'FAILED') ??
    items.find((item) => item.status === 'PAUSED') ??
    resolvePrimaryExternalTaskItem(items);
  if (primaryItem?.phase) {
    return resolvePhaseLabel(primaryItem.phase);
  }
  const externalPhase = resolveExternalTaskPhaseLabel(primaryItem?.externalTaskEngine, primaryItem?.externalTaskStatus);
  if (externalPhase) {
    return externalPhase;
  }
  return resolveJobStatusLabel(status);
}

function resolveItemPhaseLabel(item: JobItemRecord) {
  if (item.phase) {
    return resolvePhaseLabel(item.phase);
  }
  const externalPhase = resolveExternalTaskPhaseLabel(item.externalTaskEngine, item.externalTaskStatus);
  if (externalPhase) {
    return externalPhase;
  }
  return resolveItemStatusLabel(item.status);
}

function resolvePhaseLabel(phase: string) {
  if (phase === 'EXECUTING') return '执行中';
  if (phase === 'COMPLETED') return '已完成';
  if (phase === 'FAILED') return '失败';
  if (phase === 'PAUSED') return '已暂停';
  if (phase === 'CANCELED') return '已取消';
  return phase;
}

function resolveExternalTaskPhaseLabel(engine?: string, status?: string) {
  if (!status) {
    return null;
  }

  if (engine === 'CD2_REMOTE_UPLOAD') {
    if (status === 'WaitforPreprocessing' || status === 'Preprocessing') return '写入 CD2 缓存中';
    if (status === 'Inqueue') return '缓存写入完成，等待云端上传';
    if (status === 'Transfer') return 'CloudDrive2 上传中';
    if (status === 'Pause') return '已暂停';
    if (status === 'Finish') return '已完成';
    if (status === 'Skipped') return '已跳过';
    if (status === 'Cancelled') return '已取消';
    if (status === 'Error' || status === 'FatalError') return 'CloudDrive2 上传失败';
    return status;
  }

  if (engine === 'ARIA2') {
    if (status === 'active') return 'aria2 下载中';
    if (status === 'waiting') return 'aria2 等待下载';
    if (status === 'paused') return '已暂停';
    if (status === 'complete') return '已完成';
    if (status === 'error') return 'aria2 下载失败';
    if (status === 'removed') return '已取消';
    return status;
  }

  return status;
}

function resolveExecutorEngineLabel(engine?: string, routeType?: string) {
  if (engine === 'CD2_REMOTE_UPLOAD') {
    return 'CloudDrive2 上传';
  }
  if (engine === 'ARIA2') {
    return 'aria2 下载';
  }
  if (routeType === 'UPLOAD') {
    return '中心服务上传';
  }
  if (routeType === 'DOWNLOAD') {
    return '中心服务下载';
  }
  if (routeType === 'COPY') {
    return '中心服务复制';
  }
  return undefined;
}

function resolveExternalTaskStatusValue(item?: Pick<JobItemRecord, 'externalTaskEngine' | 'externalTaskStatus'> | null) {
  if (!item?.externalTaskStatus) {
    return undefined;
  }
  const normalized = resolveExternalTaskPhaseLabel(item.externalTaskEngine, item.externalTaskStatus);
  if (!normalized || normalized === item.externalTaskStatus) {
    return item.externalTaskStatus;
  }
  return `${item.externalTaskStatus}（${normalized}）`;
}

function resolveFailureLocation(job: JobRecord, item?: Pick<JobItemRecord, 'externalTaskEngine' | 'latestErrorMessage'> | null) {
  if (item?.externalTaskEngine === 'CD2_REMOTE_UPLOAD') {
    return 'CloudDrive2 上传器';
  }
  if (item?.externalTaskEngine === 'ARIA2') {
    return 'aria2 下载器';
  }
  if (job.status === 'FAILED' || job.status === 'PARTIAL_SUCCESS') {
    return '中心服务';
  }
  return undefined;
}

function resolveTaskItemKind(item: JobItemRecord): TaskItemRecord['kind'] {
  if (item.itemType === 'DIRECTORY_SCAN') return 'folder';
  if (item.itemType === 'CONNECTIVITY_CHECK') return 'step';
  return 'asset';
}

function formatSpeed(speedBps?: number) {
  if (!speedBps || speedBps <= 0) {
    return '—';
  }
  return `${formatBytes(speedBps) ?? '0 B'}/s`;
}

function formatEta(etaSeconds?: number, status?: JobStatus) {
  if (status === 'PAUSED') return '等待继续';
  if (!etaSeconds || etaSeconds <= 0) {
    return '—';
  }
  if (etaSeconds < 60) {
    return `${etaSeconds} 秒`;
  }
  if (etaSeconds < 3600) {
    return `${Math.ceil(etaSeconds / 60)} 分钟`;
  }
  return `${Math.ceil(etaSeconds / 3600)} 小时`;
}

function formatBytes(value?: number) {
  if (!value || value <= 0) {
    return undefined;
  }
  if (value >= 1024 * 1024 * 1024) {
    return `${(value / (1024 * 1024 * 1024)).toFixed(1).replace(/\.0$/, '')} GB`;
  }
  if (value >= 1024 * 1024) {
    return `${(value / (1024 * 1024)).toFixed(1).replace(/\.0$/, '')} MB`;
  }
  if (value >= 1024) {
    return `${Math.round(value / 1024)} KB`;
  }
  return `${value} B`;
}

function formatDateTime(value?: string) {
  if (!value) {
    return undefined;
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString('zh-CN', {
    hour12: false,
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function resolveCreatorLabel(createdByType: string) {
  if (createdByType === 'USER') return '用户';
  if (createdByType === 'AGENT') return '执行器';
  return '系统';
}

function resolveWaitingReason(status: JobStatus) {
  if (status === 'PAUSED') return '等待继续';
  if (status === 'WAITING_RETRY') return '等待重试';
  if (status === 'WAITING_CONFIRMATION') return '等待确认';
  if (status === 'PENDING' || status === 'QUEUED') return '等待调度';
  return undefined;
}

function resolveEndpointLabel(detailLinks: JobDetail['links']) {
  const mountLink = detailLinks.find((link) => link.mountId);
  if (mountLink?.mountId) {
    return `挂载 ${mountLink.mountId}`;
  }
  const storageNodeLink = detailLinks.find((link) => link.storageNodeId);
  if (storageNodeLink?.storageNodeId) {
    return `节点 ${storageNodeLink.storageNodeId}`;
  }
  return undefined;
}

function resolveRouteSummary(job: JobRecord, item?: JobItemRecord) {
  const source = resolveSourceLabel(job.sourceDomain);
  const target = job.routeType ? resolveRouteTypeLabel(job.routeType) : undefined;

  if (item?.sourcePath && item?.targetPath) {
    return {
      source: item.sourcePath,
      target: item.targetPath,
    };
  }

  return { source, target };
}

function resolveSourceLabel(sourceDomain: string) {
  if (sourceDomain === 'FILE_CENTER') return '文件中心';
  if (sourceDomain === 'STORAGE_NODES') return '存储节点';
  if (sourceDomain === 'IMPORT_CENTER') return '导入中心';
  if (sourceDomain === 'ISSUE_CENTER') return '异常中心';
  if (sourceDomain === 'SYSTEM_POLICY') return '系统策略';
  return sourceDomain;
}

function resolveRouteTypeLabel(routeType: string) {
  if (routeType === 'UPLOAD') return '上传';
  if (routeType === 'DOWNLOAD') return '下载';
  return '复制';
}

function describeTaskActionFeedback(action: 'pause' | 'resume' | 'retry' | 'cancel', count: number) {
  const subject = count > 1 ? `${count} 个任务` : '任务';
  if (action === 'pause') return `${subject}已暂停`;
  if (action === 'resume') return `${subject}已恢复`;
  if (action === 'retry') return `${subject}已重新进入执行队列`;
  return `${subject}已取消`;
}

function describeTaskItemActionFeedback(action: 'pause' | 'resume' | 'cancel', count: number) {
  const subject = count > 1 ? `${count} 个子任务` : '子任务';
  if (action === 'pause') return `${subject}已暂停`;
  if (action === 'resume') return `${subject}已恢复`;
  return `${subject}已取消`;
}
