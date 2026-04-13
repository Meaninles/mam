import { cleanup, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createInitialState } from '../lib/clientState';
import { jobsApi, type JobDetail, type JobPriority, type JobRecord, type JobStatus } from '../lib/jobsApi';
import { TaskCenterWorkspace } from './TaskCenterWorkspace';

vi.mock('../lib/jobsApi', () => ({
  jobsApi: {
    list: vi.fn(),
    detail: vi.fn(),
    pause: vi.fn(),
    resume: vi.fn(),
    cancel: vi.fn(),
    retry: vi.fn(),
    updatePriority: vi.fn(),
    pauseItem: vi.fn(),
    resumeItem: vi.fn(),
    cancelItem: vi.fn(),
    subscribe: vi.fn(() => () => {}),
  },
}));

describe('任务中心其它任务子页面', () => {
  const seed = createInitialState();

  beforeEach(() => {
    const jobs: JobRecord[] = [
      {
        id: 'job-scan-1',
        code: 'JOB-SCAN-001',
        jobFamily: 'MAINTENANCE',
        jobIntent: 'SCAN_DIRECTORY',
        status: 'RUNNING' satisfies JobStatus,
        priority: 'NORMAL' satisfies JobPriority,
        title: '影像 NAS 挂载目录全量扫描',
        summary: '正在扫描挂载目录',
        sourceDomain: 'STORAGE_NODES',
        progressPercent: 48,
        totalItems: 1,
        successItems: 0,
        failedItems: 0,
        skippedItems: 0,
        issueCount: 0,
        createdByType: 'USER',
        createdAt: '2026-04-13T01:00:00Z',
        updatedAt: '2026-04-13T01:02:00Z',
      },
      {
        id: 'job-scan-2',
        code: 'JOB-SCAN-002',
        jobFamily: 'MAINTENANCE',
        jobIntent: 'SCAN_DIRECTORY',
        status: 'PAUSED' satisfies JobStatus,
        priority: 'NORMAL' satisfies JobPriority,
        title: '家庭照片增量扫描',
        summary: '等待继续扫描',
        sourceDomain: 'STORAGE_NODES',
        progressPercent: 30,
        totalItems: 1,
        successItems: 0,
        failedItems: 0,
        skippedItems: 0,
        issueCount: 0,
        createdByType: 'USER',
        createdAt: '2026-04-13T02:00:00Z',
        updatedAt: '2026-04-13T02:10:00Z',
      },
      {
        id: 'job-meta-1',
        code: 'JOB-META-001',
        jobFamily: 'MAINTENANCE',
        jobIntent: 'EXTRACT_METADATA',
        status: 'RUNNING' satisfies JobStatus,
        priority: 'HIGH' satisfies JobPriority,
        title: '上海发布会素材元数据解析',
        summary: '正在解析元数据',
        sourceDomain: 'FILE_CENTER',
        progressPercent: 67,
        totalItems: 1,
        successItems: 0,
        failedItems: 0,
        skippedItems: 0,
        issueCount: 0,
        createdByType: 'USER',
        createdAt: '2026-04-13T03:00:00Z',
        updatedAt: '2026-04-13T03:03:00Z',
      },
      {
        id: 'job-delete-1',
        code: 'JOB-DELETE-001',
        jobFamily: 'MAINTENANCE',
        jobIntent: 'DELETE_ASSET',
        status: 'RUNNING' satisfies JobStatus,
        priority: 'NORMAL' satisfies JobPriority,
        title: '历史交付包后台清理',
        summary: '正在清理历史交付包',
        sourceDomain: 'SYSTEM_POLICY',
        progressPercent: 15,
        totalItems: 1,
        successItems: 0,
        failedItems: 0,
        skippedItems: 0,
        issueCount: 0,
        createdByType: 'SYSTEM',
        createdAt: '2026-04-13T04:00:00Z',
        updatedAt: '2026-04-13T04:05:00Z',
      },
    ];

    vi.mocked(jobsApi.list).mockResolvedValue({
      items: jobs,
      total: jobs.length,
      page: 1,
      pageSize: 100,
    });

    vi.mocked(jobsApi.detail).mockImplementation(async (id: string): Promise<JobDetail> => {
      const job = jobs.find((item) => item.id === id);
      if (!job) {
        throw new Error(`missing job ${id}`);
      }

      const itemTitleByJob: Record<string, string> = {
        'job-scan-1': '扫描挂载 A',
        'job-scan-2': '扫描家庭图库',
        'job-meta-1': '解析素材元数据',
        'job-delete-1': '清理历史交付包',
      };

      return {
        job,
        items: [
          {
            id: `item-${id}`,
            jobId: id,
            itemKey: `item:${id}`,
            itemType: job.jobIntent === 'SCAN_DIRECTORY' ? 'DIRECTORY_SCAN' : 'ASSET_METADATA_EXTRACT',
            status: job.status,
            phase: job.status === 'PAUSED' ? 'PAUSED' : 'EXECUTING',
            title: itemTitleByJob[id],
            summary: job.summary,
            sourcePath: id === 'job-scan-1' ? 'D:\\Assets\\NAS-01' : id === 'job-scan-2' ? 'D:\\Family' : 'D:\\Archive',
            progressPercent: job.progressPercent,
            attemptCount: 1,
            issueCount: 0,
            updatedAt: job.updatedAt,
            createdAt: job.createdAt,
          },
        ],
        links: [],
      };
    });

    vi.mocked(jobsApi.pause).mockResolvedValue({ message: '已暂停', job: {} as never });
    vi.mocked(jobsApi.resume).mockResolvedValue({ message: '已继续', job: {} as never });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('支持按其它任务类型与关键字筛选当前真实作业数据', async () => {
    const user = userEvent.setup();

    render(
      <TaskCenterWorkspace
        activeTab="other"
        visible
        fileNodes={seed.fileNodes}
        issues={[]}
        libraries={[{ id: 'photo', name: '商业摄影资产库', rootLabel: '/', itemCount: '0', health: '100%', storagePolicy: '本地' }]}
        preselectedTaskIds={null}
        statusFilter="全部"
        onConsumePreselectedTaskIds={() => {}}
        onFeedback={() => {}}
        onOpenFileCenterForTask={() => {}}
        onOpenIssueCenterForIssue={() => {}}
        onOpenIssueCenterForTask={() => {}}
        onOpenStorageNodesForTask={() => {}}
        onSetActiveTab={() => {}}
        onSetTaskStatusFilter={() => {}}
      />,
    );

    expect(await screen.findByRole('button', { name: '全部' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '扫描' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '元数据解析' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '删除清理' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '扫描' }));
    expect(screen.getByText('影像 NAS 挂载目录全量扫描')).toBeInTheDocument();
    expect(screen.getByText('家庭照片增量扫描')).toBeInTheDocument();
    expect(screen.queryByText('上海发布会素材元数据解析')).not.toBeInTheDocument();

    await user.type(screen.getByLabelText('搜索任务'), '家庭');
    expect(screen.getByText('家庭照片增量扫描')).toBeInTheDocument();
    expect(screen.queryByText('影像 NAS 挂载目录全量扫描')).not.toBeInTheDocument();
  });

  it('支持查看扫描任务详情并跳转到存储节点页', async () => {
    const user = userEvent.setup();
    const openStorage = vi.fn();

    render(
      <TaskCenterWorkspace
        activeTab="other"
        visible
        fileNodes={seed.fileNodes}
        issues={[]}
        libraries={[{ id: 'photo', name: '商业摄影资产库', rootLabel: '/', itemCount: '0', health: '100%', storagePolicy: '本地' }]}
        preselectedTaskIds={null}
        statusFilter="全部"
        onConsumePreselectedTaskIds={() => {}}
        onFeedback={() => {}}
        onOpenFileCenterForTask={() => {}}
        onOpenIssueCenterForIssue={() => {}}
        onOpenIssueCenterForTask={() => {}}
        onOpenStorageNodesForTask={openStorage}
        onSetActiveTab={() => {}}
        onSetTaskStatusFilter={() => {}}
      />,
    );

    const row = (await screen.findByText('影像 NAS 挂载目录全量扫描')).closest('article');
    expect(row).not.toBeNull();
    await user.click(within(row as HTMLElement).getByRole('button', { name: '详情' }));

    const detailSheet = await screen.findByRole('region', { name: '影像 NAS 挂载目录全量扫描' });
    expect(within(detailSheet).getByText('扫描')).toBeInTheDocument();

    await user.click(within(detailSheet).getByRole('button', { name: '查看存储节点' }));
    expect(openStorage).toHaveBeenCalledTimes(1);
  });

  it('已暂停的其它任务显示继续按钮，运行中的删除清理任务显示暂停按钮', async () => {
    render(
      <TaskCenterWorkspace
        activeTab="other"
        visible
        fileNodes={seed.fileNodes}
        issues={[]}
        libraries={[{ id: 'photo', name: '商业摄影资产库', rootLabel: '/', itemCount: '0', health: '100%', storagePolicy: '本地' }]}
        preselectedTaskIds={null}
        statusFilter="全部"
        onConsumePreselectedTaskIds={() => {}}
        onFeedback={() => {}}
        onOpenFileCenterForTask={() => {}}
        onOpenIssueCenterForIssue={() => {}}
        onOpenIssueCenterForTask={() => {}}
        onOpenStorageNodesForTask={() => {}}
        onSetActiveTab={() => {}}
        onSetTaskStatusFilter={() => {}}
      />,
    );

    const pausedRow = (await screen.findByText('家庭照片增量扫描')).closest('article');
    expect(pausedRow).not.toBeNull();
    expect(within(pausedRow as HTMLElement).getByRole('button', { name: '继续' })).toBeInTheDocument();
    expect(within(pausedRow as HTMLElement).queryByRole('button', { name: '暂停' })).not.toBeInTheDocument();

    const cleanupRow = screen.getByText('历史交付包后台清理').closest('article');
    expect(cleanupRow).not.toBeNull();
    expect(within(cleanupRow as HTMLElement).getByRole('button', { name: '暂停' })).toBeInTheDocument();
  });
});
