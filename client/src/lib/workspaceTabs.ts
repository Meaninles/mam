import type { MainView } from '../data';

export type WorkspaceView = Exclude<MainView, 'import-center'>;

export const DEFAULT_WORKSPACE_VIEW: WorkspaceView = 'file-center';

export function insertWorkspaceTab(
  order: WorkspaceView[],
  active: WorkspaceView,
  next: WorkspaceView,
): WorkspaceView[] {
  if (order.includes(next)) {
    return order;
  }

  const activeIndex = order.indexOf(active);
  if (activeIndex === -1) {
    return [...order, next];
  }

  const nextOrder = [...order];
  nextOrder.splice(activeIndex + 1, 0, next);
  return nextOrder;
}

export function closeWorkspaceTab(
  order: WorkspaceView[],
  active: WorkspaceView,
  target: WorkspaceView,
): {
  nextOrder: WorkspaceView[];
  nextActive: WorkspaceView | null;
  closed: WorkspaceView;
} {
  const targetIndex = order.indexOf(target);
  if (targetIndex === -1) {
    return {
      nextOrder: order,
      nextActive: active,
      closed: target,
    };
  }

  const nextOrder = order.filter((view) => view !== target);
  if (nextOrder.length === 0) {
    return {
      nextOrder,
      nextActive: null,
      closed: target,
    };
  }

  if (active !== target) {
    return {
      nextOrder,
      nextActive: active,
      closed: target,
    };
  }

  const rightNeighbor = order[targetIndex + 1];
  const leftNeighbor = order[targetIndex - 1];

  return {
    nextOrder,
    nextActive: (rightNeighbor && nextOrder.includes(rightNeighbor) ? rightNeighbor : leftNeighbor) ?? nextOrder[0],
    closed: target,
  };
}

export function moveWorkspaceTab(
  order: WorkspaceView[],
  target: WorkspaceView,
  direction: 'left' | 'right',
): WorkspaceView[] {
  const currentIndex = order.indexOf(target);
  if (currentIndex === -1) {
    return order;
  }

  const nextIndex = direction === 'left' ? currentIndex - 1 : currentIndex + 1;
  if (nextIndex < 0 || nextIndex >= order.length) {
    return order;
  }

  const nextOrder = [...order];
  const [item] = nextOrder.splice(currentIndex, 1);
  nextOrder.splice(nextIndex, 0, item);
  return nextOrder;
}

export function reorderWorkspaceTab(
  order: WorkspaceView[],
  source: WorkspaceView,
  target: WorkspaceView,
): WorkspaceView[] {
  if (source === target) {
    return order;
  }

  const sourceIndex = order.indexOf(source);
  const targetIndex = order.indexOf(target);
  if (sourceIndex === -1 || targetIndex === -1) {
    return order;
  }

  const nextOrder = [...order];
  const [item] = nextOrder.splice(sourceIndex, 1);
  const insertionIndex = nextOrder.indexOf(target);
  nextOrder.splice(insertionIndex, 0, item);
  return nextOrder;
}

export function reopenLastClosedWorkspace(
  order: WorkspaceView[],
  closedStack: WorkspaceView[],
): {
  nextOrder: WorkspaceView[];
  nextActive: WorkspaceView | null;
  nextClosedStack: WorkspaceView[];
  reopened: WorkspaceView | null;
} {
  const reopened = closedStack[0] ?? null;
  if (!reopened) {
    return {
      nextOrder: order,
      nextActive: null,
      nextClosedStack: closedStack,
      reopened: null,
    };
  }

  if (order.includes(reopened)) {
    return {
      nextOrder: order,
      nextActive: reopened,
      nextClosedStack: closedStack.slice(1),
      reopened,
    };
  }

  return {
    nextOrder: [...order, reopened],
    nextActive: reopened,
    nextClosedStack: closedStack.slice(1),
    reopened,
  };
}
