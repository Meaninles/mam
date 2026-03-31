import { ChevronLeft, ChevronRight, Search } from 'lucide-react';
import type { FileNode, FileTypeFilter } from '../data';
import type { ContextMenuTarget, PageSize } from '../App';
import { IconButton, SelectPill, TonePill } from '../components/Shared';

export function FileCenterPage(props: {
  breadcrumbs: string[];
  currentEntries: FileNode[];
  currentPage: number;
  currentPathChildren: number;
  fileTypeFilter: FileTypeFilter;
  pageCount: number;
  pageSize: PageSize;
  searchText: string;
  setContextMenu: (value: ContextMenuTarget) => void;
  setCurrentFolderId: (value: string | null) => void;
  setCurrentPage: (value: number) => void;
  setFileDetail: (value: FileNode | null) => void;
  setFileTypeFilter: (value: FileTypeFilter) => void;
  setPageSize: (value: PageSize) => void;
  setSearchText: (value: string) => void;
}) {
  const {
    breadcrumbs,
    currentEntries,
    currentPage,
    currentPathChildren,
    fileTypeFilter,
    pageCount,
    pageSize,
    searchText,
    setContextMenu,
    setCurrentFolderId,
    setCurrentPage,
    setFileDetail,
    setFileTypeFilter,
    setPageSize,
    setSearchText,
  } = props;

  const openItem = (item: FileNode) => {
    if (item.type === 'folder') {
      setCurrentFolderId(item.id);
      return;
    }

    setFileDetail(item);
  };

  return (
    <section className="page-stack">
      <div className="toolbar-card explorer-toolbar">
        <div className="toolbar-group">
          <IconButton ariaLabel="后退">
            <ChevronLeft size={16} />
          </IconButton>
          <IconButton ariaLabel="前进">
            <ChevronRight size={16} />
          </IconButton>
        </div>

        <div className="address-bar">
          {breadcrumbs.map((item, index) => (
            <span key={`${item}-${index}`}>{item}</span>
          ))}
        </div>

        <div className="toolbar-group">
          <label className="search-field" htmlFor="file-search">
            <Search size={14} />
            <input
              id="file-search"
              type="search"
              placeholder="搜索当前目录"
              value={searchText}
              onChange={(event) => setSearchText(event.target.value)}
            />
          </label>
          <SelectPill
            ariaLabel="文件类型筛选"
            options={['全部', '文件夹', '图片', '视频', '音频', '文档']}
            value={fileTypeFilter}
            onChange={(value) => setFileTypeFilter(value as FileTypeFilter)}
          />
        </div>
      </div>

      <div className="workspace-card">
        <table className="file-table">
          <thead>
            <tr>
              <th scope="col">名称</th>
              <th scope="col">修改日期</th>
              <th scope="col">类型</th>
              <th scope="col">大小</th>
              <th scope="col">状态</th>
            </tr>
          </thead>
          <tbody>
            {currentEntries.map((item) => (
              <tr
                key={item.id}
                onContextMenu={(event) => {
                  event.preventDefault();
                  setContextMenu({ type: 'file', item, x: event.clientX, y: event.clientY });
                }}
                onDoubleClick={() => openItem(item)}
              >
                <td>
                  <div className="file-name-cell">
                    <span className={`file-kind-icon ${item.type}`}>
                      {item.type === 'folder' ? '夹' : '件'}
                    </span>
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
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="footer-toolbar">
        <span>当前目录 {currentPathChildren} 项</span>
        <div className="toolbar-group">
          <SelectPill
            ariaLabel="每页数量"
            options={['10', '20', '50', '100']}
            value={String(pageSize)}
            onChange={(value) => setPageSize(Number(value) as PageSize)}
          />
          <div className="pager">
            <IconButton ariaLabel="上一页" onClick={() => setCurrentPage(Math.max(1, currentPage - 1))}>
              <ChevronLeft size={16} />
            </IconButton>
            <span>
              {currentPage} / {pageCount}
            </span>
            <IconButton ariaLabel="下一页" onClick={() => setCurrentPage(Math.min(pageCount, currentPage + 1))}>
              <ChevronRight size={16} />
            </IconButton>
          </div>
        </div>
      </div>
    </section>
  );
}
