import type { FileNode, StorageNode, TaskRecord } from '../data';
import type { ContextMenuTarget } from '../App';

export function ContextMenu({
  menu,
  onClose,
  onOpenFileDetail,
  onOpenFolder,
  onOpenStorageDetail,
  onOpenTaskDetail,
}: {
  menu: ContextMenuTarget;
  onClose: () => void;
  onOpenFileDetail: (item: FileNode) => void;
  onOpenFolder: (item: FileNode) => void;
  onOpenStorageDetail: (item: StorageNode) => void;
  onOpenTaskDetail: (item: TaskRecord) => void;
}) {
  if (!menu) {
    return null;
  }

  return (
    <div className="context-menu" style={{ left: menu.x, top: menu.y }}>
      {menu.type === 'file' && menu.item.type === 'folder' && (
        <button
          type="button"
          onClick={() => {
            onOpenFolder(menu.item);
            onClose();
          }}
        >
          进入
        </button>
      )}

      {menu.type === 'file' && menu.item.type === 'file' && (
        <button
          type="button"
          onClick={() => {
            onOpenFileDetail(menu.item);
            onClose();
          }}
        >
          详情
        </button>
      )}

      {menu.type === 'task' && (
        <button
          type="button"
          onClick={() => {
            onOpenTaskDetail(menu.item);
            onClose();
          }}
        >
          进入
        </button>
      )}

      {menu.type === 'storage' && (
        <button
          type="button"
          onClick={() => {
            onOpenStorageDetail(menu.item);
            onClose();
          }}
        >
          管理
        </button>
      )}
    </div>
  );
}
