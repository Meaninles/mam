import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import App from '../App';
import { resetFileCenterMock } from '../lib/fileCenterApi';

describe('任务中心其它任务子页面', () => {
  beforeEach(async () => {
    window.localStorage.clear();
    await resetFileCenterMock();
  });

  afterEach(async () => {
    cleanup();
    await resetFileCenterMock();
  });

  it('支持按四类任务统一筛选，并显示搜索、资产库与排序能力', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '其它任务' }));

    expect(screen.getByRole('button', { name: '全部' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '扫描' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '元数据解析' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '校验' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '删除清理' })).toBeInTheDocument();
    expect(screen.getByLabelText('任务状态')).toBeInTheDocument();
    expect(screen.queryByLabelText('资产库')).not.toBeInTheDocument();
    expect(screen.getByLabelText('排序方式')).toBeInTheDocument();
    expect(screen.getByLabelText('搜索任务')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /展开 / })).not.toBeInTheDocument();
  });

  it('支持按任务类型和关键字筛选其它任务', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '其它任务' }));
    await user.selectOptions(screen.getByLabelText('任务状态'), '全部');

    await user.click(screen.getByRole('button', { name: '扫描' }));
    expect(screen.getByText('影像 NAS 挂载目录全量扫描')).toBeInTheDocument();
    expect(screen.queryByText('上海发布会素材元数据解析')).not.toBeInTheDocument();

    await user.type(screen.getByLabelText('搜索任务'), '家庭');
    expect(screen.getByText('家庭照片增量扫描')).toBeInTheDocument();
    expect(screen.queryByText('影像 NAS 挂载目录全量扫描')).not.toBeInTheDocument();
  });

  it('支持查看扫描任务详情并跳转到存储节点页', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '其它任务' }));
    await user.selectOptions(screen.getByLabelText('任务状态'), '全部');

    const row = screen.getByText('影像 NAS 挂载目录全量扫描').closest('article');
    expect(row).not.toBeNull();
    await user.click(within(row as HTMLElement).getByRole('button', { name: '详情' }));

    const detailSheet = await screen.findByRole('region', { name: '影像 NAS 挂载目录全量扫描' });
    expect(within(detailSheet).getByText('扫描')).toBeInTheDocument();
    expect(within(detailSheet).getAllByText('归并提交').length).toBeGreaterThan(0);
    expect(within(detailSheet).getByRole('button', { name: '查看存储节点' })).toBeInTheDocument();

    await user.click(within(detailSheet).getByRole('button', { name: '查看存储节点' }));
    expect(await screen.findByRole('button', { name: '挂载文件夹管理' })).toBeInTheDocument();
  });

  it('支持其它任务的批量暂停、批量继续，以及删除清理的等待清理状态展示', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '其它任务' }));
    await user.selectOptions(screen.getByLabelText('任务状态'), '全部');

    await user.click(screen.getByLabelText('选择 影像 NAS 挂载目录全量扫描'));
    await user.click(screen.getByLabelText('选择 上海发布会素材元数据解析'));
    expect(screen.getByText('已选择 2 个任务')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '批量暂停' }));
    await waitFor(() => {
      expect(screen.getAllByText('已暂停').length).toBeGreaterThanOrEqual(2);
    });

    await user.click(screen.getByRole('button', { name: '批量继续' }));
    await waitFor(() => {
      expect(screen.getAllByText('运行中').length).toBeGreaterThanOrEqual(2);
    });

    await user.click(screen.getByRole('button', { name: '删除清理' }));
    const deleteRow = screen.getByText('删除资产：2026-03-29_上海发布会_A-cam_001.RAW').closest('article');
    expect(deleteRow).not.toBeNull();
    expect(within(deleteRow as HTMLElement).getByText('等待清理')).toBeInTheDocument();
  });

  it('其它任务中的已暂停任务会显示继续按钮', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '其它任务' }));
    await user.selectOptions(screen.getByLabelText('任务状态'), '全部');

    const row = screen.getByText('家庭照片增量扫描').closest('article');
    expect(row).not.toBeNull();
    expect(within(row as HTMLElement).getByRole('button', { name: '继续' })).toBeInTheDocument();
    expect(within(row as HTMLElement).queryByRole('button', { name: '暂停' })).not.toBeInTheDocument();
  });

  it('删除清理任务在运行中和等待清理时都支持暂停', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '其它任务' }));
    await user.selectOptions(screen.getByLabelText('任务状态'), '全部');

    const runningCleanupRow = screen.getByText('历史交付包后台清理').closest('article');
    expect(runningCleanupRow).not.toBeNull();
    expect(within(runningCleanupRow as HTMLElement).getByRole('button', { name: '暂停' })).toBeInTheDocument();

    const waitingCleanupRow = screen.getByText('删除资产：2026-03-29_上海发布会_A-cam_001.RAW').closest('article');
    expect(waitingCleanupRow).not.toBeNull();
    expect(within(waitingCleanupRow as HTMLElement).getByRole('button', { name: '暂停' })).toBeInTheDocument();

    await user.click(within(waitingCleanupRow as HTMLElement).getByRole('button', { name: '暂停' }));
    await waitFor(() => {
      expect(within(waitingCleanupRow as HTMLElement).getByRole('button', { name: '继续' })).toBeInTheDocument();
    });
  });
});
