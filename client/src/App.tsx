import { startTransition, useDeferredValue, useEffect, useMemo, useState } from 'react';
import {
  AlertTriangle,
  Bell,
  CircleEllipsis,
  FolderInput,
  FolderOpen,
  HardDrive,
  Settings2,
} from 'lucide-react';
import type {
  FileNode,
  FileTypeFilter,
  Library,
  MainView,
  SettingsTab,
  Severity,
  StorageNode,
  TaskRecord,
  TaskTab,
} from './data';
import { navigationItems } from './data';
import {
  collectNodeIds,
  cloneSettingsRecord,
  createId,
  getDefaultPageSize,
  getSortableSize,
  loadPersistedState,
  resolveLibraryForImport,
  resolveThemeMode,
  STORAGE_KEY,
  type NotificationItem,
  type PersistedState,
} from './lib/clientState';
import { ActionButton, IconButton, LibraryManagerSheet, Sheet } from './components/Shared';
import { FileCenterPage } from './pages/FileCenterPage';
import { FileDetailSheet } from './pages/FileDetailSheet';
import { ImportCenterPage } from './pages/ImportCenterPage';
import { IssuesPage } from './pages/IssuesPage';
import { SettingsPage } from './pages/SettingsPage';
import { StorageNodesPage } from './pages/StorageNodesPage';
import { TaskCenterPage, TaskDetailSheet } from './pages/TaskCenterPage';

export type PageSize = 10 | 20 | 50 | 100;
export type ContextMenuTarget =
  | { type: 'file'; item: FileNode; x: number; y: number }
  | { type: 'task'; item: TaskRecord; x: number; y: number }
  | { type: 'storage'; item: StorageNode; x: number; y: number }
  | null;

type FeedbackState = { message: string; tone: Severity } | null;

export default function App() {
  const [persisted, setPersisted] = useState<PersistedState>(loadPersistedState);
  const [activeView, setActiveView] = useState<MainView>('file-center');
  const [activeLibraryId, setActiveLibraryId] = useState(persisted.libraries[0]?.id ?? 'photo');
  const [taskTab, setTaskTab] = useState<TaskTab>('transfer');
  const [settingsTab, setSettingsTab] = useState<SettingsTab>('general');
  const [fileTypeFilter, setFileTypeFilter] = useState<FileTypeFilter>('全部');
  const [taskStatusFilter, setTaskStatusFilter] = useState('全部');
  const [issueTypeFilter, setIssueTypeFilter] = useState('全部');
  const [searchText, setSearchText] = useState('');
  const [fileSort, setFileSort] = useState('修改时间');
  const deferredSearchText = useDeferredValue(searchText);
  const [pageSize, setPageSize] = useState<PageSize>(() => getDefaultPageSize(persisted.settings));
  const [currentPage, setCurrentPage] = useState(1);
  const [selectedImportBatchId, setSelectedImportBatchId] = useState(persisted.importBatches[0]?.id ?? '');
  const [selectedImportTargets, setSelectedImportTargets] = useState<Record<string, string[]>>(() =>
    Object.fromEntries(persisted.importSourceFiles.map((file) => [file.id, [...file.selectedTargets]])),
  );
  const [settingsDraft, setSettingsDraft] = useState(() => cloneSettingsRecord(persisted.settings));
  const [currentFolderId, setCurrentFolderId] = useState<string | null>(null);
  const [folderHistory, setFolderHistory] = useState<Array<string | null>>([null]);
  const [historyIndex, setHistoryIndex] = useState(0);
  const [selectedFileIds, setSelectedFileIds] = useState<string[]>([]);
  const [feedback, setFeedback] = useState<FeedbackState>(null);
  const [notificationsOpen, setNotificationsOpen] = useState(false);
  const [libraryMenuOpen, setLibraryMenuOpen] = useState(false);
  const [managedLibrary, setManagedLibrary] = useState<Library | null>(null);
  const [taskDetail, setTaskDetail] = useState<TaskRecord | null>(null);
  const [fileDetail, setFileDetail] = useState<FileNode | null>(null);
  const [folderDraft, setFolderDraft] = useState<string | null>(null);
  const unreadNotificationCount = useMemo(
    () => persisted.notifications.filter((item) => item.read === false).length,
    [persisted.notifications],
  );

  useEffect(() => {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(persisted));
  }, [persisted]);

  useEffect(() => {
    setSelectedImportTargets(
      Object.fromEntries(persisted.importSourceFiles.map((file) => [file.id, [...file.selectedTargets]])),
    );
    setSettingsDraft(cloneSettingsRecord(persisted.settings));
    setPageSize(getDefaultPageSize(persisted.settings));
  }, [persisted.importSourceFiles, persisted.settings]);

  useEffect(() => {
    if (!feedback) return;
    const timer = window.setTimeout(() => setFeedback(null), 3000);
    return () => window.clearTimeout(timer);
  }, [feedback]);

  useEffect(() => {
    setCurrentPage(1);
  }, [currentFolderId, deferredSearchText, fileSort, fileTypeFilter, pageSize]);

  const theme = useMemo(() => resolveThemeMode(persisted.settings), [persisted.settings]);
  const currentLibrary = useMemo(
    () => persisted.libraries.find((item) => item.id === activeLibraryId) ?? persisted.libraries[0],
    [activeLibraryId, persisted.libraries],
  );
  const currentLibraryNodes = useMemo(
    () => persisted.fileNodes.filter((node) => node.libraryId === activeLibraryId),
    [activeLibraryId, persisted.fileNodes],
  );
  const breadcrumbs = useMemo(() => {
    const trail = [{ id: null as string | null, label: currentLibrary.name }];
    let cursor = currentFolderId ? persisted.fileNodes.find((node) => node.id === currentFolderId) : undefined;
    const stack: Array<{ id: string; label: string }> = [];
    while (cursor) {
      stack.unshift({ id: cursor.id, label: cursor.name });
      const parentId = cursor.parentId;
      cursor = parentId ? persisted.fileNodes.find((node) => node.id === parentId) : undefined;
    }
    return trail.concat(stack);
  }, [currentFolderId, currentLibrary.name, persisted.fileNodes]);

  const currentEntries = useMemo(
    () =>
      currentLibraryNodes
        .filter((node) => node.parentId === currentFolderId)
        .filter((node) => (fileTypeFilter === '全部' ? true : node.fileKind === fileTypeFilter))
        .filter((node) => {
          const keyword = deferredSearchText.trim().toLowerCase();
          return keyword ? `${node.name} ${node.path} ${node.displayType}`.toLowerCase().includes(keyword) : true;
        })
        .toSorted((left, right) => {
          if (fileSort === '名称') return left.name.localeCompare(right.name, 'zh-CN');
          if (fileSort === '大小') return getSortableSize(right.size) - getSortableSize(left.size);
          return right.modifiedAt.localeCompare(left.modifiedAt, 'zh-CN');
        }),
    [currentFolderId, currentLibraryNodes, deferredSearchText, fileSort, fileTypeFilter],
  );
  const paginatedEntries = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return currentEntries.slice(start, start + pageSize);
  }, [currentEntries, currentPage, pageSize]);
  const pageCount = Math.max(1, Math.ceil(currentEntries.length / pageSize));
  const selectedBatch = useMemo(
    () => persisted.importBatches.find((batch) => batch.id === selectedImportBatchId) ?? persisted.importBatches[0],
    [persisted.importBatches, selectedImportBatchId],
  );
  const visibleImportFiles = useMemo(
    () => persisted.importSourceFiles.filter((file) => file.batchId === selectedBatch?.id),
    [persisted.importSourceFiles, selectedBatch],
  );
  const visibleTasks = useMemo(
    () =>
      persisted.taskRecords.filter(
        (task) =>
          task.kind === taskTab && (taskStatusFilter === '全部' || task.status === taskStatusFilter),
      ),
    [persisted.taskRecords, taskStatusFilter, taskTab],
  );
  const visibleIssues = useMemo(
    () =>
      persisted.issueRecords.filter(
        (item) => (issueTypeFilter === '全部' || item.type === issueTypeFilter) && item.status !== '已处理',
      ),
    [issueTypeFilter, persisted.issueRecords],
  );
  const commitState = (updater: (current: PersistedState) => PersistedState, nextFeedback?: FeedbackState) => {
    setPersisted((current) => {
      const updated = updater(current);
      if (!nextFeedback) return updated;
      const notice: NotificationItem = {
        id: createId('notice'),
        title: nextFeedback.message,
        detail: nextFeedback.message,
        tone: nextFeedback.tone,
        createdAt: '刚刚',
        read: false,
      };
      return { ...updated, notifications: [notice, ...updated.notifications].slice(0, 20) };
    });
    if (nextFeedback) setFeedback(nextFeedback);
  };

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

  const upsertMetadata = (
    metadata: Array<{ label: string; value: string }>,
    label: string,
    value: string,
  ) => {
    const exists = metadata.some((item) => item.label === label);
    if (exists) {
      return metadata.map((item) => (item.label === label ? { ...item, value } : item));
    }
    return [...metadata, { label, value }];
  };

  const createDeleteTasks = (
    current: PersistedState,
    items: FileNode[],
    mode: 'asset' | 'endpoint',
    endpointName?: string,
  ): PersistedState => {
    const newTasks = items.map((item) => ({
      id: createId('task'),
      kind: 'other' as const,
      title: mode === 'asset' ? `删除资产：${item.name}` : `端点删除：${item.name} @ ${endpointName}`,
      type: 'DELETE',
      status: mode === 'asset' ? '等待清理' : '已提交',
      statusTone: 'info' as Severity,
      libraryId: item.libraryId,
      source: mode === 'asset' ? '资产视图' : endpointName,
      target: mode === 'asset' ? 'ASSET_CLEANUP' : '指定端点',
      progress: 0,
      speed: '—',
      eta: mode === 'asset' ? '等待后台清理' : '等待删除执行',
      fileCount: 1,
      multiFile: false,
      updatedAt: '刚刚',
    }));

    return {
      ...current,
      taskRecords: [...newTasks, ...current.taskRecords],
    };
  };

  const markEndpointMissing = (item: FileNode, endpointName: string): FileNode => {
    const nextEndpoints = item.endpoints.map((endpoint) =>
      endpoint.name === endpointName ? { ...endpoint, state: '缺失', tone: 'critical' as Severity } : endpoint,
    );
    const allMissing = nextEndpoints.every((endpoint) => endpoint.state === '缺失');
    return {
      ...item,
      lifecycleState: allMissing ? 'PENDING_DELETE' : 'ACTIVE',
      endpoints: nextEndpoints,
      metadata: upsertMetadata(item.metadata, '生命周期', allMissing ? 'PENDING_DELETE' : 'ACTIVE'),
    };
  };

  const deleteAssets = (ids: string[]) => {
    const nodeIds = collectNodeIds(persisted.fileNodes, ids);
    const targets = persisted.fileNodes.filter((node) => nodeIds.includes(node.id) && node.type === 'file');
    if (targets.length === 0) {
      setFeedback({ message: '请先选择要删除的资产', tone: 'info' });
      return;
    }

    commitState(
      (current) => {
        const nextFiles = current.fileNodes.map((node) =>
          nodeIds.includes(node.id) && node.type === 'file'
            ? {
                ...node,
                lifecycleState: 'PENDING_DELETE' as const,
                endpoints: node.endpoints.map((endpoint) => ({
                  ...endpoint,
                  state: '缺失',
                  tone: 'critical' as Severity,
                })),
                metadata: upsertMetadata(node.metadata, '生命周期', 'PENDING_DELETE'),
              }
            : node,
        );
        return createDeleteTasks({ ...current, fileNodes: nextFiles }, targets, 'asset');
      },
      { message: '删除请求已提交，资产进入等待清理', tone: 'warning' },
    );
    setSelectedFileIds([]);
    setFileDetail(null);
  };

  const deleteFromEndpoint = (item: FileNode, endpointName: string) => {
    const updatedItem = markEndpointMissing(item, endpointName);
    commitState(
      (current) => {
        const nextFiles = current.fileNodes.map((node) =>
          node.id === item.id && node.type === 'file' ? markEndpointMissing(node, endpointName) : node,
        );
        return createDeleteTasks({ ...current, fileNodes: nextFiles }, [item], 'endpoint', endpointName);
      },
      { message: '已提交端点删除请求', tone: 'info' },
    );
    setFileDetail(updatedItem);
  };

  const validationMessage =
    visibleImportFiles.find((file) => (selectedImportTargets[file.id] ?? []).length === 0) != null
      ? '仍有文件未选择目标端，提交前需要补齐。'
      : null;

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
              <span>{currentLibrary.name}</span>
              <strong>{currentLibrary.rootLabel}</strong>
            </div>
          </button>
          {libraryMenuOpen ? (
            <div className="library-menu" role="menu">
              {persisted.libraries.map((library) => (
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
            </div>
          ) : null}
        </div>

        <nav className="sidebar-nav" aria-label="主导航">
          {navigationItems.map((item) => (
            <button
              key={item.id}
              aria-label={item.label}
              className={`nav-item${item.id === activeView ? ' active' : ''}`}
              type="button"
              onClick={() => startTransition(() => setActiveView(item.id))}
            >
              <span className="nav-icon">{navIcons[item.id]}</span>
              <span className="nav-label">{item.label}</span>
              {item.badge ? <span className="nav-badge">{item.badge}</span> : null}
            </button>
          ))}
        </nav>
      </aside>

      <main className="content-shell">
        <header className="page-header">
          <div className={`page-header-feedback${feedback ? ` ${feedback.tone}` : ''}`}>
            {feedback ? feedback.message : null}
          </div>
          <div className="page-header-actions">
            <IconButton ariaLabel="导入中心" onClick={() => setActiveView('import-center')}>
              <FolderInput size={16} />
            </IconButton>
            <div className="notification-trigger">
              <IconButton ariaLabel="通知" onClick={() => setNotificationsOpen(true)}>
                <Bell size={16} />
              </IconButton>
              {unreadNotificationCount > 0 ? <span className="notification-dot" aria-hidden="true" /> : null}
            </div>
          </div>
        </header>

        {activeView === 'file-center' ? (
          <FileCenterPage
            breadcrumbs={breadcrumbs.map((item) => item.label)}
            canGoBack={historyIndex > 0}
            canGoForward={historyIndex < folderHistory.length - 1}
            currentEntries={paginatedEntries}
            currentPage={currentPage}
            currentPathChildren={currentEntries.length}
            fileTypeFilter={fileTypeFilter}
            pageCount={pageCount}
            pageSize={pageSize}
            searchText={searchText}
            selectedIds={selectedFileIds}
            sortValue={fileSort}
            onChangeSort={setFileSort}
            onCreateFolder={() => setFolderDraft('')}
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
            onOpenItem={(item) => (item.type === 'folder' ? openFolder(item.id) : setFileDetail(item))}
            onOpenItemDetail={setFileDetail}
            onDeleteSelected={() => deleteAssets(selectedFileIds)}
            onRefreshIndex={() => setFeedback({ message: '已发起索引刷新', tone: 'info' })}
            onSetCurrentPage={setCurrentPage}
            onSetFileTypeFilter={setFileTypeFilter}
            onSetPageSize={setPageSize}
            onSetSearchText={(value) => startTransition(() => setSearchText(value))}
            onToggleSelect={(id) =>
              setSelectedFileIds((current) =>
                current.includes(id) ? current.filter((item) => item !== id) : [...current, id],
              )
            }
            onToggleSelectVisible={() => {
              const ids = paginatedEntries.map((item) => item.id);
              const allSelected = ids.every((id) => selectedFileIds.includes(id));
              setSelectedFileIds((current) =>
                allSelected ? current.filter((id) => !ids.includes(id)) : Array.from(new Set([...current, ...ids])),
              );
            }}
          />
        ) : null}

        {activeView === 'import-center' ? (
          <ImportCenterPage
            batches={persisted.importBatches}
            selectedBatch={selectedBatch}
            selectedImportTargets={selectedImportTargets}
            submitDisabled={selectedBatch?.status === '已提交'}
            validationMessage={validationMessage}
            visibleFiles={visibleImportFiles}
            onApplyTargetToAll={(target) =>
              setSelectedImportTargets((current) => ({
                ...current,
                ...Object.fromEntries(
                  visibleImportFiles.map((file) => [
                    file.id,
                    Array.from(new Set([...(current[file.id] ?? []), target])),
                  ]),
                ),
              }))
            }
            onSetSelectedBatchId={setSelectedImportBatchId}
            onSetSelectedImportTargets={setSelectedImportTargets}
            onSubmitImport={() => {
              const invalid = visibleImportFiles.find((file) => (selectedImportTargets[file.id] ?? []).length === 0);
              if (invalid) {
                setFeedback({ message: `请先为 ${invalid.name} 选择目标端`, tone: 'warning' });
                return;
              }
              if (selectedBatch?.status === '已提交') {
                setFeedback({ message: '当前批次已经提交', tone: 'info' });
                return;
              }
              const taskId = createId('task');
              commitState(
                (current) => ({
                  ...current,
                  importBatches: current.importBatches.map((batch) =>
                    batch.id === selectedBatch.id ? { ...batch, status: '已提交' } : batch,
                  ),
                  importSourceFiles: current.importSourceFiles.map((file) =>
                    file.batchId === selectedBatch.id
                      ? { ...file, selectedTargets: [...(selectedImportTargets[file.id] ?? [])], status: '已排队' }
                      : file,
                  ),
                  taskRecords: [
                    {
                      id: taskId,
                      kind: 'transfer',
                      title: `${selectedBatch.name} 入库`,
                      type: 'IMPORT',
                      status: '等待确认',
                      statusTone: 'info',
                      libraryId: resolveLibraryForImport(selectedBatch.id),
                      source: selectedBatch.source,
                      target: visibleImportFiles
                        .flatMap((file) => selectedImportTargets[file.id] ?? [])
                        .filter((item, index, array) => array.indexOf(item) === index)
                        .join('、'),
                      progress: 0,
                      speed: '—',
                      eta: '等待分配执行器',
                      fileCount: visibleImportFiles.length,
                      multiFile: visibleImportFiles.length > 1,
                      updatedAt: '刚刚',
                    },
                    ...current.taskRecords,
                  ],
                  taskItemRecords: [
                    ...visibleImportFiles.map((file) => ({
                      id: createId('task-item'),
                      taskId,
                      name: file.name,
                      status: '已排队',
                      statusTone: 'info' as Severity,
                      progress: 0,
                      size: file.size,
                      speed: '—',
                    })),
                    ...current.taskItemRecords,
                  ],
                }),
                { message: '已提交导入批次，任务已加入队列', tone: 'success' },
              );
            }}
          />
        ) : null}

        {activeView === 'task-center' ? (
          <TaskCenterPage
            activeTab={taskTab}
            statusFilter={taskStatusFilter}
            tasks={visibleTasks}
            onChangeTaskStatus={(taskId, action) =>
              commitState((current) => ({
                ...current,
                taskRecords: current.taskRecords.map((task) => {
                  if (task.id !== taskId) return task;
                  if (action === 'pause') return { ...task, status: '暂停中', statusTone: 'info', updatedAt: '刚刚' };
                  if (action === 'resume') return { ...task, status: '运行中', statusTone: 'warning', updatedAt: '刚刚' };
                  if (action === 'retry') return { ...task, status: '运行中', statusTone: 'warning', progress: 12, updatedAt: '刚刚' };
                  return { ...task, status: '已完成', statusTone: 'success', progress: 100, updatedAt: '刚刚' };
                }),
              }))
            }
            onOpenTaskDetail={setTaskDetail}
            onSetActiveTab={setTaskTab}
            onSetTaskStatusFilter={setTaskStatusFilter}
          />
        ) : null}

        {activeView === 'issues' ? (
          <IssuesPage
            issueTypeFilter={issueTypeFilter}
            items={visibleIssues}
            onIgnoreIssue={(id) =>
              commitState(
                (current) => ({
                  ...current,
                  issueRecords: current.issueRecords.map((item) =>
                    item.id === id ? { ...item, status: '已忽略' } : item,
                  ),
                }),
                { message: '异常已忽略', tone: 'info' },
              )
            }
            onResolveIssue={(id) => {
              const issue = persisted.issueRecords.find((item) => item.id === id);
              if (!issue) return;
              const taskId = createId('task');
              commitState(
                (current) => ({
                  ...current,
                  issueRecords: current.issueRecords.map((item) =>
                    item.id === id ? { ...item, status: '处理中' } : item,
                  ),
                  taskRecords: [
                    {
                      id: taskId,
                      kind: 'other',
                      title: `修复：${issue.asset}`,
                      type: 'REPAIR',
                      status: '等待确认',
                      statusTone: 'info',
                      libraryId: issue.libraryId,
                      source: '异常中心',
                      target: issue.action,
                      progress: 0,
                      speed: '—',
                      eta: '待人工确认',
                      fileCount: 1,
                      multiFile: false,
                      updatedAt: '刚刚',
                    },
                    ...current.taskRecords,
                  ],
                }),
                { message: '已创建异常处理任务', tone: 'success' },
              );
            }}
            setIssueTypeFilter={setIssueTypeFilter}
          />
        ) : null}

        {activeView === 'storage-nodes' ? (
          <StorageNodesPage
            libraries={persisted.libraries}
            onFeedback={setFeedback}
            onOpenIssueCenter={() => setActiveView('issues')}
            onOpenTaskCenter={() => setActiveView('task-center')}
          />
        ) : null}

        {activeView === 'settings' ? (
          <SettingsPage
            sections={settingsDraft[settingsTab]}
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
          />
        ) : null}
      </main>

      {fileDetail ? (
        <FileDetailSheet
          item={fileDetail}
          onClose={() => setFileDetail(null)}
          onDeleteAsset={(item) => deleteAssets([item.id])}
          onDeleteEndpoint={deleteFromEndpoint}
        />
      ) : null}

      {taskDetail ? (
        <TaskDetailSheet
          item={taskDetail}
          items={persisted.taskItemRecords.filter((item) => item.taskId === taskDetail.id)}
          onClose={() => setTaskDetail(null)}
        />
      ) : null}

      {managedLibrary ? <LibraryManagerSheet library={managedLibrary} onClose={() => setManagedLibrary(null)} /> : null}

      {notificationsOpen ? (
        <Sheet onClose={() => setNotificationsOpen(false)} title="通知中心">
          <div className="workspace-card compact-list inner-list">
            {persisted.notifications.length === 0 ? (
              <p className="muted-text">还没有通知。</p>
            ) : (
              persisted.notifications.map((item) => (
                <article className="notice-card" key={item.id}>
                  <div>
                    <strong>{item.title}</strong>
                    <p>{item.detail}</p>
                  </div>
                  <div className="notice-meta">
                    <span>{item.createdAt}</span>
                    {item.read === false ? (
                      <ActionButton
                        onClick={() =>
                          setPersisted((current) => ({
                            ...current,
                            notifications: current.notifications.map((notice) =>
                              notice.id === item.id ? { ...notice, read: true } : notice,
                            ),
                          }))
                        }
                      >
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

      {folderDraft !== null ? (
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
                commitState(
                  (current) => ({
                    ...current,
                    fileNodes: [
                      {
                        id: createId('folder'),
                        libraryId: activeLibraryId,
                        parentId: currentFolderId,
                        type: 'folder',
                        lifecycleState: 'ACTIVE',
                        name: folderDraft.trim(),
                        fileKind: '文件夹',
                        displayType: '文件夹',
                        modifiedAt: '刚刚',
                        size: '0 项',
                        path: `${breadcrumbs.map((item) => item.label).join(' / ')} / ${folderDraft.trim()}`,
                        endpoints: [{ name: '本地NVMe', state: '已创建', tone: 'success' }],
                        metadata: [{ label: '来源', value: '客户端新建' }],
                      },
                      ...current.fileNodes,
                    ],
                  }),
                  { message: '目录已创建', tone: 'success' },
                );
                setFolderDraft(null);
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

const navIcons: Record<Exclude<MainView, 'import-center'>, React.ReactNode> = {
  'file-center': <FolderOpen size={16} />,
  'task-center': <CircleEllipsis size={16} />,
  issues: <AlertTriangle size={16} />,
  'storage-nodes': <HardDrive size={16} />,
  settings: <Settings2 size={16} />,
};
