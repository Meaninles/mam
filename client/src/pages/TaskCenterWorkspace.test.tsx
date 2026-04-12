import { cleanup, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createInitialState } from '../lib/clientState';
import { jobsApi } from '../lib/jobsApi';
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

describe('TaskCenterWorkspace', () => {
  const seed = createInitialState();

  beforeEach(() => {
    vi.mocked(jobsApi.list).mockResolvedValue({
      items: [
        {
          id: 'job-scan-1',
          code: 'JOB-001',
          jobFamily: 'MAINTENANCE',
          jobIntent: 'SCAN_DIRECTORY',
          status: 'RUNNING',
          priority: 'NORMAL',
          title: '扫描目录：素材库',
          summary: '正在扫描挂载目录',
          sourceDomain: 'STORAGE_NODES',
          progressPercent: 48,
          totalItems: 2,
          successItems: 0,
          failedItems: 0,
          skippedItems: 0,
          issueCount: 0,
          createdByType: 'USER',
          createdAt: '2026-04-11T08:00:00Z',
          updatedAt: '2026-04-11T08:02:00Z',
        },
      ],
      total: 1,
      page: 1,
      pageSize: 100,
    });
    vi.mocked(jobsApi.detail).mockResolvedValue({
      job: {
        id: 'job-scan-1',
        code: 'JOB-001',
        jobFamily: 'MAINTENANCE',
        jobIntent: 'SCAN_DIRECTORY',
        status: 'RUNNING',
        priority: 'NORMAL',
        title: '扫描目录：素材库',
        summary: '正在扫描挂载目录',
        sourceDomain: 'STORAGE_NODES',
        progressPercent: 48,
        totalItems: 2,
        successItems: 0,
        failedItems: 0,
        skippedItems: 0,
        issueCount: 0,
        createdByType: 'USER',
        createdAt: '2026-04-11T08:00:00Z',
        updatedAt: '2026-04-11T08:02:00Z',
      },
      items: [
        {
          id: 'job-item-1',
          jobId: 'job-scan-1',
          itemKey: 'mount:a',
          itemType: 'DIRECTORY_SCAN',
          status: 'RUNNING',
          phase: 'EXECUTING',
          title: '扫描挂载 A',
          summary: '正在扫描挂载 A',
          sourcePath: 'D:\\Assets\\A',
          progressPercent: 48,
          attemptCount: 1,
          issueCount: 0,
          updatedAt: '2026-04-11T08:02:00Z',
          createdAt: '2026-04-11T08:00:00Z',
        },
        {
          id: 'job-item-2',
          jobId: 'job-scan-1',
          itemKey: 'mount:b',
          itemType: 'DIRECTORY_SCAN',
          status: 'PENDING',
          title: '扫描挂载 B',
          summary: '等待扫描挂载 B',
          sourcePath: 'D:\\Assets\\B',
          progressPercent: 0,
          attemptCount: 0,
          issueCount: 0,
          updatedAt: '2026-04-11T08:02:00Z',
          createdAt: '2026-04-11T08:00:00Z',
        },
      ],
      links: [],
    });
    vi.mocked(jobsApi.pauseItem).mockResolvedValue({
      message: '子任务已暂停',
      job: {
        id: 'job-scan-1',
        code: 'JOB-001',
        jobFamily: 'MAINTENANCE',
        jobIntent: 'SCAN_DIRECTORY',
        status: 'PAUSED',
        priority: 'NORMAL',
        title: '扫描目录：素材库',
        summary: '正在扫描挂载目录',
        sourceDomain: 'STORAGE_NODES',
        progressPercent: 48,
        totalItems: 2,
        successItems: 0,
        failedItems: 0,
        skippedItems: 0,
        issueCount: 0,
        createdByType: 'USER',
        createdAt: '2026-04-11T08:00:00Z',
        updatedAt: '2026-04-11T08:02:30Z',
      },
      item: {
        id: 'job-item-1',
        jobId: 'job-scan-1',
        itemKey: 'mount:a',
        itemType: 'DIRECTORY_SCAN',
        status: 'PAUSED',
        title: '扫描挂载 A',
        summary: '正在扫描挂载 A',
        progressPercent: 48,
        attemptCount: 1,
        issueCount: 0,
        updatedAt: '2026-04-11T08:02:30Z',
        createdAt: '2026-04-11T08:00:00Z',
      },
    });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('使用真实作业详情渲染其它任务并支持子任务暂停', async () => {
    const user = userEvent.setup();

    render(
      <TaskCenterWorkspace
        activeTab="other"
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

    const rowTitle = await screen.findByText('扫描目录：素材库');
    const row = rowTitle.closest('article');
    expect(row).not.toBeNull();

    await user.click(within(row as HTMLElement).getByRole('button', { name: '详情' }));

    const detailSheet = await screen.findByRole('region', { name: '扫描目录：素材库' });
    const childRow = within(detailSheet).getByText('扫描挂载 A').closest('.other-task-detail-row');
    expect(childRow).not.toBeNull();

    await user.click(within(childRow as HTMLElement).getAllByRole('button')[0]);

    expect(jobsApi.pauseItem).toHaveBeenCalledWith('job-item-1');
  });

  it('传输任务详情会展示 CloudDrive2 外部执行器信息，并在列表中显示更明确阶段', async () => {
    const user = userEvent.setup();

    vi.mocked(jobsApi.list).mockResolvedValueOnce({
      items: [
        {
          id: 'job-transfer-upload-1',
          code: 'JOB-UPLOAD-001',
          libraryId: 'photo',
          jobFamily: 'TRANSFER',
          jobIntent: 'REPLICATE',
          routeType: 'UPLOAD',
          status: 'RUNNING',
          priority: 'HIGH',
          title: '同步到 115：上海发布会精选',
          summary: '正在上传到 115',
          sourceDomain: 'FILE_CENTER',
          progressPercent: 62,
          speedBps: 31457280,
          etaSeconds: 96,
          totalItems: 1,
          successItems: 0,
          failedItems: 0,
          skippedItems: 0,
          issueCount: 0,
          createdByType: 'USER',
          createdAt: '2026-04-12T12:00:00Z',
          updatedAt: '2026-04-12T12:03:00Z',
        },
      ],
      total: 1,
      page: 1,
      pageSize: 100,
    });

    vi.mocked(jobsApi.detail).mockResolvedValueOnce({
      job: {
        id: 'job-transfer-upload-1',
        code: 'JOB-UPLOAD-001',
        libraryId: 'photo',
        jobFamily: 'TRANSFER',
        jobIntent: 'REPLICATE',
        routeType: 'UPLOAD',
        status: 'RUNNING',
        priority: 'HIGH',
        title: '同步到 115：上海发布会精选',
        summary: '正在上传到 115',
        sourceDomain: 'FILE_CENTER',
        progressPercent: 62,
        speedBps: 31457280,
        etaSeconds: 96,
        totalItems: 1,
        successItems: 0,
        failedItems: 0,
        skippedItems: 0,
        issueCount: 0,
        createdByType: 'USER',
        createdAt: '2026-04-12T12:00:00Z',
        updatedAt: '2026-04-12T12:03:00Z',
      },
      items: [
        {
          id: 'job-item-upload-1',
          jobId: 'job-transfer-upload-1',
          itemKey: 'asset:cover',
          itemType: 'ASSET_TRANSFER',
          status: 'RUNNING',
          title: '上海发布会_精选封面.jpg',
          summary: '上传到 115',
          sourcePath: 'D:\\Mare\\Assets\\PhotoRaw\\上海发布会_精选封面.jpg',
          targetPath: '/MareArchive/上海发布会_精选封面.jpg',
          progressPercent: 62,
          speedBps: 31457280,
          etaSeconds: 96,
          bytesTotal: 104857600,
          bytesDone: 65011712,
          externalTaskEngine: 'CD2_REMOTE_UPLOAD',
          externalTaskId: 'upload-115-001',
          externalTaskStatus: 'Transfer',
          attemptCount: 1,
          issueCount: 0,
          updatedAt: '2026-04-12T12:03:00Z',
          createdAt: '2026-04-12T12:00:00Z',
        },
      ],
      links: [],
    });

    render(
      <TaskCenterWorkspace
        activeTab="transfer"
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

    expect(await screen.findByText('当前阶段：CloudDrive2 上传中')).toBeInTheDocument();

    const row = screen.getByText('上海发布会_精选封面.jpg').closest('article');
    expect(row).not.toBeNull();
    await user.click(within(row as HTMLElement).getByRole('button', { name: '详情' }));

    expect(await screen.findByText('执行引擎')).toBeInTheDocument();
    expect(screen.getByText('CloudDrive2 上传')).toBeInTheDocument();
    expect(screen.getByText('upload-115-001')).toBeInTheDocument();
    expect(screen.getByText(/Transfer/)).toBeInTheDocument();
  });

  it('失败的 aria2 下载任务会展示失败位置、外部状态和原始错误', async () => {
    const user = userEvent.setup();

    vi.mocked(jobsApi.list).mockResolvedValueOnce({
      items: [
        {
          id: 'job-transfer-download-1',
          code: 'JOB-DOWNLOAD-001',
          libraryId: 'photo',
          jobFamily: 'TRANSFER',
          jobIntent: 'REPLICATE',
          routeType: 'DOWNLOAD',
          status: 'FAILED',
          priority: 'NORMAL',
          title: '从 115 同步：发布会备份',
          summary: '下载到本地失败',
          sourceDomain: 'FILE_CENTER',
          progressPercent: 38,
          speedBps: 0,
          etaSeconds: 0,
          totalItems: 1,
          successItems: 0,
          failedItems: 1,
          skippedItems: 0,
          issueCount: 1,
          latestErrorCode: 'aria2_timeout',
          latestErrorMessage: 'aria2 rpc timeout',
          createdByType: 'USER',
          createdAt: '2026-04-12T12:00:00Z',
          updatedAt: '2026-04-12T12:05:00Z',
        },
      ],
      total: 1,
      page: 1,
      pageSize: 100,
    });

    vi.mocked(jobsApi.detail).mockResolvedValueOnce({
      job: {
        id: 'job-transfer-download-1',
        code: 'JOB-DOWNLOAD-001',
        libraryId: 'photo',
        jobFamily: 'TRANSFER',
        jobIntent: 'REPLICATE',
        routeType: 'DOWNLOAD',
        status: 'FAILED',
        priority: 'NORMAL',
        title: '从 115 同步：发布会备份',
        summary: '下载到本地失败',
        sourceDomain: 'FILE_CENTER',
        progressPercent: 38,
        speedBps: 0,
        etaSeconds: 0,
        totalItems: 1,
        successItems: 0,
        failedItems: 1,
        skippedItems: 0,
        issueCount: 1,
        latestErrorCode: 'aria2_timeout',
        latestErrorMessage: 'aria2 rpc timeout',
        createdByType: 'USER',
        createdAt: '2026-04-12T12:00:00Z',
        updatedAt: '2026-04-12T12:05:00Z',
      },
      items: [
        {
          id: 'job-item-download-1',
          jobId: 'job-transfer-download-1',
          itemKey: 'asset:backup',
          itemType: 'ASSET_TRANSFER',
          status: 'FAILED',
          title: '上海发布会_A-cam_001.RAW',
          summary: '从 115 下载到本地',
          sourcePath: '/MareArchive/上海发布会_A-cam_001.RAW',
          targetPath: 'D:\\Mare\\Assets\\PhotoRaw\\上海发布会_A-cam_001.RAW',
          progressPercent: 38,
          speedBps: 0,
          etaSeconds: 0,
          bytesTotal: 104857600,
          bytesDone: 39845888,
          externalTaskEngine: 'ARIA2',
          externalTaskId: 'aria2-gid-001',
          externalTaskStatus: 'error',
          attemptCount: 1,
          issueCount: 1,
          latestErrorCode: 'aria2_timeout',
          latestErrorMessage: 'aria2 rpc timeout',
          updatedAt: '2026-04-12T12:05:00Z',
          createdAt: '2026-04-12T12:00:00Z',
        },
      ],
      links: [],
    });

    render(
      <TaskCenterWorkspace
        activeTab="transfer"
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

    const row = await screen.findByText('上海发布会_A-cam_001.RAW');
    expect(row).toBeInTheDocument();
    expect(screen.getByText('当前阶段：aria2 下载失败')).toBeInTheDocument();

    await user.click(within(row.closest('article') as HTMLElement).getByRole('button', { name: '详情' }));

    expect(await screen.findByText('执行引擎')).toBeInTheDocument();
    expect(screen.getByText('aria2 下载')).toBeInTheDocument();
    expect(screen.getByText('aria2-gid-001')).toBeInTheDocument();
    expect(screen.getByText('aria2 下载器')).toBeInTheDocument();
    expect(screen.getByText('aria2 rpc timeout')).toBeInTheDocument();
  });

  it('CloudDrive2 缓存阶段不会被误判为上传成功', async () => {
    vi.mocked(jobsApi.list).mockResolvedValueOnce({
      items: [
        {
          id: 'job-transfer-upload-cache-1',
          code: 'JOB-UPLOAD-002',
          libraryId: 'photo',
          jobFamily: 'TRANSFER',
          jobIntent: 'REPLICATE',
          routeType: 'UPLOAD',
          status: 'RUNNING',
          priority: 'NORMAL',
          title: '同步到 115：缓存阶段',
          summary: '文件已进入缓存，等待云端上传',
          sourceDomain: 'FILE_CENTER',
          progressPercent: 12,
          speedBps: 0,
          etaSeconds: 0,
          totalItems: 1,
          successItems: 0,
          failedItems: 0,
          skippedItems: 0,
          issueCount: 0,
          createdByType: 'USER',
          createdAt: '2026-04-12T12:00:00Z',
          updatedAt: '2026-04-12T12:01:00Z',
        },
      ],
      total: 1,
      page: 1,
      pageSize: 100,
    });

    vi.mocked(jobsApi.detail).mockResolvedValueOnce({
      job: {
        id: 'job-transfer-upload-cache-1',
        code: 'JOB-UPLOAD-002',
        libraryId: 'photo',
        jobFamily: 'TRANSFER',
        jobIntent: 'REPLICATE',
        routeType: 'UPLOAD',
        status: 'RUNNING',
        priority: 'NORMAL',
        title: '同步到 115：缓存阶段',
        summary: '文件已进入缓存，等待云端上传',
        sourceDomain: 'FILE_CENTER',
        progressPercent: 12,
        speedBps: 0,
        etaSeconds: 0,
        totalItems: 1,
        successItems: 0,
        failedItems: 0,
        skippedItems: 0,
        issueCount: 0,
        createdByType: 'USER',
        createdAt: '2026-04-12T12:00:00Z',
        updatedAt: '2026-04-12T12:01:00Z',
      },
      items: [
        {
          id: 'job-item-upload-cache-1',
          jobId: 'job-transfer-upload-cache-1',
          itemKey: 'asset:cache-stage',
          itemType: 'ASSET_TRANSFER',
          status: 'RUNNING',
          title: '缓存阶段素材.mov',
          summary: '文件已进入 CD2 缓存',
          sourcePath: 'D:\\Mare\\Assets\\cache-stage.mov',
          targetPath: '/MareArchive/cache-stage.mov',
          progressPercent: 12,
          externalTaskEngine: 'CD2_REMOTE_UPLOAD',
          externalTaskId: 'upload-115-cache-1',
          externalTaskStatus: 'Inqueue',
          attemptCount: 1,
          issueCount: 0,
          updatedAt: '2026-04-12T12:01:00Z',
          createdAt: '2026-04-12T12:00:00Z',
        },
      ],
      links: [],
    });

    render(
      <TaskCenterWorkspace
        activeTab="transfer"
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

    const row = (await screen.findByText('缓存阶段素材.mov')).closest('article');
    expect(row).not.toBeNull();
    expect(within(row as HTMLElement).getByText('当前阶段：缓存写入完成，等待云端上传')).toBeInTheDocument();
    expect(within(row as HTMLElement).queryByText('已完成')).not.toBeInTheDocument();
  });
});
