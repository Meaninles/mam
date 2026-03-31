import { Pencil, Trash2 } from 'lucide-react';
import type { StorageNode, StorageTypeFilter } from '../data';
import type { ContextMenuTarget } from '../App';
import { DenseRow, IconButton, SelectPill, Sheet } from '../components/Shared';

export function StorageNodesPage({
  items,
  setContextMenu,
  setStorageDetail,
  setStorageTypeFilter,
  typeFilter,
}: {
  items: StorageNode[];
  setContextMenu: (value: ContextMenuTarget) => void;
  setStorageDetail: (value: StorageNode | null) => void;
  setStorageTypeFilter: (value: StorageTypeFilter) => void;
  typeFilter: StorageTypeFilter;
}) {
  return (
    <section className="page-stack">
      <div className="toolbar-card">
        <div className="toolbar-group">
          <SelectPill
            ariaLabel="存储节点类型"
            options={['全部', '本机磁盘', '移动硬盘', 'NAS/SMB', '115网盘']}
            value={typeFilter}
            onChange={(value) => setStorageTypeFilter(value as StorageTypeFilter)}
          />
        </div>
      </div>

      <div className="workspace-card compact-list">
        {items.map((item) => (
          <div
            className="list-row enterable"
            key={item.id}
            onContextMenu={(event) => {
              event.preventDefault();
              setContextMenu({ type: 'storage', item, x: event.clientX, y: event.clientY });
            }}
            onDoubleClick={() => setStorageDetail(item)}
          >
            <div className="row-main">
              <strong>{item.name}</strong>
              <span>{item.address}</span>
            </div>
            <span>{item.nodeType}</span>
            <span>{item.mountMode}</span>
            <span>{item.freeSpace}</span>
            <span>{item.lastCheck}</span>
          </div>
        ))}
      </div>
    </section>
  );
}

export function StorageNodeSheet({ item, onClose }: { item: StorageNode; onClose: () => void }) {
  return (
    <Sheet onClose={onClose} title={item.name}>
      <div className="sheet-section">
        <DenseRow label="节点类型" value={item.nodeType} />
        <DenseRow label="地址" value={item.address} />
        <DenseRow label="挂载模式" value={item.mountMode} />
        <DenseRow label="状态" value={item.status} />
        <DenseRow label="剩余空间" value={item.freeSpace} />
        <DenseRow label="最近检测" value={item.lastCheck} />
      </div>
      <div className="sheet-actions right">
        <IconButton ariaLabel="修改">
          <Pencil size={16} />
        </IconButton>
        <IconButton ariaLabel="删除">
          <Trash2 size={16} />
        </IconButton>
      </div>
    </Sheet>
  );
}
