import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import App from './App';
import { resetFileCenterMock } from './lib/fileCenterApi';
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
});
