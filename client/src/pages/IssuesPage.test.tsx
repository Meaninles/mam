import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import type { IssueRecord, Library } from '../data';
import { IssuesPage } from './IssuesPage';

const libraries: Library[] = [
  { id: 'photo', name: '商业摄影资产库', rootLabel: '/', itemCount: '0', health: '100%', storagePolicy: '双副本' },
];

function createIssue(overrides: Partial<IssueRecord>): IssueRecord {
  return {
    id: 'issue-1',
    libraryId: 'photo',
    category: '节点与权限',
    type: 'GATEWAY_FAILED',
    nature: 'BLOCKING',
    sourceDomain: '系统治理',
    severity: 'critical',
    title: 'CloudDrive2 网关离线',
    summary: 'CD2 健康检查失败',
    asset: 'CloudDrive2',
    objectLabel: 'CloudDrive2',
    action: '检查依赖服务',
    actionLabel: '查看详情',
    suggestion: '前往设置页恢复依赖服务',
    detail: 'CloudDrive2 reconnect failed',
    status: '待处理',
    createdAt: '2026-04-14 10:00',
    updatedAt: '2026-04-14 10:05',
    source: {
      sourceDomain: '系统治理',
      sourceLabel: '依赖服务',
      routeLabel: '设置 / 依赖服务',
    },
    impact: {
      assetCount: 0,
      replicaCount: 0,
      folderCount: 0,
      endpointCount: 1,
      blocksStatusCommit: true,
      blocksTaskExecution: true,
    },
    capabilities: {
      canOpenStorageNodes: true,
    },
    histories: [],
    ...overrides,
  };
}

describe('IssuesPage', () => {
  it('会在异常详情中展示云端问题类型，并允许 CD2/aria2 问题直达设置页', async () => {
    const user = userEvent.setup();
    const onOpenSettings = vi.fn();

    render(
      <IssuesPage
        issues={[createIssue({ id: 'issue-cd2' })]}
        libraries={libraries}
        focusRequest={{ issueId: 'issue-cd2' }}
        onConsumeFocusRequest={vi.fn()}
        onIssueAction={vi.fn()}
        onClearHistory={vi.fn()}
        onOpenFileCenter={vi.fn()}
        onOpenSettings={onOpenSettings}
        onOpenStorageNodes={vi.fn()}
        onOpenTaskCenter={vi.fn()}
      />,
    );

    expect(await screen.findByText('云端问题类型')).toBeInTheDocument();
    expect(screen.getByText('CloudDrive2 网关问题')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '打开设置页' }));
    expect(onOpenSettings).toHaveBeenCalledTimes(1);
  });

  it('会把 115 鉴权问题保留为存储节点治理入口', async () => {
    render(
      <IssuesPage
        issues={[
          createIssue({
            id: 'issue-115-auth',
            title: '115 Token 失效',
            summary: '115 鉴权失败',
            type: 'TOKEN_INVALID',
            objectLabel: '115 云归档',
            action: '重新扫码登录',
            detail: 'cookie rejected by 115',
          }),
        ]}
        libraries={libraries}
        focusRequest={{ issueId: 'issue-115-auth' }}
        onConsumeFocusRequest={vi.fn()}
        onIssueAction={vi.fn()}
        onClearHistory={vi.fn()}
        onOpenFileCenter={vi.fn()}
        onOpenSettings={vi.fn()}
        onOpenStorageNodes={vi.fn()}
        onOpenTaskCenter={vi.fn()}
      />,
    );

    expect(await screen.findByText('云端问题类型')).toBeInTheDocument();
    expect(screen.getByText('115 鉴权问题')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '打开存储节点' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '打开设置页' })).not.toBeInTheDocument();
  });
});
