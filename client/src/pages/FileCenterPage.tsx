import { ChevronLeft, ChevronRight, FolderPlus, RefreshCw, Search, Trash2 } from 'lucide-react';
import type { FileNode, FileTypeFilter } from '../data';
import type { PageSize } from '../App';
import {
  ActionButton,
  EmptyState,
  IconButton,
  SelectPill,
  TonePill,
} from '../components/Shared';

export function FileCenterPage(props: {
  breadcrumbs: string[];
  canGoBack: boolean;
  canGoForward: boolean;
  currentEntries: FileNode[];
  currentPage: number;
  currentPathChildren: number;
  fileTypeFilter: FileTypeFilter;
  pageCount: number;
  pageSize: PageSize;
  searchText: string;
  selectedIds: string[];
  sortValue: string;
  onChangeSort: (value: string) => void;
  onCreateFolder: () => void;
  onGoBack: () => void;
  onGoForward: () => void;
  onNavigateBreadcrumb: (index: number) => void;
  onOpenItem: (item: FileNode) => void;
  onOpenItemDetail: (item: FileNode) => void;
  onDeleteSelected: () => void;
  onRefreshIndex: () => void;
  onSetCurrentPage: (value: number) => void;
  onSetFileTypeFilter: (value: FileTypeFilter) => void;
  onSetPageSize: (value: PageSize) => void;
  onSetSearchText: (value: string) => void;
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
    pageCount,
    pageSize,
    searchText,
    selectedIds,
    sortValue,
    onChangeSort,
    onCreateFolder,
    onGoBack,
    onGoForward,
    onNavigateBreadcrumb,
    onOpenItem,
    onOpenItemDetail,
    onDeleteSelected,
    onRefreshIndex,
    onSetCurrentPage,
    onSetFileTypeFilter,
    onSetPageSize,
    onSetSearchText,
    onToggleSelect,
    onToggleSelectVisible,
  } = props;

  const allVisibleSelected =
    currentEntries.length > 0 && currentEntries.every((item) => selectedIds.includes(item.id));

  return (
    <section className="page-stack">
      <div className="toolbar-card explorer-toolbar">
        <div className="toolbar-group">
          <IconButton active={canGoBack} ariaLabel="后退" onClick={onGoBack}>
            <ChevronLeft size={16} />
          </IconButton>
          <IconButton active={canGoForward} ariaLabel="前进" onClick={onGoForward}>
            <ChevronRight size={16} />
          </IconButton>
        </div>

        <div className="address-bar">
          {breadcrumbs.map((item, index) => (
            <button key={`${item}-${index}`} type="button" onClick={() => onNavigateBreadcrumb(index)}>
              {item}
            </button>
          ))}
        </div>

        <div className="toolbar-group wrap">
          <label className="search-field" htmlFor="file-search">
            <Search size={14} />
            <input
              id="file-search"
              type="search"
              placeholder="搜索当前目录"
              value={searchText}
              onChange={(event) => onSetSearchText(event.target.value)}
            />
          </label>
          <SelectPill
            ariaLabel="文件类型筛选"
            options={['全部', '文件夹', '图片', '视频', '音频', '文档']}
            value={fileTypeFilter}
            onChange={(value) => onSetFileTypeFilter(value as FileTypeFilter)}
          />
          <SelectPill
            ariaLabel="文件排序"
            options={['修改时间', '名称', '大小']}
            value={sortValue}
            onChange={onChangeSort}
          />
        </div>
      </div>

      <div className="toolbar-card action-toolbar">
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
        <div className="toolbar-group wrap">
          <span className="selection-caption">已选 {selectedIds.length} 项</span>
          <ActionButton ariaLabel="删除资产" tone="danger" onClick={onDeleteSelected}>
            <Trash2 size={14} />
            删除资产
          </ActionButton>
        </div>
      </div>

      <div className="workspace-card">
        {currentEntries.length === 0 ? (
          <EmptyState
            title="当前目录暂无匹配结果"
            description="可以调整筛选条件、清空搜索词，或者直接新建一个目录。"
            action={<ActionButton onClick={onCreateFolder}>新建目录</ActionButton>}
          />
        ) : (
          <table className="file-table">
            <thead>
              <tr>
                <th scope="col">
                  <input
                    aria-label="选择当前页全部文件"
                    checked={allVisibleSelected}
                    type="checkbox"
                    onChange={onToggleSelectVisible}
                  />
                </th>
                <th scope="col">名称</th>
                <th scope="col">修改日期</th>
                <th scope="col">类型</th>
                <th scope="col">大小</th>
                <th scope="col">状态</th>
                <th scope="col">操作</th>
              </tr>
            </thead>
            <tbody>
              {currentEntries.map((item) => (
                <tr key={item.id} onDoubleClick={() => onOpenItem(item)}>
                  <td>
                    <input
                      aria-label={`选择 ${item.name}`}
                      checked={selectedIds.includes(item.id)}
                      type="checkbox"
                      onChange={() => onToggleSelect(item.id)}
                    />
                  </td>
                  <td>
                    <div className="file-name-cell">
                      <span className={`file-kind-icon ${item.type}`}>{item.type === 'folder' ? '夹' : '件'}</span>
                      <div>
                        <strong>{item.name}</strong>
                        <span>{item.path}</span>
                      </div>
                    </div>
                  </td>
                  <td>{item.modifiedAt}</td>
                  <td>{item.displayType}</td>
                  <td>{item.size}</td>
                  <td>
                    <div className="endpoint-row">
                      {item.endpoints.map((endpoint) => (
                        <TonePill key={`${item.id}-${endpoint.name}`} tone={endpoint.tone}>
                          {endpoint.name} · {endpoint.state}
                        </TonePill>
                      ))}
                    </div>
                  </td>
                  <td>
                    <div className="row-actions">
                      <ActionButton onClick={() => onOpenItem(item)}>{item.type === 'folder' ? '进入' : '打开'}</ActionButton>
                      <ActionButton onClick={() => onOpenItemDetail(item)}>详情</ActionButton>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <div className="footer-toolbar">
        <span>当前目录 {currentPathChildren} 项</span>
        <div className="toolbar-group">
          <SelectPill
            ariaLabel="每页数量"
            options={['10', '20', '50', '100']}
            value={String(pageSize)}
            onChange={(value) => onSetPageSize(Number(value) as PageSize)}
          />
          <div className="pager">
            <IconButton ariaLabel="上一页" onClick={() => onSetCurrentPage(Math.max(1, currentPage - 1))}>
              <ChevronLeft size={16} />
            </IconButton>
            <span>
              {currentPage} / {pageCount}
            </span>
            <IconButton ariaLabel="下一页" onClick={() => onSetCurrentPage(Math.min(pageCount, currentPage + 1))}>
              <ChevronRight size={16} />
            </IconButton>
          </div>
        </div>
      </div>
    </section>
  );
}
