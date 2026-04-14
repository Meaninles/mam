import { act, cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import App from './App';
import { fileCenterApi, resetFileCenterMock, type FileCenterDirectoryResult } from './lib/fileCenterApi';
import { jobsApi } from './lib/jobsApi';

describe('App 集成流程', () => {
  beforeEach(async () => {
    window.localStorage.clear();
    await resetFileCenterMock();
    vi.spyOn(jobsApi, 'list').mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 100 });
    vi.spyOn(jobsApi, 'subscribe').mockReturnValue(() => {});
  });

  afterEach(async () => {
    vi.restoreAllMocks();
    cleanup();
    await resetFileCenterMock();
  });

  it('任务中心默认使用活跃中状态筛选', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));

    expect(await screen.findByRole('tab', { name: '任务中心' })).toBeInTheDocument();
    expect(await screen.findByLabelText('任务状态')).toHaveValue('活跃中');
  });

  it('页头导入入口可以打开导入中心顶层标签', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '导入' }));
    expect(await screen.findByRole('tab', { name: '导入中心' })).toBeInTheDocument();
  });

  it('文件中心在同步任务完成后会响应式刷新存储状态', async () => {
    const jobSubscribers: Array<(event: Parameters<typeof jobsApi.subscribe>[0] extends (event: infer T) => void ? T : never) => void> =
      [];
    vi.mocked(jobsApi.subscribe).mockImplementation((listener) => {
      jobSubscribers.push(listener);
      return () => {};
    });

    const syncingResult: FileCenterDirectoryResult = {
      breadcrumbs: [{ id: null, label: '商业摄影资产库' }],
      items: [
        {
          id: 'asset-cover',
          libraryId: 'photo',
          parentId: null,
          type: 'file',
          lifecycleState: 'ACTIVE',
          name: 'cover.jpg',
          fileKind: '图片',
          displayType: 'JPG 图片',
          modifiedAt: '今天 09:30',
          createdAt: '2026-04-12 09:30',
          size: '12 MB',
          path: '商业摄影资产库 / cover.jpg',
          sourceLabel: '统一资产',
          lastTaskText: '同步进行中',
          lastTaskTone: 'warning',
          rating: 4,
          colorLabel: '红标',
          badges: ['RAW'],
          riskTags: ['待同步'],
          tags: ['封面图'],
          endpoints: [
            {
              name: '本地NVMe',
              state: '已同步',
              tone: 'success',
              lastSyncAt: '今天 09:18',
              endpointType: 'local',
            },
            {
              name: '影像NAS',
              state: '同步中',
              tone: 'warning',
              lastSyncAt: '刚刚',
              endpointType: 'nas',
            },
          ],
        },
      ],
      total: 1,
      currentPathChildren: 1,
      endpointNames: ['本地NVMe', '影像NAS'],
    };
    const syncedResult: FileCenterDirectoryResult = {
      ...syncingResult,
      items: syncingResult.items.map((item) => ({
        ...item,
        lastTaskText: '同步已完成',
        lastTaskTone: 'success',
        endpoints: item.endpoints.map((endpoint) =>
          endpoint.name === '影像NAS'
            ? {
                ...endpoint,
                state: '已同步',
                tone: 'success',
                lastSyncAt: '今天 10:08',
              }
            : endpoint,
        ),
      })),
    };

    let loadDirectoryCount = 0;
    vi.spyOn(fileCenterApi, 'scanDirectory').mockResolvedValue({ message: '扫描已提交' });
    vi.spyOn(fileCenterApi, 'loadDirectory').mockImplementation(async () => {
      loadDirectoryCount += 1;
      return loadDirectoryCount >= 3 ? syncedResult : syncingResult;
    });

    render(<App />);

    expect(await screen.findByRole('button', { name: '影像NAS 同步中' })).toBeInTheDocument();

    await act(async () => {
      jobSubscribers.forEach((listener) =>
        listener({
          eventId: 'evt-job-completed-1',
          topic: 'jobs',
          eventType: 'JOB_COMPLETED',
          jobId: 'job-sync-1',
          jobStatus: 'COMPLETED',
          message: '同步任务完成',
          createdAt: '2026-04-14T10:08:00+08:00',
        }),
      );
      await new Promise((resolve) => window.setTimeout(resolve, 300));
    });

    expect(await screen.findByRole('button', { name: '影像NAS 已同步' })).toBeInTheDocument();
  });
});
