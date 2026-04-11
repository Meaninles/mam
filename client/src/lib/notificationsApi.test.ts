import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { notificationsApi } from './notificationsApi';

describe('notificationsApi', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => ({
        ok: true,
        json: async () => ({
          data: {
            items: [
              {
                id: 'notice-job-1',
                kind: 'REMINDER',
                sourceType: 'JOB',
                sourceId: 'job-1',
                lifecycleStatus: 'ACTIVE',
                title: '同步任务已完成',
                summary: '已完成 12 项同步',
                severity: 'INFO',
                libraryId: 'photo',
                objectLabel: '商业摄影资产库 / 2026 / Shanghai Launch',
                createdAt: '2026-04-12T08:00:00Z',
                updatedAt: '2026-04-12T08:01:00Z',
                source: {
                  sourceDomain: 'TASK_CENTER',
                  sourceLabel: '任务中心',
                  routeLabel: '任务中心 / 同步',
                  taskId: 'job-1',
                  fileNodeId: 'dir-launch',
                },
                capabilities: {
                  canMarkRead: true,
                  canOpenTaskCenter: true,
                  canOpenFileCenter: true,
                },
                jumpParams: {
                  kind: 'task-center',
                  taskId: 'job-1',
                  libraryId: 'photo',
                  fileNodeId: 'dir-launch',
                  label: '查看同步结果',
                },
              },
            ],
            total: 1,
            page: 1,
            pageSize: 20,
          },
        }),
      })),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('会把服务端通知映射为客户端通知记录', async () => {
    const response = await notificationsApi.list({ page: 1, pageSize: 20 });

    expect(response.total).toBe(1);
    expect(response.items[0]).toMatchObject({
      id: 'notice-job-1',
      kind: 'REMINDER',
      sourceType: 'JOB',
      status: 'UNREAD',
      libraryId: 'photo',
      objectLabel: '商业摄影资产库 / 2026 / Shanghai Launch',
    });
    expect(response.items[0].sortKey).toBeGreaterThan(0);
    expect(response.items[0].source.sourceDomain).toBe('任务中心');
    expect(response.items[0].jumpParams.kind).toBe('task-center');
  });
});
