import { describe, expect, it } from 'vitest';
import {
  closeWorkspaceTab,
  insertWorkspaceTab,
  moveWorkspaceTab,
  reopenLastClosedWorkspace,
  reorderWorkspaceTab,
  type WorkspaceView,
} from './workspaceTabs';

describe('workspaceTabs', () => {
  it('在当前标签右侧插入未打开的一级页面标签', () => {
    const order: WorkspaceView[] = ['file-center', 'task-center'];

    expect(insertWorkspaceTab(order, 'file-center', 'issues')).toEqual([
      'file-center',
      'issues',
      'task-center',
    ]);
  });

  it('再次打开已存在的一级页面标签时不重复插入', () => {
    const order: WorkspaceView[] = ['file-center', 'task-center', 'issues'];

    expect(insertWorkspaceTab(order, 'task-center', 'issues')).toEqual(order);
  });

  it('关闭当前标签后优先激活右侧标签，否则激活左侧标签', () => {
    const order: WorkspaceView[] = ['file-center', 'task-center', 'issues'];

    expect(closeWorkspaceTab(order, 'task-center', 'task-center')).toEqual({
      nextActive: 'issues',
      nextOrder: ['file-center', 'issues'],
      closed: 'task-center',
    });

    expect(closeWorkspaceTab(order, 'issues', 'issues')).toEqual({
      nextActive: 'task-center',
      nextOrder: ['file-center', 'task-center'],
      closed: 'issues',
    });
  });

  it('移动标签时只在存在相邻标签时生效', () => {
    const order: WorkspaceView[] = ['file-center', 'task-center', 'issues'];

    expect(moveWorkspaceTab(order, 'task-center', 'left')).toEqual([
      'task-center',
      'file-center',
      'issues',
    ]);
    expect(moveWorkspaceTab(order, 'task-center', 'right')).toEqual([
      'file-center',
      'issues',
      'task-center',
    ]);
    expect(moveWorkspaceTab(order, 'file-center', 'left')).toEqual(order);
  });

  it('支持按拖拽结果重排标签顺序', () => {
    const order: WorkspaceView[] = ['file-center', 'task-center', 'issues', 'storage-nodes'];

    expect(reorderWorkspaceTab(order, 'storage-nodes', 'task-center')).toEqual([
      'file-center',
      'storage-nodes',
      'task-center',
      'issues',
    ]);
  });

  it('可恢复最近关闭的一级页面标签', () => {
    const order: WorkspaceView[] = ['file-center', 'issues'];

    expect(reopenLastClosedWorkspace(order, ['task-center', 'settings'])).toEqual({
      nextActive: 'task-center',
      nextClosedStack: ['settings'],
      nextOrder: ['file-center', 'issues', 'task-center'],
      reopened: 'task-center',
    });
  });
});
