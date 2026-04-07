import { useEffect, useMemo, useState } from 'react';
import { createPortal } from 'react-dom';
import { RotateCw } from 'lucide-react';
import type { WorkspaceView } from '../lib/workspaceTabs';

type MenuState = {
  view: WorkspaceView;
  x: number;
  y: number;
} | null;

export function WorkspaceTabBar({
  activeTab,
  canReopenClosed,
  labels,
  tabs,
  onActivate,
  onClose,
  onCloseLeft,
  onCloseOthers,
  onCloseRight,
  onMoveLeft,
  onMoveRight,
  onRefresh,
  onReopenLastClosed,
  onReorder,
}: {
  activeTab: WorkspaceView;
  canReopenClosed: boolean;
  labels: Record<WorkspaceView, { icon: React.ReactNode; title: string }>;
  tabs: WorkspaceView[];
  onActivate: (view: WorkspaceView) => void;
  onClose: (view: WorkspaceView) => void;
  onCloseLeft: (view: WorkspaceView) => void;
  onCloseOthers: (view: WorkspaceView) => void;
  onCloseRight: (view: WorkspaceView) => void;
  onMoveLeft: (view: WorkspaceView) => void;
  onMoveRight: (view: WorkspaceView) => void;
  onRefresh: (view: WorkspaceView) => void;
  onReopenLastClosed: () => void;
  onReorder: (source: WorkspaceView, target: WorkspaceView) => void;
}) {
  const [menuState, setMenuState] = useState<MenuState>(null);
  const [draggedView, setDraggedView] = useState<WorkspaceView | null>(null);

  useEffect(() => {
    if (!menuState) {
      return;
    }

    const handlePointerDown = (event: MouseEvent) => {
      const target = event.target as HTMLElement | null;
      if (!target) {
        return;
      }
      if (target.closest('.workspace-tab-context-menu')) {
        return;
      }
      setMenuState(null);
    };

    document.addEventListener('mousedown', handlePointerDown);
    return () => document.removeEventListener('mousedown', handlePointerDown);
  }, [menuState]);

  const activeMenuMeta = useMemo(() => {
    if (!menuState) {
      return null;
    }

    const index = tabs.indexOf(menuState.view);
    if (index === -1) {
      return null;
    }

    return {
      leftDisabled: index === 0,
      rightDisabled: index === tabs.length - 1,
      view: menuState.view,
      x: menuState.x,
      y: menuState.y,
    };
  }, [menuState, tabs]);

  return (
    <>
      <div className="workspace-tabbar-card" role="tablist" aria-label="顶层页面标签">
        {tabs.map((view) => {
          const label = labels[view];

          return (
            <div
              key={view}
              aria-selected={activeTab === view}
              className={`workspace-tab${activeTab === view ? ' active' : ''}${draggedView === view ? ' dragging' : ''}`}
              draggable
              role="tab"
              tabIndex={0}
              onClick={() => {
                setMenuState(null);
                onActivate(view);
              }}
              onContextMenu={(event) => {
                event.preventDefault();
                setMenuState({
                  view,
                  x: event.clientX,
                  y: event.clientY,
                });
              }}
              onDragEnd={() => setDraggedView(null)}
              onDragOver={(event) => {
                event.preventDefault();
                event.dataTransfer.dropEffect = 'move';
              }}
              onDragStart={(event) => {
                setDraggedView(view);
                event.dataTransfer.effectAllowed = 'move';
                event.dataTransfer.setData('text/plain', view);
              }}
              onDrop={(event) => {
                event.preventDefault();
                const sourceView = draggedView ?? (event.dataTransfer.getData('text/plain') as WorkspaceView);
                if (sourceView && sourceView !== view) {
                  onReorder(sourceView, view);
                }
                setDraggedView(null);
              }}
              onKeyDown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault();
                  setMenuState(null);
                  onActivate(view);
                }
              }}
            >
              <span className="workspace-tab-icon">{label.icon}</span>
              <span className="workspace-tab-title">{label.title}</span>
            </div>
          );
        })}
      </div>

      {activeMenuMeta
        ? createPortal(
            <div
              className="context-menu workspace-tab-context-menu"
              role="menu"
              aria-label="标签页操作"
              style={{ left: activeMenuMeta.x, top: activeMenuMeta.y }}
            >
              <button
                role="menuitem"
                type="button"
                onClick={() => {
                  setMenuState(null);
                  onRefresh(activeMenuMeta.view);
                }}
              >
                <RotateCw size={14} />
                <span>刷新当前页面</span>
              </button>
              <button
                role="menuitem"
                type="button"
                onClick={() => {
                  setMenuState(null);
                  onClose(activeMenuMeta.view);
                }}
              >
                <span>关闭当前标签</span>
              </button>
              <button
                role="menuitem"
                type="button"
                onClick={() => {
                  setMenuState(null);
                  onCloseOthers(activeMenuMeta.view);
                }}
              >
                <span>关闭其他标签</span>
              </button>
              <button
                role="menuitem"
                className={activeMenuMeta.leftDisabled ? 'is-disabled' : undefined}
                disabled={activeMenuMeta.leftDisabled}
                type="button"
                onClick={() => {
                  if (activeMenuMeta.leftDisabled) {
                    return;
                  }
                  setMenuState(null);
                  onCloseLeft(activeMenuMeta.view);
                }}
              >
                <span>关闭左侧标签</span>
              </button>
              <button
                role="menuitem"
                className={activeMenuMeta.rightDisabled ? 'is-disabled' : undefined}
                disabled={activeMenuMeta.rightDisabled}
                type="button"
                onClick={() => {
                  if (activeMenuMeta.rightDisabled) {
                    return;
                  }
                  setMenuState(null);
                  onCloseRight(activeMenuMeta.view);
                }}
              >
                <span>关闭右侧标签</span>
              </button>
              <button
                role="menuitem"
                className={!canReopenClosed ? 'is-disabled' : undefined}
                disabled={!canReopenClosed}
                type="button"
                onClick={() => {
                  if (!canReopenClosed) {
                    return;
                  }
                  setMenuState(null);
                  onReopenLastClosed();
                }}
              >
                <span>重新打开刚关闭的标签</span>
              </button>
              <button
                role="menuitem"
                className={activeMenuMeta.leftDisabled ? 'is-disabled' : undefined}
                disabled={activeMenuMeta.leftDisabled}
                type="button"
                onClick={() => {
                  if (activeMenuMeta.leftDisabled) {
                    return;
                  }
                  setMenuState(null);
                  onMoveLeft(activeMenuMeta.view);
                }}
              >
                <span>向左移动</span>
              </button>
              <button
                role="menuitem"
                className={activeMenuMeta.rightDisabled ? 'is-disabled' : undefined}
                disabled={activeMenuMeta.rightDisabled}
                type="button"
                onClick={() => {
                  if (activeMenuMeta.rightDisabled) {
                    return;
                  }
                  setMenuState(null);
                  onMoveRight(activeMenuMeta.view);
                }}
              >
                <span>向右移动</span>
              </button>
            </div>,
            document.body,
          )
        : null}
    </>
  );
}
