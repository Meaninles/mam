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

type MockManagedTag = {
  id: string;
  name: string;
  normalizedName: string;
  groupId: string;
  groupName: string;
  orderIndex: number;
  isPinned: boolean;
  usageCount: number;
  libraryIds: string[];
  linkedLibraryIds: string[];
  outOfScopeUsageCount: number;
  createdAt: string;
  updatedAt: string;
};

function createTagApiFetchMock() {
  const groups = [
    { id: 'tag-group-ungrouped', name: '未分组', orderIndex: 0 },
    { id: 'tag-group-project', name: '项目语义', orderIndex: 1 },
  ];
  const tags: MockManagedTag[] = [
    {
      id: 'tag-social',
      name: '社媒候选',
      normalizedName: '社媒候选',
      groupId: 'tag-group-project',
      groupName: '项目语义',
      orderIndex: 0,
      isPinned: false,
      usageCount: 2,
      libraryIds: ['photo'],
      linkedLibraryIds: ['photo'],
      outOfScopeUsageCount: 0,
      createdAt: '2026-04-11 10:00',
      updatedAt: '2026-04-11 10:00',
    },
    {
      id: 'tag-picked',
      name: '客户精选',
      normalizedName: '客户精选',
      groupId: 'tag-group-project',
      groupName: '项目语义',
      orderIndex: 1,
      isPinned: true,
      usageCount: 3,
      libraryIds: ['photo', 'family'],
      linkedLibraryIds: ['photo', 'family'],
      outOfScopeUsageCount: 0,
      createdAt: '2026-04-11 10:00',
      updatedAt: '2026-04-11 10:00',
    },
    {
      id: 'tag-review',
      name: '待校验',
      normalizedName: '待校验',
      groupId: 'tag-group-ungrouped',
      groupName: '未分组',
      orderIndex: 0,
      isPinned: false,
      usageCount: 1,
      libraryIds: ['video', 'family'],
      linkedLibraryIds: ['video'],
      outOfScopeUsageCount: 0,
      createdAt: '2026-04-11 10:00',
      updatedAt: '2026-04-11 10:00',
    },
  ];

  const buildSnapshot = () => ({
    overview: {
      totalTags: tags.length,
      usedTagCount: tags.filter((tag) => tag.usageCount > 0).length,
      ungroupedTagCount: tags.filter((tag) => tag.groupId === 'tag-group-ungrouped').length,
      crossLibraryTagCount: tags.filter((tag) => tag.libraryIds.length > 1).length,
    },
    groups: groups.map((group) => {
      const groupTags = tags.filter((tag) => tag.groupId === group.id);
      return {
        id: group.id,
        name: group.name,
        orderIndex: group.orderIndex,
        tagCount: groupTags.length,
        usedTagCount: groupTags.filter((tag) => tag.usageCount > 0).length,
      };
    }),
    tags: [...tags],
    libraries: libraries.map((library) => ({ id: library.id, name: library.name })),
  });

  return vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    const method = init?.method ?? 'GET';

    if (url.endsWith('/api/tags/management') && method === 'GET') {
      return {
        ok: true,
        json: async () => ({ data: buildSnapshot() }),
      };
    }

    if (url.endsWith('/api/tags') && method === 'POST') {
      const payload = JSON.parse(String(init?.body ?? '{}')) as {
        name: string;
        groupId: string;
        libraryIds: string[];
        isPinned: boolean;
      };
      const group = groups.find((item) => item.id === payload.groupId) ?? groups[0]!;
      tags.push({
        id: `tag-${tags.length + 1}`,
        name: payload.name,
        normalizedName: payload.name,
        groupId: payload.groupId,
        groupName: group.name,
        orderIndex: tags.filter((tag) => tag.groupId === payload.groupId).length,
        isPinned: payload.isPinned,
        usageCount: 0,
        libraryIds: payload.libraryIds,
        linkedLibraryIds: [],
        outOfScopeUsageCount: 0,
        createdAt: '2026-04-11 10:00',
        updatedAt: '2026-04-11 10:00',
      });
      return {
        ok: true,
        json: async () => ({ data: { message: '标签已创建', tagId: tags.at(-1)?.id } }),
      };
    }

    if (url.includes('/api/tags/') && url.endsWith('/merge') && method === 'POST') {
      const sourceId = url.split('/api/tags/')[1]!.replace('/merge', '');
      JSON.parse(String(init?.body ?? '{}')) as { targetId: string };
      const sourceIndex = tags.findIndex((tag) => tag.id === sourceId);
      if (sourceIndex >= 0) {
        tags.splice(sourceIndex, 1);
      }
      return {
        ok: true,
        json: async () => ({ data: { message: '标签已合并' } }),
      };
    }

    if (url.includes('/api/tags/') && method === 'DELETE') {
      const tagId = url.split('/api/tags/')[1]!;
      const tagIndex = tags.findIndex((tag) => tag.id === tagId);
      if (tagIndex >= 0) {
        tags.splice(tagIndex, 1);
      }
      return {
        ok: true,
        json: async () => ({ data: { message: '标签已删除' } }),
      };
    }

    throw new Error(`Unhandled fetch: ${method} ${url}`);
  });
}

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
    vi.stubGlobal('fetch', createTagApiFetchMock());

    render(<TagManagementPage libraries={libraries} onFeedback={onFeedback} />);

    expect(await screen.findByText('标签总数')).toBeInTheDocument();
    expect(screen.getByText('使用中标签')).toBeInTheDocument();
    expect(screen.getByText('未分组标签')).toBeInTheDocument();
    expect(screen.getByText('跨库标签')).toBeInTheDocument();

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
    vi.stubGlobal('fetch', createTagApiFetchMock());

    render(<TagManagementPage libraries={libraries} onFeedback={onFeedback} />);
    await screen.findByText('标签总数');

    await user.click(screen.getByRole('button', { name: '社媒候选' }));
    const detail = await screen.findByRole('region', { name: '标签详情' });
    expect(within(detail).getByText('合并标签（危险操作）')).toBeInTheDocument();
    expect(within(detail).getByText('合并标签需二次确认，请确认范围。')).toBeInTheDocument();
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
