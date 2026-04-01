import { useEffect, useMemo, useState, type MouseEvent as ReactMouseEvent } from 'react';
import { createPortal } from 'react-dom';
import {
  ChevronLeft,
  ChevronRight,
  ChevronRight as SubmenuArrow,
  Cloud,
  Ellipsis,
  Flag,
  File,
  FileAudio2,
  FileImage,
  FileText,
  Folder,
  FolderPlus,
  Info,
  LoaderCircle,
  RefreshCw,
  Server,
  Star,
  Trash2,
} from 'lucide-react';
import type { FileTypeFilter, Severity, ThemeMode } from '../data';
import type {
  FileCenterColorLabel,
  FileCenterEndpoint,
  FileCenterEntry,
  FileCenterSortValue,
  FileCenterStatusFilter,
} from '../lib/fileCenterApi';
import type { PageSize } from '../App';
import { ActionButton, EmptyState, IconButton, SelectPill, TonePill } from '../components/Shared';

const FILE_TYPE_OPTIONS: FileTypeFilter[] = ['全部', '文件夹', '图片', '视频', '音频', '文档'];
const STATUS_OPTIONS: FileCenterStatusFilter[] = ['全部', '待同步', '有异常', '多端齐全', '待清理'];
const SORT_OPTIONS: FileCenterSortValue[] = ['修改时间', '名称', '大小'];
const PAGE_SIZE_OPTIONS: PageSize[] = [10, 20, 50, 100];

type MenuState = {
  id: string;
  submenu: 'sync' | 'delete' | null;
  top: number;
  right: number;
} | null;

export function FileCenterPage(props: {
  breadcrumbs: Array<{ id: string | null; label: string }>;
  canGoBack: boolean;
  canGoForward: boolean;
  currentEntries: FileCenterEntry[];
  currentPage: number;
  currentPathChildren: number;
  fileTypeFilter: FileTypeFilter;
  loading: boolean;
  pageCount: number;
  pageSize: PageSize;
  searchText: string;
  selectedIds: string[];
  sortValue: FileCenterSortValue;
  statusFilter: FileCenterStatusFilter;
  theme: ThemeMode;
  total: number;
  onChangeSort: (value: FileCenterSortValue) => void;
  onClearSelection: () => void;
  onCreateFolder: () => void;
  onDeleteAssetDirect: (item: FileCenterEntry) => void;
  onDeleteSelected: () => void;
  onGoBack: () => void;
  onGoForward: () => void;
  onNavigateBreadcrumb: (index: number) => void;
  onOpenItem: (item: FileCenterEntry) => void;
  onOpenItemDetail: (item: FileCenterEntry) => void;
  onOpenTagEditor: (item: FileCenterEntry) => void;
  onRefreshIndex: () => void;
  onRequestDeleteEndpoint: (item: FileCenterEntry, endpointName: string) => void;
  onRequestSyncEndpoint: (item: FileCenterEntry, endpointName: string) => void;
  onSetCurrentPage: (value: number) => void;
  onSetFileTypeFilter: (value: FileTypeFilter) => void;
  onSetPageSize: (value: PageSize) => void;
  onSetSearchText: (value: string) => void;
  onSetStatusFilter: (value: FileCenterStatusFilter) => void;
  onToggleSelect: (id: string) => void;
  onToggleSelectVisible: () => void;
}) {
  const {
    breadcrumbs,
    canGoBack,
    canGoForward,
    currentEntries,
    currentPage,
    currentPathChildren,
    fileTypeFilter,
    loading,
    pageCount,
    pageSize,
    searchText,
    selectedIds,
    sortValue,
    statusFilter,
    theme,
    total,
    onChangeSort,
    onClearSelection,
    onCreateFolder,
    onDeleteAssetDirect,
    onDeleteSelected,
    onGoBack,
    onGoForward,
    onNavigateBreadcrumb,
    onOpenItem,
    onOpenItemDetail,
    onOpenTagEditor,
    onRefreshIndex,
    onRequestDeleteEndpoint,
    onRequestSyncEndpoint,
    onSetCurrentPage,
    onSetFileTypeFilter,
    onSetPageSize,
    onSetSearchText,
    onSetStatusFilter,
    onToggleSelect,
    onToggleSelectVisible,
  } = props;

  const [menuState, setMenuState] = useState<MenuState>(null);

  useEffect(() => {
    if (!menuState) {
      return;
    }

    const handlePointerDown = (event: MouseEvent) => {
      const target = event.target as HTMLElement | null;
      if (!target) return;
      if (target.closest('.file-row-menu-anchor')) return;
      if (target.closest('.context-menu')) return;
      setMenuState(null);
    };

    document.addEventListener('mousedown', handlePointerDown);
    return () => document.removeEventListener('mousedown', handlePointerDown);
  }, [menuState]);

  const allVisibleSelected =
    currentEntries.length > 0 && currentEntries.every((item) => selectedIds.includes(item.id));

  const resultSummary = useMemo(() => {
    if (loading) {
      return '正在读取目录索引…';
    }
    return `当前目录 ${currentPathChildren} 项，匹配 ${total} 项`;
  }, [currentPathChildren, loading, total]);

  return (
    <section className="page-stack file-center-page">
      <div className="toolbar-card explorer-toolbar file-center-toolbar">
        <div className="toolbar-group">
          <IconButton active={canGoBack} ariaLabel="后退" tooltip="后退" onClick={onGoBack}>
            <ChevronLeft size={16} />
          </IconButton>
          <IconButton active={canGoForward} ariaLabel="前进" tooltip="前进" onClick={onGoForward}>
            <ChevronRight size={16} />
          </IconButton>
        </div>

        <div className="address-bar">
          {breadcrumbs.map((item, index) => (
            <button key={`${item.label}-${index}`} type="button" onClick={() => onNavigateBreadcrumb(index)}>
              {item.label}
            </button>
          ))}
        </div>

        <label className="search-field" htmlFor="file-search">
          <input
            id="file-search"
            type="search"
            placeholder="搜索名称、路径、来源端"
            value={searchText}
            onChange={(event) => onSetSearchText(event.target.value)}
          />
        </label>
      </div>

      <div className="toolbar-card action-toolbar storage-topbar">
        <div className="toolbar-group wrap storage-toolbar-main">
          <SelectPill
            ariaLabel="文件类型筛选"
            options={FILE_TYPE_OPTIONS}
            value={fileTypeFilter}
            onChange={(value) => onSetFileTypeFilter(value as FileTypeFilter)}
          />
          <SelectPill
            ariaLabel="状态筛选"
            options={STATUS_OPTIONS}
            value={statusFilter}
            onChange={(value) => onSetStatusFilter(value as FileCenterStatusFilter)}
          />
          <SelectPill
            ariaLabel="排序方式"
            options={SORT_OPTIONS}
            value={sortValue}
            onChange={(value) => onChangeSort(value as FileCenterSortValue)}
          />
        </div>
        <div className="toolbar-group wrap">
          <ActionButton onClick={onCreateFolder}>
            <FolderPlus size={14} />
            新建目录
          </ActionButton>
          <ActionButton onClick={onRefreshIndex}>
            <RefreshCw size={14} />
            刷新索引
          </ActionButton>
        </div>
      </div>

      {selectedIds.length > 0 ? (
        <div className="toolbar-card selection-toolbar">
          <span className="selection-caption">已选择 {selectedIds.length} 项</span>
          <div className="toolbar-group wrap">
            <ActionButton ariaLabel="删除资产" tone="danger" onClick={onDeleteSelected}>
              <Trash2 size={14} />
              删除资产
            </ActionButton>
            <ActionButton ariaLabel="清空选择" onClick={onClearSelection}>
              清空选择
            </ActionButton>
          </div>
        </div>
      ) : null}

      <div className="workspace-card storage-table-card file-center-table-card">
        {loading ? (
          <div className="empty-state">
            <LoaderCircle className="spin" size={18} />
            <strong>正在加载文件中心</strong>
            <p>正在从本地索引读取目录和资产状态。</p>
          </div>
        ) : currentEntries.length === 0 ? (
          <EmptyState
            title="当前目录暂无匹配结果"
            description="可以调整筛选条件、清空搜索词，或者直接新建一个目录。"
            action={<ActionButton onClick={onCreateFolder}>新建目录</ActionButton>}
          />
        ) : (
          <>
            <div className="storage-table-wrap">
              <table className="file-table storage-table file-center-table">
                <thead>
                  <tr>
                    <th className="checkbox-cell">
                      <input
                        aria-label="选择当前页全部文件"
                        checked={allVisibleSelected}
                        type="checkbox"
                        onChange={onToggleSelectVisible}
                      />
                    </th>
                    <th>名称</th>
                    <th>类型</th>
                    <th>大小</th>
                    <th>修改时间</th>
                    <th>存储状态</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {currentEntries.map((item) => (
                    <tr key={item.id} aria-selected={selectedIds.includes(item.id)} onDoubleClick={() => onOpenItem(item)}>
                      <td className="checkbox-cell">
                        <input
                          aria-label={`选择 ${item.name}`}
                          checked={selectedIds.includes(item.id)}
                          type="checkbox"
                          onChange={() => onToggleSelect(item.id)}
                        />
                      </td>
                      <td>
                        <div className="file-center-name-cell">
                          <div className={`file-center-icon ${item.type}`}>
                            <FileKindIcon item={item} />
                          </div>
                          <div className="storage-node-cell">
                            <div className="storage-node-title">
                              <strong>{item.name}</strong>
                            </div>
                            {item.tags.length > 0 ? (
                              <div className="endpoint-row file-center-tag-row">
                                {item.tags.slice(0, 3).map((tag) => (
                                  <TonePill key={`${item.id}-${tag}`} tone="success">
                                    {tag}
                                  </TonePill>
                                ))}
                                {item.tags.length > 3 ? (
                                  <span className="selection-caption">+{item.tags.length - 3} 标签</span>
                                ) : null}
                              </div>
                            ) : null}
                            {item.rating > 0 || item.colorLabel !== '无' ? (
                              <div className="file-center-annotation-row">
                                {item.rating > 0 ? (
                                  <span className="file-rating" aria-label={`${item.rating} 星`}>
                                    {Array.from({ length: item.rating }, (_, index) => (
                                      <Star key={`${item.id}-star-${index}`} size={12} fill="currentColor" />
                                    ))}
                                  </span>
                                ) : null}
                                {item.colorLabel !== '无' ? <ColorFlagBadge colorLabel={item.colorLabel} /> : null}
                              </div>
                            ) : null}
                          </div>
                        </div>
                      </td>
                      <td>{item.displayType}</td>
                      <td>{item.size}</td>
                      <td>{item.modifiedAt}</td>
                      <td>
                        <div className="file-endpoint-statuses">
                          {item.endpoints.map((endpoint) => {
                            const syncAvailable = canSyncEndpoint(endpoint);
                            const toneClass = resolveEndpointStateClass(endpoint.state);
                            return (
                              <button
                                key={`${item.id}-${endpoint.name}`}
                                aria-label={`${endpoint.name} ${endpoint.state}`}
                                className={`endpoint-status-button ${toneClass}${syncAvailable ? '' : ' disabled'}`}
                                type="button"
                                onClick={() => syncAvailable && onRequestSyncEndpoint(item, endpoint.name)}
                              >
                                <EndpointGlyph endpoint={endpoint} />
                                <span>{endpoint.name}</span>
                                <strong>{endpoint.state}</strong>
                              </button>
                            );
                          })}
                        </div>
                      </td>
                      <td>
                        <div className="row-actions storage-row-actions">
                          <IconButton
                            ariaLabel={`${item.type === 'folder' ? '进入' : '打开详情'} ${item.name}`}
                            tooltip={item.type === 'folder' ? '进入' : '打开详情'}
                            onClick={() => onOpenItem(item)}
                          >
                            <Info size={15} />
                          </IconButton>
                          <div className="file-row-menu-anchor storage-menu-anchor">
                            <IconButton
                              ariaLabel={`更多操作 ${item.name}`}
                              tooltip="更多操作"
                              onClick={(event: ReactMouseEvent<HTMLButtonElement>) => {
                                const rect = event.currentTarget.getBoundingClientRect();
                                setMenuState((current) =>
                                  current?.id === item.id
                                    ? null
                                    : {
                                        id: item.id,
                                        submenu: null,
                                        top: rect.bottom + 8,
                                        right: Math.max(12, window.innerWidth - rect.right),
                                      },
                                );
                              }}
                            >
                              <Ellipsis size={15} />
                            </IconButton>
                            {menuState?.id === item.id
                              ? createPortal(
                                  <div
                                    className={`context-menu storage-menu-inline file-center-more-menu file-center-floating-menu theme-${theme}-popup`}
                                    style={{ position: 'fixed', top: menuState.top, right: menuState.right }}
                                  >
                                <button type="button" onClick={() => onOpenItemDetail(item)}>
                                  查看详情
                                </button>
                                <button
                                  type="button"
                                  onClick={() => {
                                    setMenuState(null);
                                    onOpenTagEditor(item);
                                  }}
                                >
                                  标签
                                </button>
                                <button
                                  className="has-submenu"
                                  type="button"
                                  onMouseEnter={() =>
                                    setMenuState((current) =>
                                      current
                                        ? { ...current, id: item.id, submenu: 'sync' }
                                        : current,
                                    )
                                  }
                                >
                                  同步
                                  <SubmenuArrow size={14} />
                                </button>
                                <button
                                  className="has-submenu danger-text"
                                  type="button"
                                  onMouseEnter={() =>
                                    setMenuState((current) =>
                                      current
                                        ? { ...current, id: item.id, submenu: 'delete' }
                                        : current,
                                    )
                                  }
                                >
                                  删除
                                  <SubmenuArrow size={14} />
                                </button>
                                {menuState.submenu === 'sync' ? (
                                  <div className="context-menu submenu-menu">
                                    {item.endpoints.map((endpoint) => {
                                      const syncAvailable = canSyncEndpoint(endpoint);
                                      return (
                                        <button
                                          key={`${item.id}-sync-${endpoint.name}`}
                                          className={syncAvailable ? '' : 'is-disabled'}
                                          disabled={!syncAvailable}
                                          type="button"
                                          onClick={() => {
                                            if (!syncAvailable) return;
                                            setMenuState(null);
                                            onRequestSyncEndpoint(item, endpoint.name);
                                          }}
                                        >
                                          {endpoint.name}
                                        </button>
                                      );
                                    })}
                                  </div>
                                ) : null}
                                {menuState.submenu === 'delete' ? (
                                  <div className="context-menu submenu-menu">
                                    {item.endpoints.map((endpoint) => {
                                      const deleteAvailable = canDeleteReplica(endpoint);
                                      return (
                                        <button
                                          key={`${item.id}-delete-${endpoint.name}`}
                                          className={deleteAvailable ? 'danger-text' : 'is-disabled'}
                                          disabled={!deleteAvailable}
                                          type="button"
                                          onClick={() => {
                                            if (!deleteAvailable) return;
                                            setMenuState(null);
                                            onRequestDeleteEndpoint(item, endpoint.name);
                                          }}
                                        >
                                          {endpoint.name}
                                        </button>
                                      );
                                    })}
                                    <button
                                      className="danger-text"
                                      type="button"
                                      onClick={() => {
                                        setMenuState(null);
                                        onDeleteAssetDirect(item);
                                      }}
                                    >
                                      删除资产
                                    </button>
                                  </div>
                                ) : null}
                                  </div>,
                                  document.body,
                                )
                              : null}
                          </div>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="storage-pagination file-center-pagination">
              <span className="selection-caption">{resultSummary}</span>
              <div className="storage-pagination-controls">
                <IconButton ariaLabel="首页" onClick={() => onSetCurrentPage(1)}>
                  <ChevronLeft size={14} />
                  <ChevronLeft size={14} />
                </IconButton>
                <IconButton ariaLabel="上一页" onClick={() => onSetCurrentPage(Math.max(1, currentPage - 1))}>
                  <ChevronLeft size={14} />
                </IconButton>
                <button className="storage-page-chip active" type="button">
                  {currentPage}
                </button>
                <IconButton ariaLabel="下一页" onClick={() => onSetCurrentPage(Math.min(pageCount, currentPage + 1))}>
                  <ChevronRight size={14} />
                </IconButton>
                <IconButton ariaLabel="末页" onClick={() => onSetCurrentPage(pageCount)}>
                  <ChevronRight size={14} />
                  <ChevronRight size={14} />
                </IconButton>
              </div>
              <label className="select-pill storage-page-size">
                <select
                  aria-label="每页数量"
                  value={String(pageSize)}
                  onChange={(event) => onSetPageSize(Number(event.target.value) as PageSize)}
                >
                  {PAGE_SIZE_OPTIONS.map((option) => (
                    <option key={option} value={option}>
                      {option}
                    </option>
                  ))}
                </select>
              </label>
              <span className="selection-caption">
                页 {currentPage}/{pageCount}
              </span>
            </div>
          </>
        )}
      </div>
    </section>
  );
}

function FileKindIcon({ item }: { item: FileCenterEntry }) {
  if (item.type === 'folder') {
    return <Folder size={16} />;
  }
  if (item.fileKind === '图片') {
    return <FileImage size={16} />;
  }
  if (item.fileKind === '视频') {
    return <File size={16} />;
  }
  if (item.fileKind === '音频') {
    return <FileAudio2 size={16} />;
  }
  if (item.fileKind === '文档') {
    return <FileText size={16} />;
  }
  return <File size={16} />;
}

function EndpointGlyph({ endpoint }: { endpoint: FileCenterEndpoint }) {
  if (endpoint.endpointType === 'cloud') {
    return <Cloud size={13} />;
  }
  if (endpoint.endpointType === 'nas') {
    return <Server size={13} />;
  }
  return <Folder size={13} />;
}

function ColorFlagBadge({ colorLabel }: { colorLabel: FileCenterColorLabel }) {
  const toneClass =
    colorLabel === '红标'
      ? 'red'
      : colorLabel === '黄标'
        ? 'yellow'
        : colorLabel === '绿标'
          ? 'green'
          : colorLabel === '蓝标'
        ? 'blue'
        : 'purple';
  return (
    <span className={`color-flag-badge ${toneClass}`} aria-label={colorLabel}>
      <Flag size={12} fill="currentColor" />
    </span>
  );
}

function canSyncEndpoint(endpoint: FileCenterEndpoint) {
  return endpoint.state === '未同步';
}

function canDeleteReplica(endpoint: FileCenterEndpoint) {
  return endpoint.state === '已同步';
}

function resolveEndpointStateClass(state: string): Severity {
  if (state === '已同步') return 'success';
  if (state === '同步中') return 'warning';
  if (state === '未同步') return 'critical';
  return 'info';
}
