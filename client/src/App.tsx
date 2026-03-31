import { useEffect, useMemo, useState } from 'react';
import {
  Bell,
  FolderInput,
  HardDrive,
  Moon,
  Settings2,
  SunMedium,
  AlertTriangle,
  ArchiveRestore,
  FolderOpen,
  CircleEllipsis,
} from 'lucide-react';
import type {
  FileNode,
  FileTypeFilter,
  Library,
  MainView,
  SettingsTab,
  StorageNode,
  StorageTypeFilter,
  TaskRecord,
  TaskTab,
  ThemeMode,
} from './data';
import {
  fileNodes,
  importBatches,
  importSourceFiles,
  issueRecords,
  libraries,
  navigationItems,
  recycleRecords,
  storageNodes,
  taskItemRecords,
  taskRecords,
} from './data';
import { IconButton, LibraryManagerSheet } from './components/Shared';
import { ContextMenu } from './components/ContextMenu';
import { FileCenterPage } from './pages/FileCenterPage';
import { ImportCenterPage } from './pages/ImportCenterPage';
import { IssuesPage } from './pages/IssuesPage';
import { RecyclePage } from './pages/RecyclePage';
import { SettingsPage } from './pages/SettingsPage';
import { StorageNodeSheet, StorageNodesPage } from './pages/StorageNodesPage';
import { FileDetailSheet } from './pages/FileDetailSheet';
import { TaskCenterPage, TaskDetailSheet } from './pages/TaskCenterPage';

export type PageSize = 10 | 20 | 50 | 100;
export type ContextMenuTarget =
  | { type: 'file'; item: FileNode; x: number; y: number }
  | { type: 'task'; item: TaskRecord; x: number; y: number }
  | { type: 'storage'; item: StorageNode; x: number; y: number }
  | null;

export default function App() {
  const [theme, setTheme] = useState<ThemeMode>('dark');
  const [activeView, setActiveView] = useState<MainView>('file-center');
  const [activeLibraryId, setActiveLibraryId] = useState(libraries[0].id);
  const [libraryMenuOpen, setLibraryMenuOpen] = useState(false);
  const [managedLibrary, setManagedLibrary] = useState<Library | null>(null);
  const [storageDetail, setStorageDetail] = useState<StorageNode | null>(null);
  const [taskDetail, setTaskDetail] = useState<TaskRecord | null>(null);
  const [fileDetail, setFileDetail] = useState<FileNode | null>(null);
  const [contextMenu, setContextMenu] = useState<ContextMenuTarget>(null);
  const [taskTab, setTaskTab] = useState<TaskTab>('transfer');
  const [settingsTab, setSettingsTab] = useState<SettingsTab>('general');
  const [pageSize, setPageSize] = useState<PageSize>(20);
  const [currentPage, setCurrentPage] = useState(1);
  const [searchText, setSearchText] = useState('');
  const [currentFolderId, setCurrentFolderId] = useState<string | null>(null);
  const [fileTypeFilter, setFileTypeFilter] = useState<FileTypeFilter>('全部');
  const [taskStatusFilter, setTaskStatusFilter] = useState('全部');
  const [issueTypeFilter, setIssueTypeFilter] = useState('全部');
  const [recycleTypeFilter, setRecycleTypeFilter] = useState<FileTypeFilter>('全部');
  const [recycleEndpointFilter, setRecycleEndpointFilter] = useState('全部');
  const [storageTypeFilter, setStorageTypeFilter] = useState<StorageTypeFilter>('全部');
  const [selectedImportBatchId, setSelectedImportBatchId] = useState(importBatches[0].id);
  const [selectedImportTargets, setSelectedImportTargets] = useState<Record<string, string[]>>(() =>
    Object.fromEntries(importSourceFiles.map((file) => [file.id, file.selectedTargets])),
  );

  const currentLibrary = useMemo(
    () => libraries.find((library) => library.id === activeLibraryId) ?? libraries[0],
    [activeLibraryId],
  );

  const breadcrumbs = useMemo(() => {
    const chain = [currentLibrary.name];
    let cursor: FileNode | undefined = currentFolderId
      ? fileNodes.find((node) => node.id === currentFolderId)
      : undefined;
    while (cursor) {
      chain.push(cursor.name);
      const parentId = cursor.parentId;
      cursor = parentId ? fileNodes.find((node) => node.id === parentId) : undefined;
    }
    return chain;
  }, [currentFolderId, currentLibrary.name]);

  const currentEntries = useMemo(() => {
    const keyword = searchText.trim().toLowerCase();
    return fileNodes
      .filter((node) => node.libraryId === activeLibraryId && node.parentId === currentFolderId)
      .filter((node) => (fileTypeFilter === '全部' ? true : node.fileKind === fileTypeFilter))
      .filter((node) =>
        keyword ? `${node.name} ${node.path} ${node.displayType}`.toLowerCase().includes(keyword) : true,
      );
  }, [activeLibraryId, currentFolderId, fileTypeFilter, searchText]);

  const paginatedEntries = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return currentEntries.slice(start, start + pageSize);
  }, [currentEntries, currentPage, pageSize]);

  const pageCount = Math.max(1, Math.ceil(currentEntries.length / pageSize));

  const selectedBatch = useMemo(
    () => importBatches.find((batch) => batch.id === selectedImportBatchId) ?? importBatches[0],
    [selectedImportBatchId],
  );

  const visibleImportFiles = useMemo(
    () => importSourceFiles.filter((file) => file.batchId === selectedBatch.id),
    [selectedBatch.id],
  );

  const visibleTasks = useMemo(
    () =>
      taskRecords.filter(
        (task) =>
          task.libraryId === activeLibraryId &&
          task.kind === taskTab &&
          (taskStatusFilter === '全部' || task.status === taskStatusFilter),
      ),
    [activeLibraryId, taskStatusFilter, taskTab],
  );

  const visibleIssues = useMemo(
    () =>
      issueRecords.filter(
        (item) =>
          item.libraryId === activeLibraryId &&
          (issueTypeFilter === '全部' || item.type === issueTypeFilter),
      ),
    [activeLibraryId, issueTypeFilter],
  );

  const visibleRecycle = useMemo(
    () =>
      recycleRecords.filter(
        (item) =>
          (recycleTypeFilter === '全部' || item.fileType === recycleTypeFilter) &&
          (recycleEndpointFilter === '全部' || item.endpoint === recycleEndpointFilter),
      ),
    [recycleEndpointFilter, recycleTypeFilter],
  );

  const visibleStorage = useMemo(
    () => storageNodes.filter((node) => (storageTypeFilter === '全部' ? true : node.nodeType === storageTypeFilter)),
    [storageTypeFilter],
  );

  useEffect(() => setCurrentPage(1), [currentFolderId, fileTypeFilter, pageSize, searchText]);
  useEffect(() => {
    const handleClose = () => setContextMenu(null);
    window.addEventListener('click', handleClose);
    return () => window.removeEventListener('click', handleClose);
  }, []);

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
              <path className="mare-wave-foam" d="M23 19c3 1 5 3 6 7 1-3 3-5 6-6-1 2-1 4 0 5 1 1 2 2 4 2-3 1-5 3-5 6-1-3-3-5-6-5-2 0-4 1-6 3 1-2 1-4 0-6-1-1-2-2-4-2 2 0 4-1 5-4z" />
              <path className="mare-wave-foam" d="M18 43c4-3 7-4 10-4 4 0 7 2 9 6 1 3 2 4 4 4 2 0 4-2 5-6 2-6 5-9 10-9 2 0 4 1 6 2-2 0-3 1-4 2-2 1-4 3-5 7-2 6-5 9-10 9-4 0-7-2-9-6-2-4-4-5-8-5-3 0-5 1-8 3z" />
              <path className="mare-wave-front" d="M10 50c3-4 6-6 10-6 4 0 7 2 10 7 2-9 6-15 13-15 5 0 8 3 10 7 2 4 4 6 8 7v7H10z" />
              <path className="mare-inner-sea" d="M38 32c4 1 6 2 8 5 1 2 2 4 5 4 2 0 4-1 6-3-1 3-4 5-8 5-4 0-7-2-9-5-1-3-2-5-5-6 1 0 2 0 3 0z" />
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
              {libraries.map((library) => (
                <div className={`library-option${library.id === currentLibrary.id ? ' active' : ''}`} key={library.id}>
                  <button
                    className="library-option-main"
                    type="button"
                    onClick={() => {
                      setActiveLibraryId(library.id);
                      setCurrentFolderId(null);
                      setLibraryMenuOpen(false);
                    }}
                  >
                    <div>
                      <strong>{library.name}</strong>
                      <span>
                        {library.itemCount} 项 · {library.storagePolicy}
                      </span>
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
              className={`nav-item${item.id === activeView ? ' active' : ''}`}
              type="button"
              onClick={() => setActiveView(item.id)}
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
          <div className="page-header-title">
            <span>{currentLibrary.name}</span>
            <strong>{viewTitles[activeView]}</strong>
          </div>
          <div className="page-header-actions">
            <IconButton ariaLabel="导入中心" onClick={() => setActiveView('import-center')}>
              <FolderInput size={16} />
            </IconButton>
            <IconButton ariaLabel="通知">
              <Bell size={16} />
            </IconButton>
            <IconButton
              ariaLabel={theme === 'dark' ? '切换到浅色模式' : '切换到深色模式'}
              onClick={() => setTheme((value) => (value === 'dark' ? 'light' : 'dark'))}
            >
              {theme === 'dark' ? <SunMedium size={16} /> : <Moon size={16} />}
            </IconButton>
          </div>
        </header>

        {activeView === 'file-center' && (
          <FileCenterPage
            breadcrumbs={breadcrumbs}
            currentEntries={paginatedEntries}
            currentPage={currentPage}
            currentPathChildren={currentEntries.length}
            fileTypeFilter={fileTypeFilter}
            pageCount={pageCount}
            pageSize={pageSize}
            searchText={searchText}
            setContextMenu={setContextMenu}
            setCurrentFolderId={setCurrentFolderId}
            setCurrentPage={setCurrentPage}
            setFileDetail={setFileDetail}
            setFileTypeFilter={setFileTypeFilter}
            setPageSize={setPageSize}
            setSearchText={setSearchText}
          />
        )}

        {activeView === 'import-center' && (
          <ImportCenterPage
            batches={importBatches}
            selectedBatch={selectedBatch}
            selectedImportTargets={selectedImportTargets}
            setSelectedBatchId={setSelectedImportBatchId}
            setSelectedImportTargets={setSelectedImportTargets}
            visibleFiles={visibleImportFiles}
          />
        )}

        {activeView === 'task-center' && (
          <TaskCenterPage
            activeTab={taskTab}
            setActiveTab={setTaskTab}
            setContextMenu={setContextMenu}
            setTaskDetail={setTaskDetail}
            setTaskStatusFilter={setTaskStatusFilter}
            statusFilter={taskStatusFilter}
            tasks={visibleTasks}
          />
        )}

        {activeView === 'issues' && (
          <IssuesPage issueTypeFilter={issueTypeFilter} items={visibleIssues} setIssueTypeFilter={setIssueTypeFilter} />
        )}

        {activeView === 'recycle' && (
          <RecyclePage
            endpointFilter={recycleEndpointFilter}
            items={visibleRecycle}
            setEndpointFilter={setRecycleEndpointFilter}
            setTypeFilter={setRecycleTypeFilter}
            typeFilter={recycleTypeFilter}
          />
        )}

        {activeView === 'storage-nodes' && (
          <StorageNodesPage
            items={visibleStorage}
            setContextMenu={setContextMenu}
            setStorageDetail={setStorageDetail}
            setStorageTypeFilter={setStorageTypeFilter}
            typeFilter={storageTypeFilter}
          />
        )}

        {activeView === 'settings' && <SettingsPage settingsTab={settingsTab} setSettingsTab={setSettingsTab} />}
      </main>

      {contextMenu && (
        <ContextMenu
          menu={contextMenu}
          onClose={() => setContextMenu(null)}
          onOpenFileDetail={setFileDetail}
          onOpenFolder={(item) => setCurrentFolderId(item.id)}
          onOpenStorageDetail={setStorageDetail}
          onOpenTaskDetail={setTaskDetail}
        />
      )}

      {fileDetail && <FileDetailSheet item={fileDetail} onClose={() => setFileDetail(null)} />}
      {taskDetail && (
        <TaskDetailSheet item={taskDetail} items={taskItemRecords.filter((taskItem) => taskItem.taskId === taskDetail.id)} onClose={() => setTaskDetail(null)} />
      )}
      {storageDetail && <StorageNodeSheet item={storageDetail} onClose={() => setStorageDetail(null)} />}
      {managedLibrary && <LibraryManagerSheet library={managedLibrary} onClose={() => setManagedLibrary(null)} />}
    </div>
  );
}

const navIcons: Record<Exclude<MainView, 'import-center'>, React.ReactNode> = {
  'file-center': <FolderOpen size={16} />,
  'task-center': <CircleEllipsis size={16} />,
  issues: <AlertTriangle size={16} />,
  recycle: <ArchiveRestore size={16} />,
  'storage-nodes': <HardDrive size={16} />,
  settings: <Settings2 size={16} />,
};

const viewTitles: Record<MainView, string> = {
  'file-center': '文件中心',
  'task-center': '任务中心',
  issues: '异常中心',
  recycle: '回收站',
  'storage-nodes': '存储节点',
  settings: '设置',
  'import-center': '导入中心',
};
