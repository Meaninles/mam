import { describe, expect, it } from 'vitest';
import type { NoticeRecord } from '../data';
import {
  applyNoticeConsumptions,
  consumeReminderNoticeIds,
  createEmptyNoticeConsumptionState,
  markNoticeConsumed,
  pruneNoticeConsumptions,
} from './noticeCenter';

const BASE_NOTICE: Omit<NoticeRecord, 'id' | 'title' | 'summary' | 'sourceType' | 'sourceId' | 'objectLabel'> = {
  kind: 'REMINDER',
  severity: 'info',
  status: 'UNREAD',
  createdAt: '2026-04-12T08:00:00Z',
  updatedAt: '2026-04-12T08:01:00Z',
  sortKey: Date.parse('2026-04-12T08:01:00Z'),
  source: {
    sourceDomain: '任务中心',
    sourceLabel: '任务中心',
    routeLabel: '任务中心 / 同步',
  },
  capabilities: {
    canMarkRead: true,
    canOpenTaskCenter: true,
  },
  jumpParams: {
    kind: 'task-center',
    taskId: 'job-1',
    label: '查看任务',
  },
};

describe('noticeCenter consumption state', () => {
  it('会在本地消费时间早于服务端更新时间时重新视为未消费', () => {
    const state = markNoticeConsumed(createEmptyNoticeConsumptionState(), 'notice-1', 'READ', '2026-04-12T08:00:30Z');
    const records = applyNoticeConsumptions(
      [
        {
          ...BASE_NOTICE,
          id: 'notice-1',
          title: '同步完成',
          summary: '已完成 12 项同步',
          sourceType: 'JOB',
          sourceId: 'job-1',
          objectLabel: '商业摄影资产库 / Shanghai Launch',
          updatedAt: '2026-04-12T08:01:00Z',
          sortKey: Date.parse('2026-04-12T08:01:00Z'),
        },
      ],
      state,
    );

    expect(records[0].status).toBe('UNREAD');
  });

  it('同一异常通知即使服务端更新时间变化，也保持本地已读状态', () => {
    const state = markNoticeConsumed(createEmptyNoticeConsumptionState(), 'notice-issue-1', 'READ', '2026-04-12T08:00:30Z');
    const records = applyNoticeConsumptions(
      [
        {
          ...BASE_NOTICE,
          id: 'notice-issue-1',
          kind: 'ACTION_REQUIRED',
          title: '影像 NAS 01 共享目录写入权限异常',
          summary: '已累计发生 3 次',
          sourceType: 'ISSUE',
          sourceId: 'issue-1',
          issueId: 'issue-1',
          objectLabel: '影像 NAS 01 / 共享目录',
          updatedAt: '2026-04-12T08:05:00Z',
          sortKey: Date.parse('2026-04-12T08:05:00Z'),
          jumpParams: {
            kind: 'issues',
            issueId: 'issue-1',
            label: '定位异常：影像 NAS 01 共享目录写入权限异常',
          },
        },
      ],
      state,
      { activeIssueIds: ['issue-1'] },
    );

    expect(records[0].status).toBe('READ');
    expect(records[0].readAt).toBe('2026-04-12T08:00:30Z');
  });

  it('打开通知中心时只会自动消费提醒类通知', () => {
    const state = consumeReminderNoticeIds(
      createEmptyNoticeConsumptionState(),
      [
        {
          ...BASE_NOTICE,
          id: 'notice-1',
          title: '同步完成',
          summary: '已完成',
          sourceType: 'JOB',
          sourceId: 'job-1',
          objectLabel: '商业摄影资产库 / Shanghai Launch',
        },
        {
          ...BASE_NOTICE,
          id: 'notice-2',
          kind: 'ACTION_REQUIRED',
          title: '扫描失败',
          summary: '需要处理',
          sourceType: 'ISSUE',
          sourceId: 'issue-1',
          issueId: 'issue-1',
          objectLabel: '商业摄影资产库 / Shanghai Launch',
          jumpParams: { kind: 'issues', issueId: 'issue-1', label: '处理异常' },
        },
      ],
      '2026-04-12T08:02:00Z',
    );

    expect(state.byId['notice-1']?.status).toBe('READ');
    expect(state.byId['notice-2']).toBeUndefined();
  });

  it('会清理已不存在通知的本地消费状态', () => {
    const state = {
      byId: {
        'notice-1': { status: 'READ' as const, consumedAt: '2026-04-12T08:00:00Z' },
        'notice-2': { status: 'JUMPED' as const, consumedAt: '2026-04-12T08:01:00Z' },
      },
    };

    const next = pruneNoticeConsumptions(state, ['notice-2']);

    expect(next.byId['notice-1']).toBeUndefined();
    expect(next.byId['notice-2']).toBeDefined();
  });
});
