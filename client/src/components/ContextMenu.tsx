import type { StorageNode, TaskRecord } from '../data';
import type { ContextMenuTarget } from '../App';
import type { FileCenterEntry } from '../lib/fileCenterApi';

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
  onOpenFileDetail: (item: FileCenterEntry) => void;
  onOpenFolder: (item: FileCenterEntry) => void;
  onOpenStorageDetail: (item: StorageNode) => void;
  onOpenTaskDetail: (item: TaskRecord) => void;
}) {
  if (!menu) {
    return null;
  }

  return (
    <div className="context-menu" style={{ left: menu.x, top: menu.y }}>
      {menu.type === 'file' && (
        <>
          <button
            type="button"
            onClick={() => {
              if (menu.item.type === 'folder') {
                onOpenFolder(menu.item);
              } else {
                onOpenFileDetail(menu.item);
              }
              onClose();
            }}
          >
            {menu.item.type === 'folder' ? '进入目录' : '查看详情'}
          </button>
        </>
      )}

      {menu.type === 'task' && (
        <button
          type="button"
          onClick={() => {
            onOpenTaskDetail(menu.item);
            onClose();
          }}
        >
          查看任务详情
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
          编辑节点
        </button>
      )}
    </div>
  );
}
