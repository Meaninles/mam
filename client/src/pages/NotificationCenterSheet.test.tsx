import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import type { NoticeRecord } from '../data';
import { NotificationCenterSheet } from './NotificationCenterSheet';

function createNotice(overrides: Partial<NoticeRecord>): NoticeRecord {
  return {
    id: 'notice-1',
    kind: 'ACTION_REQUIRED',
    sourceType: 'ISSUE',
    sourceId: 'issue-1',
    issueId: 'issue-1',
    title: 'CloudDrive2 网关离线',
    summary: 'CD2 健康检查失败',
    severity: 'warning',
    objectLabel: 'CloudDrive2',
    status: 'UNREAD',
    createdAt: '2026-04-14 10:00',
    updatedAt: '2026-04-14 10:05',
    sortKey: 1,
    libraryId: 'photo',
    source: {
      sourceDomain: '异常中心',
      issueCategory: '节点与权限',
      issueNature: 'BLOCKING',
      issueSourceDomain: '系统治理',
      sourceLabel: '依赖服务',
      routeLabel: '设置 / 依赖服务',
    },
    capabilities: {
      canMarkRead: true,
      canOpenIssueCenter: true,
      canOpenTaskCenter: false,
      canOpenFileCenter: false,
      canOpenStorageNodes: false,
      canOpenImportCenter: false,
    },
    jumpParams: {
      kind: 'issues',
      issueId: 'issue-1',
      label: '定位异常：CloudDrive2 网关离线',
    },
    ...overrides,
  };
}

describe('NotificationCenterSheet', () => {
  it('会把 CloudDrive2 和 aria2 运行态问题优先跳转到设置页', async () => {
    const user = userEvent.setup();
    const onOpenTarget = vi.fn();

    render(
      <NotificationCenterSheet
        noticeRecords={[createNotice({ id: 'notice-cd2' })]}
        onMarkRead={vi.fn()}
        onOpenTarget={onOpenTarget}
      />,
    );

    await user.click(screen.getByRole('button', { name: /去处理 CloudDrive2 网关离线/ }));
    expect(onOpenTarget).toHaveBeenCalledWith(expect.objectContaining({ id: 'notice-cd2' }), 'settings');
  });

  it('会为云端运行态通知提供打开设置页菜单项', async () => {
    const user = userEvent.setup();
    const onOpenTarget = vi.fn();

    render(
      <NotificationCenterSheet
        noticeRecords={[createNotice({ id: 'notice-cd2' })]}
        onMarkRead={vi.fn()}
        onOpenTarget={onOpenTarget}
      />,
    );

    await user.click(screen.getByRole('button', { name: '更多操作 CloudDrive2 网关离线' }));
    await user.click(screen.getByRole('button', { name: '打开设置页' }));

    expect(onOpenTarget).toHaveBeenCalledWith(expect.objectContaining({ id: 'notice-cd2' }), 'settings');
  });

  it('会把 115 鉴权失效类通知优先跳转到存储节点', async () => {
    const user = userEvent.setup();
    const onOpenTarget = vi.fn();

    render(
      <NotificationCenterSheet
        noticeRecords={[
          createNotice({
            id: 'notice-115-auth',
            title: '115 Token 失效',
            summary: '115 鉴权失败，请重新登录',
            objectLabel: '115 云归档',
            source: {
              sourceDomain: '异常中心',
              issueCategory: '节点与权限',
              issueNature: 'BLOCKING',
              issueSourceDomain: '存储节点',
              sourceLabel: '115 云归档',
              routeLabel: '存储节点 / 网盘管理',
            },
            jumpParams: {
              kind: 'issues',
              issueId: 'issue-1',
              label: '定位异常：115 Token 失效',
            },
            capabilities: {
              canMarkRead: true,
              canOpenIssueCenter: true,
              canOpenStorageNodes: true,
            },
          }),
        ]}
        onMarkRead={vi.fn()}
        onOpenTarget={onOpenTarget}
      />,
    );

    await user.click(screen.getByRole('button', { name: /去处理 115 Token 失效/ }));
    expect(onOpenTarget).toHaveBeenCalledWith(expect.objectContaining({ id: 'notice-115-auth' }), 'storage-nodes');
  });
});
