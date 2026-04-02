import { useEffect, useMemo, useState } from 'react';
import { AlertTriangle, ChevronDown, ChevronLeft, ChevronRight, Search } from 'lucide-react';
import type {
  FileNode,
  IssueRecord,
  Library,
  TaskItemRecord,
  TaskPriority,
  TaskRecord,
  TaskTab,
  TransferBusinessType,
  TransferSyncLinkType,
} from '../data';
import {
  ActionButton,
  DenseRow,
  EmptyState,
  IconButton,
  ProgressBar,
  SelectPill,
  Sheet,
  TabSwitch,
  TonePill,
} from '../components/Shared';

type TaskStatusAction = 'pause' | 'resume' | 'retry' | 'cancel';
type TaskItemStatusAction = 'pause' | 'resume' | 'cancel';

const TRANSFER_BUSINESS_ITEMS = [
  { id: 'IMPORT', label: '导入' },
  { id: 'SYNC', label: '同步' },
];

const TRANSFER_STATUS_OPTIONS = ['全部', '运行中', '等待确认', '暂停任务', '异常待处理', '部分成功', '失败', '已完成', '已取消'];
const OTHER_STATUS_OPTIONS = ['全部', '运行中', '等待确认', '暂停任务', '失败', '已完成'];
const SYNC_LINK_OPTIONS = ['全部', '复制', '上传', '下载'];
const SORT_OPTIONS = ['默认排序', '优先级', '进度', '名称'];
const PAGE_SIZE_OPTIONS = [10, 20, 50, 100] as const;

const STATUS_ORDER: Record<string, number> = {
  运行中: 0,
  异常待处理: 1,
  已暂停: 2,
  等待确认: 3,
  部分成功: 4,
  失败: 5,
  已完成: 6,
  已取消: 7,
  待调度: 8,
};

const PRIORITY_ORDER: Record<TaskPriority, number> = {
  高优先级: 0,
  普通优先级: 1,
  低优先级: 2,
};

function resolveBusinessType(task: TaskRecord): TransferBusinessType | null {
  if (task.businessType) {
    return task.businessType;
  }
  if (task.type === 'IMPORT') {
    return 'IMPORT';
  }
  if (task.kind === 'transfer') {
    return 'SYNC';
  }
  return null;
}

function resolveLinkType(task: TaskRecord): TransferSyncLinkType | null {
  if (task.syncLinkType) {
    return task.syncLinkType;
  }
  if (task.type === 'COPY') return 'COPY';
  if (task.type === 'DOWNLOAD') return 'DOWNLOAD';
  return null;
}

function resolveBusinessLabel(task: TaskRecord) {
  return resolveBusinessType(task) === 'IMPORT' ? '导入' : '同步';
}

function resolveLinkLabel(linkType: TransferSyncLinkType | null | undefined) {
  if (linkType === 'COPY') return '复制';
  if (linkType === 'UPLOAD') return '上传';
  if (linkType === 'DOWNLOAD') return '下载';
  return '';
}

function normalizeTaskName(name: string) {
  return name.split(' / ')[0]?.trim() || name.trim();
}

function summarizeTaskNames(names: string[]) {
  const unique = Array.from(new Set(names.map(normalizeTaskName).filter(Boolean)));
  if (unique.length === 0) return '';
  if (unique.length === 1) return unique[0];
  if (unique.length === 2) return `${unique[0]}、${unique[1]}`;
  return `${unique[0]}、${unique[1]}...`;
}

function buildFileNodeMap(fileNodes: FileNode[]) {
  return new Map(fileNodes.map((node) => [node.id, node]));
}

function resolveTaskItemName(item: TaskItemRecord, fileNodeMap: Map<string, FileNode>) {
  if (item.fileNodeId) {
    return fileNodeMap.get(item.fileNodeId)?.name ?? item.name ?? '文件已删除';
  }
  return item.name ?? '未命名文件';
}

function resolveTaskItemSize(item: TaskItemRecord, fileNodeMap: Map<string, FileNode>) {
  if (item.fileNodeId) {
    return fileNodeMap.get(item.fileNodeId)?.size ?? item.size ?? '—';
  }
  return item.size ?? '—';
}

function resolveTaskItemPathValue(item: TaskItemRecord, fileNodeMap: Map<string, FileNode>) {
  if (item.fileNodeId) {
    return fileNodeMap.get(item.fileNodeId)?.path ?? item.sourcePath ?? item.targetPath ?? null;
  }
  return item.sourcePath ?? item.targetPath ?? null;
}

function resolveTaskDisplayTitle(task: TaskRecord, items: TaskItemRecord[], fileNodeMap: Map<string, FileNode>) {
  if (task.kind !== 'transfer') {
    return task.title;
  }

  const displayItems = getDisplayTaskItems(items, task.id);
  if (displayItems.length > 0) {
    return summarizeTaskNames(displayItems.map((item) => resolveTaskItemName(item, fileNodeMap)));
  }

  const topLevelItems = items.filter((item) => !item.parentId);
  if (topLevelItems.length === 0) {
    return task.title;
  }

  if (resolveBusinessType(task) === 'IMPORT' && topLevelItems.length === 1) {
    const childItems = items.filter((item) => item.parentId === topLevelItems[0].id);
    if (childItems.length > 0) {
      return summarizeTaskNames(childItems.map((item) => resolveTaskItemName(item, fileNodeMap)));
    }
  }

  return summarizeTaskNames(topLevelItems.map((item) => resolveTaskItemName(item, fileNodeMap)));
}

function getTaskIssues(task: TaskRecord, issues: IssueRecord[]) {
  const ownIssues = issues.filter((issue) => issue.taskId === task.id);
  if (ownIssues.length > 0) {
    return ownIssues;
  }
  return issues.filter((issue) => task.issueIds?.includes(issue.id));
}

function resolveIssueTone(issues: IssueRecord[]) {
  if (issues.some((issue) => issue.severity === 'critical')) return 'critical';
  if (issues.some((issue) => issue.severity === 'warning')) return 'warning';
  if (issues.some((issue) => issue.severity === 'success')) return 'success';
  return 'info';
}

function getRelatedTaskItems(items: TaskItemRecord[], taskId: string) {
  return items.filter((item) => item.taskId === taskId);
}

function getDisplayTaskItems(items: TaskItemRecord[], taskId: string) {
  const fileItems = items.filter((item) => item.taskId === taskId && (item.kind === 'file' || item.kind === undefined));
  if (fileItems.length > 0) {
    return fileItems;
  }
  return items.filter((item) => item.taskId === taskId && !item.parentId);
}

function matchesSearch(task: TaskRecord, items: TaskItemRecord[], query: string, fileNodeMap: Map<string, FileNode>) {
  if (!query.trim()) {
    return true;
  }

  const keyword = query.trim().toLowerCase();
  const fields = [task.title, task.source, task.target, task.sourcePath, task.targetPath].filter(Boolean).join(' ').toLowerCase();
  if (fields.includes(keyword)) {
    return true;
  }

  return items.some((item) => {
    const itemFields = [
      resolveTaskItemName(item, fileNodeMap),
      resolveTaskItemPathValue(item, fileNodeMap),
      item.targetPath,
    ]
      .filter(Boolean)
      .join(' ')
      .toLowerCase();
    return itemFields.includes(keyword);
  });
}

function compareTransferTasks(left: TaskRecord, right: TaskRecord, sortValue: string) {
  if (sortValue === '名称') {
    return left.title.localeCompare(right.title, 'zh-CN');
  }

  if (sortValue === '进度') {
    return right.progress - left.progress || left.title.localeCompare(right.title, 'zh-CN');
  }

  if (sortValue === '优先级') {
    const leftPriority = PRIORITY_ORDER[left.priority ?? '普通优先级'];
    const rightPriority = PRIORITY_ORDER[right.priority ?? '普通优先级'];
    return leftPriority - rightPriority || left.title.localeCompare(right.title, 'zh-CN');
  }

  const leftStatus = STATUS_ORDER[left.status] ?? 99;
  const rightStatus = STATUS_ORDER[right.status] ?? 99;
  if (leftStatus !== rightStatus) {
    return leftStatus - rightStatus;
  }

  const leftPriority = PRIORITY_ORDER[left.priority ?? '普通优先级'];
  const rightPriority = PRIORITY_ORDER[right.priority ?? '普通优先级'];
  if (leftPriority !== rightPriority) {
    return leftPriority - rightPriority;
  }

  return left.title.localeCompare(right.title, 'zh-CN');
}

function renderPriority(priority?: TaskPriority) {
  return priority ?? '普通优先级';
}

function compactPathLabel(path: string, maxLength = 42) {
  if (path.length <= maxLength) {
    return path;
  }

  const headLength = Math.max(12, Math.ceil(maxLength * 0.58));
  const tailLength = Math.max(10, maxLength - headLength - 3);
  return `${path.slice(0, headLength)}...${path.slice(-tailLength)}`;
}

function resolveTaskSummary(task: TaskRecord) {
  return `${task.fileCount} 个文件 · ${task.folderCount ?? 0} 个文件夹`;
}

function resolveTaskInlinePath(task: TaskRecord, items: TaskItemRecord[], fileNodeMap: Map<string, FileNode>) {
  const pathCandidates = new Set<string>();

  (task.fileNodeIds ?? []).forEach((id) => {
    const path = fileNodeMap.get(id)?.path;
    if (path) {
      pathCandidates.add(path);
    }
  });

  items.forEach((item) => {
    const path = resolveTaskItemPathValue(item, fileNodeMap);
    if (path) {
      pathCandidates.add(path);
    }
  });

  if (pathCandidates.size === 0) {
    const fallbackPath = task.sourcePath ?? task.targetPath;
    if (fallbackPath) {
      pathCandidates.add(fallbackPath);
    }
  }

  if (pathCandidates.size !== 1) {
    return null;
  }

  const [path] = Array.from(pathCandidates);
  return {
    full: path,
    display: compactPathLabel(path),
  };
}

function resolveTaskProgressMeta(task: TaskRecord) {
  if (!task.source || !task.target) {
    return null;
  }

  return {
    full: `${task.source} → ${task.target}`,
    display: `${task.source} → ${task.target}`,
  };
}

function resolveTaskItemPath(item: TaskItemRecord, fileNodeMap: Map<string, FileNode>) {
  const path = resolveTaskItemPathValue(item, fileNodeMap);
  if (!path) {
    return null;
  }

  return {
    full: path,
    display: compactPathLabel(path),
  };
}

function canPause(task: TaskRecord) {
  return !['已完成', '已取消', '已暂停'].includes(task.status);
}

function canResume(task: TaskRecord) {
  return task.status === '已暂停';
}

function canRetry(task: TaskRecord) {
  return ['失败', '异常待处理', '已取消'].includes(task.status);
}

function canCancel(task: TaskRecord) {
  return !['已完成', '已取消'].includes(task.status);
}

function canPauseItem(item: TaskItemRecord) {
  return !['已完成', '已取消', '已暂停'].includes(item.status);
}

function canResumeItem(item: TaskItemRecord) {
  return item.status === '已暂停';
}

function canCancelItem(item: TaskItemRecord) {
  return !['已完成', '已取消'].includes(item.status);
}

function getEffectiveSelectedTransferCount(taskIds: string[], taskItemIds: string[], items: TaskItemRecord[]) {
  const effectiveItemIds = new Set(taskItemIds);
  const rootOnlyTaskIds = new Set<string>();

  taskIds.forEach((taskId) => {
    const displayItems = getDisplayTaskItems(items, taskId);
    if (displayItems.length === 0) {
      rootOnlyTaskIds.add(taskId);
      return;
    }

    displayItems.forEach((item) => effectiveItemIds.add(item.id));
  });

  return effectiveItemIds.size + rootOnlyTaskIds.size;
}

export function TaskCenterPage(props: {
  activeTab: TaskTab;
  fileNodes: FileNode[];
  issues: IssueRecord[];
  libraries: Library[];
  preselectedTaskIds: string[] | null;
  statusFilter: string;
  taskItems: TaskItemRecord[];
  tasks: TaskRecord[];
  onChangeTaskPriority: (ids: string[], priority: TaskPriority) => void;
  onChangeTaskItemStatus: (ids: string[], action: TaskItemStatusAction) => void;
  onChangeTaskStatus: (ids: string[], action: TaskStatusAction) => void;
  onOpenIssueCenterForIssue: (issue: IssueRecord) => void;
  onOpenIssueCenterForTask: (task: TaskRecord) => void;
  onOpenTaskDetail: (value: TaskRecord | null) => void;
  onConsumePreselectedTaskIds: () => void;
  onSetActiveTab: (value: TaskTab) => void;
  onSetTaskStatusFilter: (value: string) => void;
}) {
  const {
    activeTab,
    fileNodes,
    issues,
    libraries,
    preselectedTaskIds,
    statusFilter,
    taskItems,
    tasks,
    onChangeTaskPriority,
    onChangeTaskItemStatus,
    onChangeTaskStatus,
    onOpenIssueCenterForIssue,
    onOpenIssueCenterForTask,
    onOpenTaskDetail,
    onConsumePreselectedTaskIds,
    onSetActiveTab,
    onSetTaskStatusFilter,
  } = props;

  const fileNodeMap = useMemo(() => buildFileNodeMap(fileNodes), [fileNodes]);

  const [businessFilter, setBusinessFilter] = useState<TransferBusinessType>('SYNC');
  const [syncLinkFilter, setSyncLinkFilter] = useState('全部');
  const [libraryFilter, setLibraryFilter] = useState('全部');
  const [sortValue, setSortValue] = useState('默认排序');
  const [query, setQuery] = useState('');
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState<(typeof PAGE_SIZE_OPTIONS)[number]>(20);
  const [expandedTaskIds, setExpandedTaskIds] = useState<string[]>([]);
  const [selectedTaskIds, setSelectedTaskIds] = useState<string[]>([]);
  const [selectedTaskItemIds, setSelectedTaskItemIds] = useState<string[]>([]);
  const [issuePopoverTaskId, setIssuePopoverTaskId] = useState<string | null>(null);
  const [taskItemDetail, setTaskItemDetail] = useState<TaskItemRecord | null>(null);

  useEffect(() => {
    setSelectedTaskIds([]);
    setSelectedTaskItemIds([]);
    setIssuePopoverTaskId(null);
    setTaskItemDetail(null);
    setCurrentPage(1);
  }, [activeTab, businessFilter, libraryFilter, query, sortValue, statusFilter, syncLinkFilter]);

  useEffect(() => {
    if (!issuePopoverTaskId) {
      return;
    }

    const handlePointerDown = (event: MouseEvent) => {
      const target = event.target as HTMLElement | null;
      if (!target) return;
      if (target.closest('.task-issue-popover') || target.closest('.task-issue-anchor')) {
        return;
      }
      setIssuePopoverTaskId(null);
    };

    document.addEventListener('mousedown', handlePointerDown);
    return () => document.removeEventListener('mousedown', handlePointerDown);
  }, [issuePopoverTaskId]);

  useEffect(() => {
    if (!taskItemDetail) {
      return;
    }

    const nextTaskItemDetail = taskItems.find((item) => item.id === taskItemDetail.id) ?? null;
    if (!nextTaskItemDetail) {
      setTaskItemDetail(null);
      return;
    }

    setTaskItemDetail((current) => {
      if (!current || current.id !== nextTaskItemDetail.id) {
        return current;
      }

      return JSON.stringify(current) === JSON.stringify(nextTaskItemDetail) ? current : nextTaskItemDetail;
    });
  }, [taskItemDetail?.id, taskItems]);

  useEffect(() => {
    if (!preselectedTaskIds || preselectedTaskIds.length === 0) {
      return;
    }

    const nextTaskIds = Array.from(new Set(preselectedTaskIds));
    const nextTaskItemIds = taskItems
      .filter((item) => nextTaskIds.includes(item.taskId))
      .map((item) => item.id);

    setSelectedTaskIds(nextTaskIds);
    setSelectedTaskItemIds(nextTaskItemIds);
    onConsumePreselectedTaskIds();
  }, [onConsumePreselectedTaskIds, preselectedTaskIds, taskItems]);

  const transferTasks = useMemo(
    () => tasks.filter((task) => task.kind === 'transfer'),
    [tasks],
  );

  const otherTasks = useMemo(
    () =>
      tasks.filter((task) => {
        if (task.kind !== 'other') {
          return false;
        }
        const normalizedStatusFilter = statusFilter === '暂停任务' ? '已暂停' : statusFilter;
        return normalizedStatusFilter === '全部' || task.status === normalizedStatusFilter;
      }),
    [statusFilter, tasks],
  );

  const visibleTransferTasks = useMemo(() => {
    return transferTasks
      .filter((task) => {
        const businessType = resolveBusinessType(task);
        if (!businessType || businessType !== businessFilter) {
          return false;
        }

        if (businessFilter === 'SYNC' && syncLinkFilter !== '全部' && resolveLinkLabel(resolveLinkType(task)) !== syncLinkFilter) {
          return false;
        }

        const normalizedStatusFilter = statusFilter === '暂停任务' ? '已暂停' : statusFilter;
        if (normalizedStatusFilter !== '全部' && task.status !== normalizedStatusFilter) {
          return false;
        }

        if (libraryFilter !== '全部') {
          const library = libraries.find((item) => item.id === task.libraryId);
          if (library?.name !== libraryFilter) {
            return false;
          }
        }

        return matchesSearch(task, getRelatedTaskItems(taskItems, task.id), query, fileNodeMap);
      })
      .sort((left, right) => compareTransferTasks(left, right, sortValue));
  }, [businessFilter, fileNodeMap, libraries, libraryFilter, query, sortValue, statusFilter, syncLinkFilter, taskItems, transferTasks]);

  const selectedTasks = visibleTransferTasks.filter((task) => selectedTaskIds.includes(task.id));
  const selectedTaskItems = taskItems.filter((item) => selectedTaskItemIds.includes(item.id));
  const transferPageCount = Math.max(1, Math.ceil(visibleTransferTasks.length / pageSize));
  const otherPageCount = Math.max(1, Math.ceil(otherTasks.length / pageSize));
  const pageCount = activeTab === 'transfer' ? transferPageCount : otherPageCount;

  useEffect(() => {
    setCurrentPage((current) => Math.min(current, pageCount));
  }, [pageCount]);

  const pagedTransferTasks = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return visibleTransferTasks.slice(start, start + pageSize);
  }, [currentPage, pageSize, visibleTransferTasks]);

  const pagedOtherTasks = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return otherTasks.slice(start, start + pageSize);
  }, [currentPage, otherTasks, pageSize]);

  const allVisibleSelected =
    pagedTransferTasks.length > 0 && pagedTransferTasks.every((task) => selectedTaskIds.includes(task.id));

  const selectedTransferEntryCount = getEffectiveSelectedTransferCount(selectedTaskIds, selectedTaskItemIds, taskItems);
  const batchPauseTaskIds = selectedTasks.filter(canPause).map((task) => task.id);
  const batchPauseTaskItemIds = selectedTaskItems
    .filter((item) => !selectedTaskIds.includes(item.taskId))
    .filter(canPauseItem)
    .map((item) => item.id);
  const batchResumeTaskIds = selectedTasks.filter(canResume).map((task) => task.id);
  const batchResumeTaskItemIds = selectedTaskItems
    .filter((item) => !selectedTaskIds.includes(item.taskId))
    .filter(canResumeItem)
    .map((item) => item.id);
  const batchRetryTaskIds = selectedTasks.filter(canRetry).map((task) => task.id);
  const batchCancelTaskIds = selectedTasks.filter(canCancel).map((task) => task.id);
  const batchCancelTaskItemIds = selectedTaskItems
    .filter((item) => !selectedTaskIds.includes(item.taskId))
    .filter(canCancelItem)
    .map((item) => item.id);

  const canBatchPause = batchPauseTaskIds.length + batchPauseTaskItemIds.length > 0;
  const canBatchResume = batchResumeTaskIds.length + batchResumeTaskItemIds.length > 0;
  const canBatchRetry = batchRetryTaskIds.length > 0;
  const canBatchCancel = batchCancelTaskIds.length + batchCancelTaskItemIds.length > 0;
  const canBatchPriority = selectedTaskIds.length > 0;

  function toggleTaskSelection(taskId: string) {
    const displayItems = getDisplayTaskItems(taskItems, taskId);
    const itemIds = displayItems.map((item) => item.id);
    const isSelected = selectedTaskIds.includes(taskId);
    setSelectedTaskIds((current) =>
      isSelected ? current.filter((item) => item !== taskId) : [...current, taskId],
    );
    setSelectedTaskItemIds((current) =>
      isSelected
        ? current.filter((id) => !itemIds.includes(id))
        : Array.from(new Set([...current, ...itemIds])),
    );
  }

  function toggleTaskItemSelection(taskId: string, taskItemId: string) {
    setSelectedTaskIds((current) => current.filter((item) => item !== taskId));
    setSelectedTaskItemIds((current) =>
      current.includes(taskItemId) ? current.filter((item) => item !== taskItemId) : [...current, taskItemId],
    );
  }

  function toggleSelectVisible() {
    const visibleChildIds = pagedTransferTasks.flatMap((task) => getDisplayTaskItems(taskItems, task.id).map((item) => item.id));
    setSelectedTaskIds((current) =>
      allVisibleSelected
        ? current.filter((id) => !pagedTransferTasks.some((task) => task.id === id))
        : Array.from(new Set([...current, ...pagedTransferTasks.map((task) => task.id)])),
    );
    setSelectedTaskItemIds((current) =>
      allVisibleSelected
        ? current.filter((id) => !visibleChildIds.includes(id))
        : Array.from(new Set([...current, ...visibleChildIds])),
    );
  }

  function toggleExpanded(taskId: string) {
    setExpandedTaskIds((current) =>
      current.includes(taskId) ? current.filter((item) => item !== taskId) : [...current, taskId],
    );
  }

  function renderOtherTasks() {
    if (otherTasks.length === 0) {
      return <EmptyState title="当前没有其它任务" description="删除、修复、校验等非传输任务会汇总在这里。" />;
    }

    return pagedOtherTasks.map((task) => (
      <article className="list-row task-row" key={task.id}>
        <div className="row-main">
          <strong>{task.title}</strong>
          <span>{task.type}</span>
          <p>{task.eta}</p>
        </div>
        <span>{task.fileCount} 项</span>
        <span>{task.speed}</span>
        <span>{task.eta}</span>
        <TonePill tone={task.statusTone}>{task.status}</TonePill>
        <div className="row-progress">
          <ProgressBar value={task.progress} />
        </div>
        <div className="row-actions">
          <ActionButton onClick={() => onOpenTaskDetail(task)}>详情</ActionButton>
          {canRetry(task) ? <ActionButton onClick={() => onChangeTaskStatus([task.id], 'retry')}>重试</ActionButton> : null}
        </div>
      </article>
    ));
  }

  function renderPagination(total: number) {
    return (
      <div className="storage-pagination task-pagination">
        <span className="selection-caption">页 {currentPage}/{pageCount}</span>
        <div className="storage-pagination-controls">
          <IconButton ariaLabel="首页" onClick={() => setCurrentPage(1)}>
            <ChevronLeft size={14} />
            <ChevronLeft size={14} />
          </IconButton>
          <IconButton ariaLabel="上一页" onClick={() => setCurrentPage(Math.max(1, currentPage - 1))}>
            <ChevronLeft size={14} />
          </IconButton>
          <button className="storage-page-chip active" type="button">
            {currentPage}
          </button>
          <IconButton ariaLabel="下一页" onClick={() => setCurrentPage(Math.min(pageCount, currentPage + 1))}>
            <ChevronRight size={14} />
          </IconButton>
          <IconButton ariaLabel="末页" onClick={() => setCurrentPage(pageCount)}>
            <ChevronRight size={14} />
            <ChevronRight size={14} />
          </IconButton>
        </div>
        <label className="select-pill storage-page-size">
          <select
            aria-label="每页数量"
            value={String(pageSize)}
            onChange={(event) => {
              setPageSize(Number(event.target.value) as (typeof PAGE_SIZE_OPTIONS)[number]);
              setCurrentPage(1);
            }}
          >
            {PAGE_SIZE_OPTIONS.map((option) => (
              <option key={option} value={option}>
                {option}
              </option>
            ))}
          </select>
        </label>
        <span className="selection-caption">项/页，共 {total} 项</span>
      </div>
    );
  }

  return (
    <section className="page-stack task-center-page">
      <div className="toolbar-card action-toolbar task-center-toolbar">
        <div className="toolbar-group wrap task-toolbar-main">
          <TabSwitch
            items={[
              { id: 'transfer', label: '传输任务' },
              { id: 'other', label: '其它任务' },
            ]}
            value={activeTab}
            onChange={(value) => onSetActiveTab(value as TaskTab)}
          />

          {activeTab === 'transfer' ? (
            <>
              <TabSwitch items={TRANSFER_BUSINESS_ITEMS} value={businessFilter} onChange={(value) => setBusinessFilter(value as TransferBusinessType)} />
              {businessFilter === 'SYNC' ? (
                <SelectPill ariaLabel="同步链路" options={SYNC_LINK_OPTIONS} value={syncLinkFilter} onChange={setSyncLinkFilter} />
              ) : null}
              <SelectPill ariaLabel="任务状态" options={TRANSFER_STATUS_OPTIONS} value={statusFilter} onChange={onSetTaskStatusFilter} />
              <SelectPill
                ariaLabel="资产库"
                options={['全部', ...libraries.map((item) => item.name)]}
                value={libraryFilter}
                onChange={setLibraryFilter}
              />
              <SelectPill ariaLabel="排序方式" options={SORT_OPTIONS} value={sortValue} onChange={setSortValue} />
            </>
          ) : (
            <SelectPill ariaLabel="任务状态" options={OTHER_STATUS_OPTIONS} value={statusFilter} onChange={onSetTaskStatusFilter} />
          )}
        </div>

        {activeTab === 'transfer' ? (
          <label className="search-field task-search" htmlFor="task-search">
            <Search size={15} />
            <input
              id="task-search"
              aria-label="搜索任务"
              placeholder="搜索任务名称、路径、文件名"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
            />
          </label>
        ) : null}
      </div>

      {activeTab === 'transfer' && selectedTransferEntryCount > 0 ? (
        <div className="toolbar-card selection-toolbar">
          <span className="selection-caption">已选择 {selectedTransferEntryCount} 个任务项</span>
          <div className="row-actions">
            <ActionButton
              onClick={() => {
                if (batchPauseTaskIds.length > 0) onChangeTaskStatus(batchPauseTaskIds, 'pause');
                if (batchPauseTaskItemIds.length > 0) onChangeTaskItemStatus(batchPauseTaskItemIds, 'pause');
              }}
              disabled={!canBatchPause}
            >
              批量暂停
            </ActionButton>
            <ActionButton
              onClick={() => {
                if (batchResumeTaskIds.length > 0) onChangeTaskStatus(batchResumeTaskIds, 'resume');
                if (batchResumeTaskItemIds.length > 0) onChangeTaskItemStatus(batchResumeTaskItemIds, 'resume');
              }}
              disabled={!canBatchResume}
            >
              批量继续
            </ActionButton>
            <ActionButton onClick={() => onChangeTaskStatus(batchRetryTaskIds, 'retry')} disabled={!canBatchRetry}>
              批量重试
            </ActionButton>
            <ActionButton
              onClick={() => {
                if (batchCancelTaskIds.length > 0) onChangeTaskStatus(batchCancelTaskIds, 'cancel');
                if (batchCancelTaskItemIds.length > 0) onChangeTaskItemStatus(batchCancelTaskItemIds, 'cancel');
              }}
              disabled={!canBatchCancel}
              tone="danger"
            >
              批量取消
            </ActionButton>
            <ActionButton onClick={() => onChangeTaskPriority(selectedTaskIds, '高优先级')} disabled={!canBatchPriority}>
              批量设为高优先级
            </ActionButton>
            <ActionButton
              onClick={() => {
                setSelectedTaskIds([]);
                setSelectedTaskItemIds([]);
              }}
            >
              清空选择
            </ActionButton>
          </div>
        </div>
      ) : null}

      <div className="workspace-card compact-list transfer-task-board">
        {activeTab === 'other' ? (
          renderOtherTasks()
        ) : visibleTransferTasks.length === 0 ? (
          <EmptyState title="当前没有匹配的传输任务" description="可以调整筛选条件，或去导入中心、文件中心继续发起传输任务。" />
        ) : (
          <>
            <div className="transfer-task-list-header">
              <label className="transfer-task-check">
                <input
                  aria-label="选择全部可见任务"
                  checked={allVisibleSelected}
                  type="checkbox"
                  onChange={toggleSelectVisible}
                />
                <span>全选</span>
              </label>
            </div>

            {pagedTransferTasks.map((task) => {
              const taskIssueRecords = getTaskIssues(task, issues);
              const relatedItems = getRelatedTaskItems(taskItems, task.id);
              const listLevelItems = task.multiFile ? getDisplayTaskItems(taskItems, task.id) : [];
              const hasSecondaryItems = listLevelItems.length > 0;
              const expanded = hasSecondaryItems && expandedTaskIds.includes(task.id);
              const displayTitle = resolveTaskDisplayTitle(task, relatedItems, fileNodeMap);
              const linkLabel = resolveLinkLabel(resolveLinkType(task));
              const issueTone = resolveIssueTone(taskIssueRecords);
              const inlinePath = resolveTaskInlinePath(task, relatedItems, fileNodeMap);
              const progressMeta = resolveTaskProgressMeta(task);

              return (
                <article className="transfer-task-card" key={task.id}>
                  <div className="transfer-task-row">
                    <label className="transfer-task-check">
                      <input
                        aria-label={`选择 ${displayTitle}`}
                        checked={selectedTaskIds.includes(task.id)}
                        type="checkbox"
                        onChange={() => toggleTaskSelection(task.id)}
                      />
                    </label>

                    {hasSecondaryItems ? (
                      <button
                        aria-label={`${expanded ? '收起' : '展开'} ${displayTitle}`}
                        className="transfer-expand-button"
                        type="button"
                        onClick={() => toggleExpanded(task.id)}
                      >
                        {expanded ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                      </button>
                    ) : (
                      <span className="transfer-expand-placeholder" aria-hidden="true" />
                    )}

                    <div className="transfer-task-main">
                      <strong className="transfer-task-title">{displayTitle}</strong>
                      {inlinePath ? (
                        <span className="transfer-task-inline-path" title={inlinePath.full}>
                          {inlinePath.display}
                        </span>
                      ) : null}
                      <span className="transfer-task-summary">{resolveTaskSummary(task)}</span>
                    </div>

                    <div className="transfer-task-metrics">
                      <div className="transfer-task-metric-head">
                        <div className="transfer-task-tag-group">
                          <span className="transfer-size-pill">{task.totalSize ?? '大小计算中'}</span>
                          {linkLabel ? <span className="transfer-link-pill">{linkLabel}</span> : null}
                          <span className="transfer-priority-pill">{renderPriority(task.priority)}</span>
                        </div>
                        <strong>{task.progress}%</strong>
                      </div>
                      <div className="row-progress">
                        <ProgressBar value={task.progress} />
                      </div>
                      {progressMeta ? (
                        <span className="transfer-progress-meta" title={progressMeta.full}>
                          {progressMeta.display}
                        </span>
                      ) : null}
                    </div>

                    <div className="transfer-task-side">
                      <span className="transfer-task-speed">{task.speed}</span>
                      <span>{task.eta}</span>
                      <div className="transfer-task-status-row">
                        <TonePill tone={task.statusTone}>{task.status}</TonePill>
                        <div className="task-issue-anchor">
                          {taskIssueRecords.length > 0 ? (
                            <button
                              aria-label={`查看异常 ${displayTitle}`}
                              className={`task-issue-badge ${issueTone}`}
                              type="button"
                              onClick={() => setIssuePopoverTaskId((current) => (current === task.id ? null : task.id))}
                            >
                              <AlertTriangle size={14} />
                              <span>{taskIssueRecords.length > 99 ? '99+' : taskIssueRecords.length}</span>
                            </button>
                          ) : null}
                        </div>
                      </div>
                    </div>

                    <div className="row-actions transfer-task-actions">
                      <ActionButton onClick={() => onOpenTaskDetail({ ...task, title: displayTitle })}>详情</ActionButton>
                      {canResume(task) ? (
                        <ActionButton onClick={() => onChangeTaskStatus([task.id], 'resume')}>继续</ActionButton>
                      ) : (
                        <ActionButton disabled={!canPause(task)} onClick={() => onChangeTaskStatus([task.id], 'pause')}>暂停</ActionButton>
                      )}
                      {canRetry(task) ? <ActionButton onClick={() => onChangeTaskStatus([task.id], 'retry')}>重试</ActionButton> : null}
                      <ActionButton disabled={!canCancel(task)} tone="danger" onClick={() => onChangeTaskStatus([task.id], 'cancel')}>取消</ActionButton>
                    </div>
                  </div>

                  {issuePopoverTaskId === task.id ? (
                    <div className="task-issue-popover" role="dialog" aria-label={`异常 ${task.title}`}>
                      <div className="task-issue-popover-header">
                        <strong>当前任务异常</strong>
                        <span>{taskIssueRecords.length} 条</span>
                      </div>
                      <div className="task-issue-list">
                        {taskIssueRecords.map((issue) => (
                          <button
                            key={issue.id}
                            className="task-issue-item"
                            type="button"
                            onClick={() => onOpenIssueCenterForIssue(issue)}
                          >
                            <span className={`tone-text-${issue.severity}`}>{issue.type}</span>
                            <strong>{issue.asset}</strong>
                            <p>{issue.detail}</p>
                          </button>
                        ))}
                      </div>
                      <div className="sheet-actions right">
                        <ActionButton onClick={() => onOpenIssueCenterForTask({ ...task, title: displayTitle })}>处置所有异常</ActionButton>
                      </div>
                    </div>
                  ) : null}

                  {expanded ? (
                    <div className="transfer-children-panel">
                      {listLevelItems.length === 0 ? (
                        <p className="muted-paragraph">当前任务还没有可展示的二级任务。</p>
                      ) : (
                        listLevelItems.map((item) => {
                          const childIssueCount = issues.filter((issue) => issue.taskItemId === item.id || item.issueIds?.includes(issue.id)).length;
                          const childName = resolveTaskItemName(item, fileNodeMap);
                          const childPath = resolveTaskItemPath(item, fileNodeMap);
                          const childSize = resolveTaskItemSize(item, fileNodeMap);
                          return (
                            <div className="transfer-task-row transfer-child-task-row" key={item.id}>
                              <label className="transfer-task-check">
                                <input
                                  aria-label={`选择 ${childName}`}
                                  checked={selectedTaskItemIds.includes(item.id)}
                                  type="checkbox"
                                  onChange={() => toggleTaskItemSelection(task.id, item.id)}
                                />
                              </label>
                              <span className="transfer-expand-placeholder" aria-hidden="true" />

                              <div className="transfer-task-main transfer-child-main">
                                <strong className="transfer-task-title">{childName}</strong>
                                {childPath ? (
                                  <span className="transfer-task-path" title={childPath.full}>
                                    {childPath.display}
                                  </span>
                                ) : null}
                              </div>

                              <div className="transfer-task-metrics">
                                <div className="transfer-task-metric-head">
                                  <div className="transfer-task-tag-group">
                                    <span className="transfer-size-pill">{childSize}</span>
                                    <span className="transfer-priority-pill">{renderPriority(item.priority)}</span>
                                  </div>
                                  <strong>{item.progress}%</strong>
                                </div>
                                <div className="row-progress">
                                  <ProgressBar value={item.progress} />
                                </div>
                              </div>

                              <div className="transfer-task-side">
                                <span className="transfer-task-speed">{item.speed}</span>
                                <span>{item.phase ?? '待处理'}</span>
                                <div className="transfer-task-status-row">
                                  <TonePill tone={item.statusTone}>{item.status}</TonePill>
                                  <div className="task-issue-anchor">
                                    {childIssueCount > 0 ? (
                                      <span className="task-issue-badge critical">
                                        <AlertTriangle size={14} />
                                        <span>{childIssueCount > 99 ? '99+' : childIssueCount}</span>
                                      </span>
                                    ) : null}
                                  </div>
                                </div>
                              </div>

                              <div className="row-actions transfer-task-actions transfer-child-actions">
                                <ActionButton onClick={() => setTaskItemDetail(item)}>详情</ActionButton>
                                {canResumeItem(item) ? (
                                  <ActionButton onClick={() => onChangeTaskItemStatus([item.id], 'resume')}>继续</ActionButton>
                                ) : (
                                  <ActionButton disabled={!canPauseItem(item)} onClick={() => onChangeTaskItemStatus([item.id], 'pause')}>暂停</ActionButton>
                                )}
                                <ActionButton disabled={!canCancelItem(item)} tone="danger" onClick={() => onChangeTaskItemStatus([item.id], 'cancel')}>取消</ActionButton>
                              </div>
                            </div>
                          );
                        })
                      )}
                    </div>
                  ) : null}
                </article>
              );
            })}
            {renderPagination(visibleTransferTasks.length)}
          </>
        )}
        {activeTab === 'other' && otherTasks.length > 0 ? renderPagination(otherTasks.length) : null}
      </div>
      {taskItemDetail ? (
        <TaskItemDetailSheet
          fileNodes={fileNodes}
          issues={issues}
          item={taskItemDetail}
          onChangeTaskItemStatus={onChangeTaskItemStatus}
          onClose={() => setTaskItemDetail(null)}
        />
      ) : null}
    </section>
  );
}

function TaskItemDetailSheet({
  fileNodes,
  issues,
  item,
  onChangeTaskItemStatus,
  onClose,
}: {
  fileNodes: FileNode[];
  issues: IssueRecord[];
  item: TaskItemRecord;
  onChangeTaskItemStatus: (ids: string[], action: TaskItemStatusAction) => void;
  onClose: () => void;
}) {
  const itemIssues = issues.filter((issue) => issue.taskItemId === item.id || item.issueIds?.includes(issue.id));
  const fileNodeMap = useMemo(() => buildFileNodeMap(fileNodes), [fileNodes]);
  const itemName = resolveTaskItemName(item, fileNodeMap);
  const itemSize = resolveTaskItemSize(item, fileNodeMap);
  const itemSourcePath = resolveTaskItemPathValue(item, fileNodeMap) ?? '—';
  const itemTargetPath = item.targetPath ?? '—';

  return (
    <Sheet onClose={onClose} title={itemName}>
      <div className="sheet-section">
        <div className="row-actions">
          {canResumeItem(item) ? (
            <ActionButton onClick={() => onChangeTaskItemStatus([item.id], 'resume')}>继续</ActionButton>
          ) : (
            <ActionButton disabled={!canPauseItem(item)} onClick={() => onChangeTaskItemStatus([item.id], 'pause')}>暂停</ActionButton>
          )}
          <ActionButton disabled={!canCancelItem(item)} tone="danger" onClick={() => onChangeTaskItemStatus([item.id], 'cancel')}>取消</ActionButton>
        </div>
      </div>

      <div className="sheet-section">
        <DenseRow label="节点类型" value={item.kind === 'group' ? '子任务分组' : item.kind === 'folder' ? '文件夹' : '文件'} />
        <DenseRow label="状态" tone={item.statusTone} value={item.status} />
        <DenseRow label="优先级" value={renderPriority(item.priority)} />
        <DenseRow label="当前阶段" value={item.phase ?? '—'} />
        <DenseRow label="大小" value={itemSize} />
        <DenseRow label="速度" value={item.speed} />
        <DenseRow label="进度" value={`${item.progress}%`} />
        <DenseRow label="源路径" value={itemSourcePath} />
        <DenseRow label="目标路径" value={itemTargetPath} />
        <DenseRow label="异常数量" value={`${itemIssues.length}`} />
      </div>
    </Sheet>
  );
}

export function TaskDetailSheet({
  fileNodes,
  item,
  issues,
  items,
  onChangeTaskPriority,
  onChangeTaskStatus,
  onClose,
  onOpenFileCenterForTask,
  onOpenIssueCenterForTask,
}: {
  fileNodes: FileNode[];
  item: TaskRecord;
  issues: IssueRecord[];
  items: TaskItemRecord[];
  onChangeTaskPriority: (ids: string[], priority: TaskPriority) => void;
  onChangeTaskStatus: (ids: string[], action: TaskStatusAction) => void;
  onClose: () => void;
  onOpenFileCenterForTask: (task: TaskRecord) => void;
  onOpenIssueCenterForTask: (task: TaskRecord) => void;
}) {
  const itemIssues = getTaskIssues(item, issues);
  const isTransferTask = item.kind === 'transfer';
  const fileNodeMap = useMemo(() => buildFileNodeMap(fileNodes), [fileNodes]);
  const linkedNodePaths = (item.fileNodeIds ?? [])
    .map((id) => fileNodeMap.get(id)?.path)
    .filter((value): value is string => Boolean(value));
  const taskSourcePath = linkedNodePaths[0] ?? item.sourcePath ?? '—';

  return (
    <Sheet onClose={onClose} title={item.title}>
      <div className="sheet-section">
        <div className="row-actions">
          {canResume(item) ? (
            <ActionButton onClick={() => onChangeTaskStatus([item.id], 'resume')}>继续</ActionButton>
          ) : (
            <ActionButton disabled={!canPause(item)} onClick={() => onChangeTaskStatus([item.id], 'pause')}>暂停</ActionButton>
          )}
          {canRetry(item) ? <ActionButton onClick={() => onChangeTaskStatus([item.id], 'retry')}>重试</ActionButton> : null}
          <ActionButton disabled={!canCancel(item)} tone="danger" onClick={() => onChangeTaskStatus([item.id], 'cancel')}>取消</ActionButton>
          {item.priority !== '高优先级' ? (
            <ActionButton onClick={() => onChangeTaskPriority([item.id], '高优先级')}>设为高优先级</ActionButton>
          ) : (
            <ActionButton onClick={() => onChangeTaskPriority([item.id], '普通优先级')}>恢复普通优先级</ActionButton>
          )}
        </div>
      </div>

      <div className="sheet-section">
        <DenseRow label="业务类型" value={item.kind === 'transfer' ? resolveBusinessLabel(item) : item.type} />
        <DenseRow label="链路类型" value={isTransferTask ? resolveLinkLabel(resolveLinkType(item)) || '—' : '—'} />
        <DenseRow label="状态" tone={item.statusTone} value={item.status} />
        <DenseRow label="优先级" value={renderPriority(item.priority)} />
        <DenseRow label="源端" value={item.source ?? '—'} />
        <DenseRow label="目标端" value={item.target ?? '—'} />
        <DenseRow label="源路径" value={taskSourcePath} />
        <DenseRow label="目标路径" value={item.targetPath ?? '—'} />
        <DenseRow label="文件数量" value={`${item.fileCount}`} />
        <DenseRow label="文件夹数量" value={`${item.folderCount ?? 0}`} />
        <DenseRow label="当前速度" value={item.speed} />
        <DenseRow label="剩余时间" value={item.eta} />
      </div>

      {!isTransferTask ? (
        <div className="workspace-card compact-list inner-list">
          {items.length === 0 ? (
            <EmptyState title="暂无子任务" description="当前任务没有拆分的文件级记录。" />
          ) : (
            items.map((taskItem) => (
              <div className="list-row task-detail-row" key={taskItem.id}>
                <div className="row-main">
                  <strong>{taskItem.name}</strong>
                  <span>{taskItem.size}</span>
                </div>
                <span>{taskItem.speed}</span>
                <TonePill tone={taskItem.statusTone}>{taskItem.status}</TonePill>
                <div className="row-progress">
                  <ProgressBar value={taskItem.progress} />
                </div>
              </div>
            ))
          )}
        </div>
      ) : null}

      <div className="sheet-section">
        <DenseRow label="异常数量" value={`${itemIssues.length}`} />
        <DenseRow label="最近异常" value={itemIssues[0]?.detail ?? '当前无异常'} />
      </div>

      <div className="sheet-actions right">
        <ActionButton onClick={() => onOpenFileCenterForTask(item)}>查看文件中心</ActionButton>
        <ActionButton onClick={() => onOpenIssueCenterForTask(item)}>查看异常中心</ActionButton>
      </div>
    </Sheet>
  );
}
