import { cleanup, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createInitialState } from '../lib/clientState';
import { jobsApi } from '../lib/jobsApi';
import { taskExceptionsApi } from '../lib/taskExceptionsApi';
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

vi.mock('../lib/taskExceptionsApi', () => ({
  taskExceptionsApi: {
    listByJobIds: vi.fn(),
  },
}));

describe('TaskCenterWorkspace', () => {
  const seed = createInitialState();

  beforeEach(() => {
    vi.mocked(taskExceptionsApi.listByJobIds).mockResolvedValue([]);
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
        libraries={[{ id: 'photo', name: '商业摄影资产库', rootLabel: '/', itemCount: '0', health: '100%', storagePolicy: '本地' }]}
        preselectedTaskIds={null}
        statusFilter="全部"
        onConsumePreselectedTaskIds={() => {}}
        onFeedback={() => {}}
        onOpenFileCenterForTask={() => {}}
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
});
