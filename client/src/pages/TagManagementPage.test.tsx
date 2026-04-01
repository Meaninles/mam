import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Library } from '../data';
import { resetFileCenterMock } from '../lib/fileCenterApi';
import { TagManagementPage } from './TagManagementPage';

const libraries: Library[] = [
  {
    id: 'photo',
    name: '商业摄影资产库',
    rootLabel: '2026 / Shanghai Launch',
    itemCount: '46,820',
    health: '98.2%',
    storagePolicy: '本地 + NAS + 115',
  },
  {
    id: 'video',
    name: '视频工作流资产库',
    rootLabel: '2026 / Interview',
    itemCount: '28,406',
    health: '91.6%',
    storagePolicy: '本地 + NAS',
  },
  {
    id: 'family',
    name: '家庭照片资产库',
    rootLabel: 'Archive / Family',
    itemCount: '49,222',
    health: '99.4%',
    storagePolicy: '本地 + NAS + 云归档',
  },
];

describe('TagManagementPage', () => {
  beforeEach(async () => {
    window.localStorage.clear();
    await resetFileCenterMock();
  });

  afterEach(async () => {
    cleanup();
    await resetFileCenterMock();
    vi.restoreAllMocks();
  });

  it('支持创建标签并在详情面板中配置作用资产库', async () => {
    const user = userEvent.setup();
    const onFeedback = vi.fn();

    render(<TagManagementPage libraries={libraries} onFeedback={onFeedback} />);

    expect(await screen.findByText('标签总数')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '新增标签' }));
    const dialog = await screen.findByRole('dialog', { name: '新增标签' });

    await user.type(within(dialog).getByLabelText('标签名称'), '直播切片');
    await user.selectOptions(within(dialog).getByLabelText('所属分组'), 'tag-group-project');
    await user.click(within(dialog).getByLabelText('视频工作流资产库'));
    await user.click(within(dialog).getByRole('button', { name: '创建标签' }));

    expect(await screen.findByRole('button', { name: '直播切片' })).toBeInTheDocument();
    expect(onFeedback).toHaveBeenCalledWith(
      expect.objectContaining({
        message: '标签已创建',
        tone: 'success',
      }),
    );

    await user.click(screen.getByRole('button', { name: '直播切片' }));
    const detail = await screen.findByRole('region', { name: '标签详情' });
    expect(within(detail).getByDisplayValue('直播切片')).toBeInTheDocument();
    expect(within(detail).getByLabelText('视频工作流资产库')).toBeChecked();
    expect(within(detail).getByLabelText('所属分组')).toHaveValue('tag-group-project');
  });

  it('支持合并标签并展示删除确认信息', async () => {
    const user = userEvent.setup();
    const onFeedback = vi.fn();

    render(<TagManagementPage libraries={libraries} onFeedback={onFeedback} />);
    await screen.findByText('标签总数');

    await user.click(screen.getByRole('button', { name: '社媒候选' }));
    const detail = await screen.findByRole('region', { name: '标签详情' });
    await user.selectOptions(within(detail).getByLabelText('合并到目标标签'), '客户精选');
    await user.click(within(detail).getByRole('button', { name: '合并标签' }));

    const mergeDialog = await screen.findByRole('dialog', { name: '确认合并标签' });
    expect(within(mergeDialog).getByText(/将把“社媒候选”的资产关联迁移到“客户精选”/)).toBeInTheDocument();
    await user.click(within(mergeDialog).getByRole('button', { name: '确认合并' }));

    await waitFor(() =>
      expect(onFeedback).toHaveBeenCalledWith(
        expect.objectContaining({
          message: '标签已合并',
          tone: 'success',
        }),
      ),
    );
    expect(screen.queryByRole('button', { name: '社媒候选' })).toBeNull();

    await user.click(screen.getByRole('button', { name: '待校验' }));
    await user.click(within(await screen.findByRole('region', { name: '标签详情' })).getByRole('button', { name: '删除标签' }));
    const deleteDialog = await screen.findByRole('dialog', { name: '确认删除标签' });
    expect(within(deleteDialog).getByText(/影响资产/)).toBeInTheDocument();
    await user.click(within(deleteDialog).getByRole('button', { name: '确认删除' }));

    await waitFor(() =>
      expect(onFeedback).toHaveBeenCalledWith(
        expect.objectContaining({
          message: '标签已删除',
          tone: 'warning',
        }),
      ),
    );
  });
});
