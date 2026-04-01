import { startTransition, useDeferredValue, useEffect, useMemo, useState } from 'react';
import {
  AlertTriangle,
  Bell,
  CircleEllipsis,
  FolderInput,
  FolderOpen,
  HardDrive,
  Settings2,
  Star,
} from 'lucide-react';
import type {
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
  cloneSettingsRecord,
  createId,
  getDefaultPageSize,
  loadPersistedState,
  resolveLibraryForImport,
  resolveThemeMode,
  STORAGE_KEY,
  type NotificationItem,
  type PersistedState,
} from './lib/clientState';
import {
  canSyncFileCenterEndpoint,
  canDeleteFileCenterEndpoint,
  fileCenterApi,
  type FileCenterColorLabel,
  type FileCenterDirectoryResult,
  type FileCenterEntry,
  type FileCenterSortDirection,
  type FileCenterSortValue,
  type FileCenterStatusFilter,
  type FileCenterTagSuggestion,
} from './lib/fileCenterApi';
import { ActionButton, IconButton, LibraryManagerSheet, Sheet } from './components/Shared';
import { FileCenterPage } from './pages/FileCenterPage';
import { FileDetailSheet } from './pages/FileDetailSheet';
import { ImportCenterPage } from './pages/ImportCenterPage';
import { IssuesPage } from './pages/IssuesPage';
import { SettingsPage } from './pages/SettingsPage';
import { StorageNodesPage } from './pages/StorageNodesPage';
import { TagManagementPage } from './pages/TagManagementPage';
import { TaskCenterPage, TaskDetailSheet } from './pages/TaskCenterPage';

export type PageSize = 10 | 20 | 50 | 100;
export type ContextMenuTarget =
  | { type: 'file'; item: FileCenterEntry; x: number; y: number }
  | { type: 'task'; item: TaskRecord; x: number; y: number }
  | { type: 'storage'; item: StorageNode; x: number; y: number }
  | null;

type FeedbackState = { message: string; tone: Severity } | null;
type FileConfirmAction =
  | { kind: 'sync'; items: FileCenterEntry[]; endpointName: string; totalSelected: number }
  | { kind: 'delete-asset'; items: FileCenterEntry[] }
  | {
      kind: 'delete-endpoint';
      items: FileCenterEntry[];
      endpointName: string;
      totalSelected: number;
      willDeleteAssetCount: number;
    };
type TagEditorState = {
  item: FileCenterEntry;
} | null;
type BatchAnnotationState = {
  items: FileCenterEntry[];
} | null;
type BatchEndpointAction = {
  endpointName: string;
  enabled: boolean;
};

export default function App() {
  const [persisted, setPersisted] = useState<PersistedState>(loadPersistedState);
  const [activeView, setActiveView] = useState<MainView>('file-center');
  const [activeLibraryId, setActiveLibraryId] = useState(persisted.libraries[0]?.id ?? 'photo');
  const [taskTab, setTaskTab] = useState<TaskTab>('transfer');
  const [settingsTab, setSettingsTab] = useState<SettingsTab>('general');
  const [fileTypeFilter, setFileTypeFilter] = useState<FileTypeFilter>('全部');
  const [fileStatusFilter, setFileStatusFilter] = useState<FileCenterStatusFilter>('全部');
  const [partialSyncEndpointNames, setPartialSyncEndpointNames] = useState<string[]>([]);
  const [taskStatusFilter, setTaskStatusFilter] = useState('全部');
  const [issueTypeFilter, setIssueTypeFilter] = useState('全部');
  const [searchText, setSearchText] = useState('');
  const [fileSort, setFileSort] = useState<FileCenterSortValue>('修改时间');
  const [fileSortDirection, setFileSortDirection] = useState<FileCenterSortDirection>('desc');
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
  const [fileDetail, setFileDetail] = useState<FileCenterEntry | null>(null);
  const [fileCenterState, setFileCenterState] = useState<FileCenterDirectoryResult>({
    breadcrumbs: [],
    items: [],
    total: 0,
    currentPathChildren: 0,
  });
  const [fileCenterLoading, setFileCenterLoading] = useState(true);
  const [fileCenterVersion, setFileCenterVersion] = useState(0);
  const [availableTags, setAvailableTags] = useState<FileCenterTagSuggestion[]>([]);
  const [pendingAction, setPendingAction] = useState<FileConfirmAction | null>(null);
  const [tagEditorState, setTagEditorState] = useState<TagEditorState>(null);
  const [batchAnnotationState, setBatchAnnotationState] = useState<BatchAnnotationState>(null);
  const [selectedFileEntries, setSelectedFileEntries] = useState<FileCenterEntry[]>([]);
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
  }, [currentFolderId, deferredSearchText, fileSort, fileStatusFilter, fileTypeFilter, pageSize, partialSyncEndpointNames]);

  useEffect(() => {
    setSelectedFileIds([]);
  }, [activeLibraryId, currentFolderId]);

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
    void fileCenterApi.loadTagSuggestions('', activeLibraryId).then(setAvailableTags).catch(() => {
      setAvailableTags([]);
    });
  }, [activeLibraryId, fileCenterVersion]);

  const theme = useMemo(() => resolveThemeMode(persisted.settings), [persisted.settings]);
  const currentLibrary = useMemo(
    () => persisted.libraries.find((item) => item.id === activeLibraryId) ?? persisted.libraries[0],
    [activeLibraryId, persisted.libraries],
  );
  const statusFilterEndpointNames = useMemo(
    () => fileCenterApi.listLibraryEndpointNames(activeLibraryId),
    [activeLibraryId],
  );
  const breadcrumbs = fileCenterState.breadcrumbs.length > 0
    ? fileCenterState.breadcrumbs
    : [{ id: null as string | null, label: currentLibrary.name }];
  const pageCount = Math.max(1, Math.ceil(fileCenterState.total / pageSize));

  useEffect(() => {
    if (currentPage > pageCount) {
      setCurrentPage(pageCount);
    }
  }, [currentPage, pageCount]);

  useEffect(() => {
    let disposed = false;
    setFileCenterLoading(true);

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
        });
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

  const createDeleteTasks = (
    current: PersistedState,
    items: FileCenterEntry[],
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

  const createSyncTasks = (
    current: PersistedState,
    items: FileCenterEntry[],
    endpointName: string,
  ): PersistedState => ({
    ...current,
    taskRecords: [
      ...items.map((item): TaskRecord => ({
        id: createId('task'),
        kind: 'transfer',
        title: `同步资产：${item.name}`,
        type: 'SYNC',
        status: '等待确认',
        statusTone: 'warning',
        libraryId: item.libraryId,
        source: '统一资产',
        target: endpointName,
        progress: 0,
        speed: '—',
        eta: '等待执行器接管',
        fileCount: 1,
        multiFile: false,
        updatedAt: '刚刚',
      })),
      ...current.taskRecords,
    ],
  });

  const getManagedReplicaCount = (item: FileCenterEntry, excludingEndpointName?: string) =>
    item.endpoints.filter(
      (endpoint) =>
        endpoint.endpointType !== 'removable' &&
        endpoint.name !== excludingEndpointName &&
        canDeleteFileCenterEndpoint(endpoint),
    ).length;

  const batchEndpointActions = useMemo(() => {
    const sourceItems = selectedFileEntries.length > 0
      ? selectedFileEntries
      : fileCenterState.items.filter((item) => selectedFileIds.includes(item.id));
    const endpointNames = Array.from(
      new Set(sourceItems.flatMap((item) => item.endpoints.map((endpoint) => endpoint.name))),
    );

    const syncActions: BatchEndpointAction[] = endpointNames.map((endpointName) => ({
      endpointName,
      enabled: sourceItems.some((item) =>
        item.endpoints.some(
          (endpoint) => endpoint.name === endpointName && canSyncFileCenterEndpoint(endpoint),
        ),
      ),
    }));

    const deleteActions: BatchEndpointAction[] = endpointNames.map((endpointName) => ({
      endpointName,
      enabled: sourceItems.some((item) =>
        item.endpoints.some(
          (endpoint) => endpoint.name === endpointName && canDeleteFileCenterEndpoint(endpoint),
        ),
      ),
    }));

    return {
      syncActions,
      deleteActions,
    };
  }, [fileCenterState.items, selectedFileEntries, selectedFileIds]);

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

  const performDeleteAssets = async (items: FileCenterEntry[]) => {
    const result = await fileCenterApi.deleteAssets(items.map((item) => item.id));
    commitState((current) => createDeleteTasks(current, items, 'asset'), {
      message: result.message,
      tone: 'warning',
    });
    setSelectedFileIds([]);
    setFileDetail(null);
    setFileCenterVersion((current) => current + 1);
  };

  const requestDeleteEndpoint = (item: FileCenterEntry, endpointName: string) => {
    requestBatchDeleteEndpoint(endpointName, [item]);
  };

  const requestBatchDeleteEndpoint = (endpointName: string, sourceItems = selectedActionItems) => {
    const eligibleItems = sourceItems.filter((item) =>
      item.endpoints.some(
        (endpoint) => endpoint.name === endpointName && canDeleteFileCenterEndpoint(endpoint),
      ),
    );

    if (eligibleItems.length === 0) {
      setFeedback({ message: `当前所选资产在 ${endpointName} 上没有可删除的副本`, tone: 'info' });
      return;
    }

    setPendingAction({
      kind: 'delete-endpoint',
      endpointName,
      items: eligibleItems,
      totalSelected: sourceItems.length,
      willDeleteAssetCount: eligibleItems.filter((item) => getManagedReplicaCount(item, endpointName) === 0).length,
    });
  };

  const performDeleteFromEndpoint = async (items: FileCenterEntry[], endpointName: string) => {
    const shouldRefreshDetail = items.some((item) => item.id === fileDetail?.id);
    const results = await Promise.all(items.map((item) => fileCenterApi.deleteFromEndpoint(item.id, endpointName)));
    const deletedCount = results.filter((result) => result.deleted).length;
    const deletedIds = items.filter((_, index) => results[index]?.deleted).map((item) => item.id);
    const updatedItem = shouldRefreshDetail && fileDetail ? await fileCenterApi.loadEntryDetail(fileDetail.id) : null;
    commitState((current) => createDeleteTasks(current, items, 'endpoint', endpointName), {
      message:
        deletedCount === 0
          ? items.length === 1
            ? '已提交端点删除请求'
            : `已提交 ${items.length} 项资产的端点删除请求`
          : deletedCount === items.length
            ? items.length === 1
              ? '资产已因无剩余副本自动删除'
              : `${items.length} 项资产已因无剩余副本自动删除`
            : `已提交端点删除请求，其中 ${deletedCount} 项资产因无剩余副本自动删除`,
      tone: 'info',
    });
    setSelectedFileIds((current) => current.filter((id) => !deletedIds.includes(id)));
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
    const eligibleItems = sourceItems.filter((item) =>
      item.endpoints.some(
        (endpoint) => endpoint.name === endpointName && canSyncFileCenterEndpoint(endpoint),
      ),
    );

    if (eligibleItems.length === 0) {
      setFeedback({ message: `当前所选资产在 ${endpointName} 上没有可同步的副本`, tone: 'info' });
      return;
    }

    setPendingAction({ kind: 'sync', items: eligibleItems, endpointName, totalSelected: sourceItems.length });
  };

  const performSyncToEndpoint = async (items: FileCenterEntry[], endpointName: string) => {
    const shouldRefreshDetail = items.some((item) => item.id === fileDetail?.id);
    await Promise.all(items.map((item) => fileCenterApi.syncToEndpoint(item.id, endpointName)));
    const updatedItem = shouldRefreshDetail && fileDetail ? await fileCenterApi.loadEntryDetail(fileDetail.id) : null;
    commitState((current) => createSyncTasks(current, items, endpointName), {
      message: items.length === 1 ? `已创建同步任务到 ${endpointName}` : `已为 ${items.length} 项资产创建同步任务到 ${endpointName}`,
      tone: 'info',
    });
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

  const handleUploadSelection = async (mode: 'files' | 'folder', files: File[]) => {
    const result = await fileCenterApi.uploadSelection({
      libraryId: activeLibraryId,
      parentId: currentFolderId,
      mode,
      items: files.map((file) => ({
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
  };

  const validationMessage =
    visibleImportFiles.find((file) => (selectedImportTargets[file.id] ?? []).length === 0) != null
      ? '仍有文件未选择目标端，提交前需要补齐。'
      : null;

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
            breadcrumbs={breadcrumbs}
            canGoBack={historyIndex > 0}
            canGoForward={historyIndex < folderHistory.length - 1}
            currentEntries={fileCenterState.items}
            currentPage={currentPage}
            currentPathChildren={fileCenterState.currentPathChildren}
            fileTypeFilter={fileTypeFilter}
            loading={fileCenterLoading}
            pageCount={pageCount}
            pageSize={pageSize}
            partialSyncEndpointNames={partialSyncEndpointNames}
            searchText={searchText}
            selectedIds={selectedFileIds}
            sortValue={fileSort}
            sortDirection={fileSortDirection}
            statusFilterEndpointNames={statusFilterEndpointNames}
            statusFilter={fileStatusFilter}
            theme={theme}
            total={fileCenterState.total}
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
            onOpenTagEditor={(item) => setTagEditorState({ item })}
            onDeleteSelected={() => void requestDeleteAssets(selectedFileIds)}
            onRefreshIndex={() =>
              void fileCenterApi
                .refreshIndex()
                .then((result) => {
                  setFeedback({ message: result.message, tone: 'info' });
                  setFileCenterVersion((current) => current + 1);
                })
                .catch(() => {
                  setFeedback({ message: '索引刷新失败，请稍后重试', tone: 'critical' });
                })
            }
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
            customContent={
              <TagManagementPage
                libraries={persisted.libraries}
                onFeedback={setFeedback}
              />
            }
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
          onSaveAnnotations={saveAnnotations}
        />
      ) : null}

      {tagEditorState ? (
        <TagEditorDialog
          availableTags={availableTags}
          item={tagEditorState.item}
          onClose={() => setTagEditorState(null)}
          onSave={(tags) => void saveTags(tagEditorState.item, tags)}
        />
      ) : null}

      {batchAnnotationState ? (
        <BatchAnnotationDialog
          count={batchAnnotationState.items.length}
          onClose={() => setBatchAnnotationState(null)}
          onSave={(input) => void saveBatchAnnotations(input)}
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

      {pendingAction ? (
        <FileActionDialog
          action={pendingAction}
          onCancel={() => setPendingAction(null)}
          onConfirm={() => void confirmPendingAction()}
        />
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
}: {
  action: FileConfirmAction;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const isDeleteAsset = action.kind === 'delete-asset';
  const skippedCount = action.kind === 'delete-asset' ? 0 : action.totalSelected - action.items.length;

  const title =
    action.kind === 'sync'
      ? '确认同步'
      : action.kind === 'delete-endpoint'
        ? '确认删除副本'
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
    if (action.willDeleteAssetCount > 0) {
      notes.push({ text: `其中 ${action.willDeleteAssetCount} 项会移除最后一个受管副本，资产将被删除。`, critical: true });
    } else {
      notes.push({ text: '删除后可在任务中心查看执行状态。' });
    }
  }

  if (isDeleteAsset) {
    notes.push({ text: '删除后可在任务中心查看执行状态。', critical: true });
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
          <ActionButton tone={action.kind === 'sync' ? 'primary' : 'danger'} onClick={onConfirm}>
            {action.kind === 'sync' ? '确认同步' : '确认删除'}
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

function resolveBatchColorClass(colorLabel: FileCenterColorLabel) {
  if (colorLabel === '红标') return 'red';
  if (colorLabel === '黄标') return 'yellow';
  if (colorLabel === '绿标') return 'green';
  if (colorLabel === '蓝标') return 'blue';
  if (colorLabel === '紫标') return 'purple';
  return 'none';
}

const navIcons: Record<Exclude<MainView, 'import-center'>, React.ReactNode> = {
  'file-center': <FolderOpen size={16} />,
  'task-center': <CircleEllipsis size={16} />,
  issues: <AlertTriangle size={16} />,
  'storage-nodes': <HardDrive size={16} />,
  settings: <Settings2 size={16} />,
};
