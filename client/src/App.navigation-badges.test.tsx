import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import App from './App';
import { resetFileCenterMock } from './lib/fileCenterApi';
import { issuesApi } from './lib/issuesApi';
import { jobsApi, type JobRecord } from './lib/jobsApi';
import type { IssueRecord } from './data';

describe('App navigation badges', () => {
  beforeEach(async () => {
    window.localStorage.clear();
    await resetFileCenterMock();
  });

  afterEach(async () => {
    vi.restoreAllMocks();
    cleanup();
    await resetFileCenterMock();
  });

  it('shows active task and pending issue counts while hiding the storage nodes badge', async () => {
    const jobs: JobRecord[] = [
      createJob('job-running-1', 'RUNNING'),
      createJob('job-running-2', 'RUNNING'),
      createJob('job-paused', 'PAUSED'),
      createJob('job-waiting-confirmation', 'WAITING_CONFIRMATION'),
      createJob('job-completed', 'COMPLETED'),
      createJob('job-failed', 'FAILED'),
    ];
    const issues: IssueRecord[] = [
      createIssue('issue-pending-1', '待处理'),
      createIssue('issue-pending-2', '待处理'),
      createIssue('issue-processing', '处理中'),
    ];

    vi.spyOn(jobsApi, 'list').mockResolvedValue({
      items: jobs,
      total: jobs.length,
      page: 1,
      pageSize: 100,
    });
    vi.spyOn(jobsApi, 'subscribe').mockReturnValue(() => {});
    vi.spyOn(issuesApi, 'listAll').mockResolvedValue(issues);

    render(<App />);

    const taskCenterButton = screen.getByRole('button', { name: '任务中心' });
    const issueCenterButton = screen.getByRole('button', { name: '异常中心' });
    const storageNodesButton = screen.getByRole('button', { name: '存储节点' });

    await waitFor(() => {
      expect(within(taskCenterButton).getByText('4')).toBeInTheDocument();
      expect(within(issueCenterButton).getByText('2')).toBeInTheDocument();
    });

    expect(within(storageNodesButton).queryByText(/^\d+$/)).not.toBeInTheDocument();
  });
});

function createJob(id: string, status: JobRecord['status']): JobRecord {
  return {
    id,
    code: id,
    libraryId: 'photo',
    jobFamily: 'TRANSFER',
    jobIntent: 'SYNC',
    routeType: 'COPY',
    status,
    priority: 'NORMAL',
    title: id,
    summary: id,
    sourceDomain: 'FILE_CENTER',
    progressPercent: 0,
    totalItems: 1,
    successItems: 0,
    failedItems: 0,
    skippedItems: 0,
    issueCount: 0,
    createdByType: 'USER',
    createdAt: '2026-04-12T00:00:00Z',
    updatedAt: '2026-04-12T00:00:00Z',
  };
}

function createIssue(id: string, status: IssueRecord['status']): IssueRecord {
  return {
    id,
    libraryId: 'photo',
    category: '传输',
    type: '校验失败',
    nature: 'BLOCKING',
    sourceDomain: '传输任务',
    severity: 'critical',
    title: id,
    summary: id,
    asset: id,
    objectLabel: id,
    action: '重试',
    actionLabel: '重试',
    suggestion: '重试',
    detail: id,
    status,
    createdAt: '今天 10:00',
    updatedAt: '今天 10:00',
    source: {
      sourceDomain: '传输任务',
      sourceLabel: id,
    },
    impact: {
      assetCount: 1,
      replicaCount: 1,
      folderCount: 0,
      endpointCount: 1,
      blocksStatusCommit: true,
      blocksTaskExecution: true,
    },
    capabilities: {
      canRetry: true,
    },
    histories: [],
  };
}
