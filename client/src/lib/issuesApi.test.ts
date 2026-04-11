// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { issuesApi } from './issuesApi';

vi.mock('./runtimeConfig', () => ({
  getRuntimeConfig: () => ({
    centerBaseUrl: 'http://127.0.0.1:8080',
  }),
}));

describe('issuesApi', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('从中心服务读取异常并映射为客户端记录', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          data: {
            items: [
              {
                id: 'issue-1',
                code: 'issue-1',
                libraryId: 'photo',
                taskId: 'job-1',
                taskItemId: 'item-1',
                issueCategory: 'TRANSFER',
                issueType: 'REPLICATE_FAILED',
                nature: 'BLOCKING',
                sourceDomain: 'TRANSFER_JOB',
                severity: 'CRITICAL',
                status: 'OPEN',
                title: '封面图同步失败',
                summary: '目标端点写入失败',
                assetLabel: 'cover.jpg',
                objectLabel: 'cover.jpg / 商业摄影原片库',
                suggestedAction: 'RETRY',
                suggestedActionLabel: '重试',
                suggestion: '建议先重试当前任务。',
                detail: '目标端点返回权限不足。',
                createdAt: '2026-04-12T12:00:00Z',
                updatedAt: '2026-04-12T12:00:30Z',
                source: {
                  taskId: 'job-1',
                  taskTitle: '封面图同步',
                  taskItemId: 'item-1',
                  taskItemTitle: '同步封面图',
                  assetId: 'asset-1',
                  entryId: 'asset-1',
                  endpointId: 'mount-1',
                  endpointLabel: '商业摄影原片库',
                  path: 'D:\\Assets\\Photo\\cover.jpg',
                  sourceLabel: '商业摄影原片库',
                  routeLabel: '商业摄影原片库',
                },
                impact: {
                  assetCount: 1,
                  replicaCount: 1,
                  directoryCount: 1,
                  endpointCount: 1,
                  blocksStatusCommit: true,
                  blocksTaskExecution: true,
                },
                capabilities: {
                  canRetry: true,
                  canConfirm: true,
                canIgnore: true,
                canArchive: false,
                  canClearHistory: false,
                  canOpenTaskCenter: true,
                  canOpenFileCenter: true,
                  canOpenStorageNodes: true,
                },
                histories: [
                  {
                    id: 'evt-1',
                    issueId: 'issue-1',
                    action: '自动发现',
                    operatorLabel: '系统',
                    result: '已基于真实任务失败创建异常。',
                    createdAt: '2026-04-12T12:00:00Z',
                  },
                ],
              },
            ],
            total: 1,
            page: 1,
            pageSize: 20,
          },
        }),
      }),
    );

    const result = await issuesApi.list();

    expect(result.total).toBe(1);
    expect(result.items[0]?.category).toBe('传输');
    expect(result.items[0]?.sourceDomain).toBe('传输任务');
    expect(result.items[0]?.status).toBe('待处理');
    expect(result.items[0]?.severity).toBe('critical');
    expect(result.items[0]?.impact.folderCount).toBe(1);
    expect(result.items[0]?.source.endpointId).toBe('mount-1');
    expect(result.items[0]?.histories[0]?.action).toBe('自动发现');
  });

  it('按任务批量查询异常摘要', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          data: {
            items: [
              {
                id: 'issue-1',
                code: 'issue-1',
                issueCategory: 'SCAN_PARSE',
                issueType: 'SCAN_DIRECTORY_FAILED',
                nature: 'BLOCKING',
                sourceDomain: 'STORAGE_DOMAIN',
                severity: 'WARNING',
                status: 'OPEN',
                title: '挂载扫描失败',
                summary: '远端目录读取超时',
                objectLabel: '商业摄影原片库',
                createdAt: '2026-04-12T12:00:00Z',
                updatedAt: '2026-04-12T12:00:30Z',
                source: {
                  taskId: 'job-1',
                  endpointId: 'mount-1',
                },
                impact: {
                  assetCount: 0,
                  replicaCount: 0,
                  directoryCount: 0,
                  endpointCount: 1,
                  blocksStatusCommit: true,
                  blocksTaskExecution: true,
                },
                capabilities: {},
                histories: [],
              },
            ],
          },
        }),
      }),
    );

    const result = await issuesApi.listByJobIds(['job-1']);

    expect(result).toHaveLength(1);
    expect(result[0]?.source.taskId).toBe('job-1');
    expect(result[0]?.sourceDomain).toBe('存储节点');
  });
});
