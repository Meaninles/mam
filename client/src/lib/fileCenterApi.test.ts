import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { fileCenterApi, resetFileCenterMock } from './fileCenterApi';

describe('fileCenterApi', () => {
  beforeEach(async () => {
    window.localStorage.clear();
    await resetFileCenterMock();
  });

  afterEach(async () => {
    await resetFileCenterMock();
  });

  it('支持按目录加载文件中心列表，并返回面包屑与分页信息', async () => {
    const result = await fileCenterApi.loadDirectory({
      libraryId: 'photo',
      parentId: null,
      page: 1,
      pageSize: 20,
      searchText: '',
      fileTypeFilter: '全部',
      statusFilter: '全部',
      sortValue: '修改时间',
    });

    expect(result.breadcrumbs.at(0)?.label).toBe('商业摄影资产库');
    expect(result.items.length).toBeGreaterThan(0);
    expect(result.total).toBeGreaterThan(20);
    expect(result.items.some((item) => item.type === 'folder')).toBe(true);
  });

  it('支持在当前目录创建新目录，并在后续查询中可见', async () => {
    const created = await fileCenterApi.createFolder({
      libraryId: 'photo',
      parentId: null,
      name: '新建目录',
    });

    expect(created.message).toBe('目录已创建');

    const result = await fileCenterApi.loadDirectory({
      libraryId: 'photo',
      parentId: null,
      page: 1,
      pageSize: 50,
      searchText: '新建目录',
      fileTypeFilter: '全部',
      statusFilter: '全部',
      sortValue: '名称',
    });

    expect(result.items.some((item) => item.id === created.item.id)).toBe(true);
  });

  it('支持从指定端点删除副本，并保留其它端点状态', async () => {
    const detail = await fileCenterApi.loadEntryDetail('photo-file-raw-001');
    expect(detail).not.toBeNull();

    const result = await fileCenterApi.deleteFromEndpoint('photo-file-raw-001', '115');
    expect(result.message).toBe('已提交端点删除请求');

    const next = await fileCenterApi.loadEntryDetail('photo-file-raw-001');
    expect(next?.lifecycleState).toBe('ACTIVE');
    expect(next?.endpoints.find((item) => item.name === '115')?.state).toBe('未同步');
    expect(next?.endpoints.some((item) => item.state === '已同步')).toBe(true);
  });

  it('支持将资产同步到指定端点，并更新端点状态', async () => {
    const result = await fileCenterApi.syncToEndpoint('photo-file-raw-001', '115');
    expect(result.message).toContain('已创建同步任务');

    const next = await fileCenterApi.loadEntryDetail('photo-file-raw-001');
    expect(next?.endpoints.find((item) => item.name === '115')?.state).toBe('同步中');
  });

  it('支持更新资产的星级、色标和标签', async () => {
    await fileCenterApi.updateAnnotations('photo-file-raw-001', {
      rating: 4,
      colorLabel: '红标',
      tags: ['封面候选', '客户已选'],
    });

    const next = await fileCenterApi.loadEntryDetail('photo-file-raw-001');
    expect(next?.rating).toBe(4);
    expect(next?.colorLabel).toBe('红标');
    expect(next?.tags).toEqual(expect.arrayContaining(['封面候选', '客户已选']));
  });

  it('支持批量删除资产，并将生命周期标记为等待清理', async () => {
    const result = await fileCenterApi.deleteAssets(['photo-file-raw-001', 'photo-file-raw-002']);
    expect(result.message).toBe('删除请求已提交，资产进入等待清理');

    const detail = await fileCenterApi.loadEntryDetail('photo-file-raw-001');
    expect(detail?.lifecycleState).toBe('PENDING_DELETE');
    expect(detail?.endpoints.every((item) => item.state === '未同步')).toBe(true);
  });

  it('支持查询已有标签建议', async () => {
    const tags = await fileCenterApi.loadTagSuggestions('发');
    expect(tags.some((item) => item.name.includes('发布会'))).toBe(true);
  });
});
