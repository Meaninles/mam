import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
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
    expect(result.items.find((item) => item.id === 'photo-root-raw')?.size).toBe('64 项');
    expect(result.items.find((item) => item.id === 'photo-root-delivery')?.size).toBe('1 项');
  });

  it('文件夹存储状态会按嵌套文件真实状态派生', async () => {
    const result = await fileCenterApi.loadDirectory({
      libraryId: 'photo',
      parentId: null,
      page: 1,
      pageSize: 100,
      searchText: '',
      fileTypeFilter: '全部',
      statusFilter: '全部',
      sortValue: '修改时间',
    });

    const rawFolder = result.items.find((item) => item.id === 'photo-root-raw');
    const deliveryFolder = result.items.find((item) => item.id === 'photo-root-delivery');
    const retouchFolder = result.items.find((item) => item.id === 'photo-root-retouch');

    expect(rawFolder?.endpoints.find((item) => item.name === '本地NVMe')?.state).toBe('已同步');
    expect(rawFolder?.endpoints.find((item) => item.name === '影像NAS')?.state).toBe('未同步');
    expect(rawFolder?.endpoints.find((item) => item.name === '115')?.state).toBe('部分同步');

    expect(deliveryFolder?.endpoints.every((item) => item.state === '已同步')).toBe(true);
    expect(retouchFolder?.endpoints.every((item) => item.state === '未同步')).toBe(true);
  });

  it('支持按完全同步筛选', async () => {
    const result = await fileCenterApi.loadDirectory({
      libraryId: 'photo',
      parentId: null,
      page: 1,
      pageSize: 100,
      searchText: '',
      fileTypeFilter: '全部',
      statusFilter: '完全同步',
      sortValue: '修改时间',
    });

    expect(result.items.some((item) => item.id === 'photo-root-delivery')).toBe(true);
    expect(result.items.some((item) => item.id === 'photo-root-raw')).toBe(false);
  });

  it('支持按未同步筛选', async () => {
    const result = await fileCenterApi.loadDirectory({
      libraryId: 'photo',
      parentId: null,
      page: 1,
      pageSize: 100,
      searchText: '',
      fileTypeFilter: '全部',
      statusFilter: '未同步',
      sortValue: '修改时间',
    });

    expect(result.items.some((item) => item.id === 'photo-root-retouch')).toBe(true);
    expect(result.items.some((item) => item.id === 'photo-root-raw')).toBe(false);
  });

  it('支持按部分同步和端点组合筛选', async () => {
    const result = await fileCenterApi.loadDirectory({
      libraryId: 'photo',
      parentId: null,
      page: 1,
      pageSize: 100,
      searchText: '',
      fileTypeFilter: '全部',
      statusFilter: '部分同步',
      partialSyncEndpointNames: ['影像NAS', '115'],
      sortValue: '修改时间',
    });

    expect(result.items).toHaveLength(0);
    expect(result.items.some((item) => item.id === 'photo-root-delivery')).toBe(false);
  });

  it('部分同步端点组合筛选使用 AND 关系', async () => {
    const result = await fileCenterApi.loadDirectory({
      libraryId: 'photo',
      parentId: null,
      page: 1,
      pageSize: 100,
      searchText: '',
      fileTypeFilter: '全部',
      statusFilter: '部分同步',
      partialSyncEndpointNames: ['本地NVMe', '影像NAS', '115'],
      sortValue: '修改时间',
    });

    expect(result.items.some((item) => item.id === 'photo-root-raw')).toBe(false);
    expect(result.items.some((item) => item.id === 'photo-root-retouch')).toBe(false);
  });

  it('支持按星级逆序排序，相同时按修改时间从新到旧', async () => {
    const result = await fileCenterApi.loadDirectory({
      libraryId: 'photo',
      parentId: 'photo-root-raw',
      page: 1,
      pageSize: 10,
      searchText: '',
      fileTypeFilter: '全部',
      statusFilter: '全部',
      sortValue: '星级',
      sortDirection: 'desc',
    });

    expect(result.items[0]?.rating).toBe(5);
    expect(result.items[1]?.rating).toBeGreaterThanOrEqual(result.items[2]?.rating ?? 0);
  });

  it('支持按星级正序排序', async () => {
    const result = await fileCenterApi.loadDirectory({
      libraryId: 'photo',
      parentId: 'photo-root-raw',
      page: 1,
      pageSize: 10,
      searchText: '',
      fileTypeFilter: '全部',
      statusFilter: '全部',
      sortValue: '星级',
      sortDirection: 'asc',
    });

    expect(result.items[0]?.rating).toBe(0);
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
    expect(created.item.endpoints.every((item) => item.state === '未同步')).toBe(true);
  });

  it('支持上传文件，并将新文件标记为仅本地已同步', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-04-01T13:24:35+08:00'));

    try {
    const result = await fileCenterApi.uploadSelection({
      libraryId: 'photo',
      parentId: 'photo-root-raw',
      mode: 'files',
      items: [{ name: '客户logo.png', size: 1024 * 240 }],
    });

    expect(result.message).toBe('已开始上传 1 个文件');

    const directory = await fileCenterApi.loadDirectory({
      libraryId: 'photo',
      parentId: 'photo-root-raw',
      page: 1,
      pageSize: 100,
      searchText: '客户logo.png',
      fileTypeFilter: '全部',
      statusFilter: '全部',
      sortValue: '修改时间',
    });

    const uploaded = directory.items.find((item) => item.name === '客户logo.png');
    expect(uploaded).toBeTruthy();
    expect(uploaded?.modifiedAt).toBe('今天 13:24');
    expect(uploaded?.endpoints.find((item) => item.name === '本地NVMe')?.state).toBe('已同步');
    expect(uploaded?.endpoints.filter((item) => item.name !== '本地NVMe').every((item) => item.state === '未同步')).toBe(true);
    } finally {
      vi.useRealTimers();
    }
  });

  it('支持上传文件夹，并在当前目录下生成上传目录', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-04-01T14:08:12+08:00'));

    try {
    const fileA = new File(['a'], 'scene-001.jpg', { type: 'image/jpeg' });
    const fileB = new File(['b'], 'scene-002.jpg', { type: 'image/jpeg' });
    Object.defineProperty(fileA, 'webkitRelativePath', { value: '新品拍摄/scene-001.jpg' });
    Object.defineProperty(fileB, 'webkitRelativePath', { value: '新品拍摄/scene-002.jpg' });

    const result = await fileCenterApi.uploadSelection({
      libraryId: 'photo',
      parentId: 'photo-root-raw',
      mode: 'folder',
      items: [
        { name: fileA.name, size: fileA.size, relativePath: (fileA as File & { webkitRelativePath: string }).webkitRelativePath },
        { name: fileB.name, size: fileB.size, relativePath: (fileB as File & { webkitRelativePath: string }).webkitRelativePath },
      ],
    });

    expect(result.message).toContain('已开始上传文件夹');

    const directory = await fileCenterApi.loadDirectory({
      libraryId: 'photo',
      parentId: 'photo-root-raw',
      page: 1,
      pageSize: 100,
      searchText: '新品拍摄',
      fileTypeFilter: '全部',
      statusFilter: '全部',
      sortValue: '修改时间',
    });

    const uploadedFolder = directory.items.find((item) => item.name === '新品拍摄');
    expect(uploadedFolder).toBeTruthy();
    expect(uploadedFolder?.type).toBe('folder');
    expect(uploadedFolder?.modifiedAt).toBe('今天 14:08');
    expect(uploadedFolder?.endpoints.find((item) => item.name === '本地NVMe')?.state).toBe('已同步');
    expect(uploadedFolder?.endpoints.filter((item) => item.name !== '本地NVMe').every((item) => item.state === '未同步')).toBe(true);
    } finally {
      vi.useRealTimers();
    }
  });

  it('上传内容会按精确时间排序，而不是按显示文案排序', async () => {
    vi.useFakeTimers();

    try {
      vi.setSystemTime(new Date('2026-04-01T15:06:05+08:00'));
      await fileCenterApi.uploadSelection({
        libraryId: 'photo',
        parentId: 'photo-root-raw',
        mode: 'files',
        items: [{ name: 'alpha.png', size: 1024 }],
      });

      vi.setSystemTime(new Date('2026-04-01T15:06:45+08:00'));
      await fileCenterApi.uploadSelection({
        libraryId: 'photo',
        parentId: 'photo-root-raw',
        mode: 'files',
        items: [{ name: 'beta.png', size: 1024 }],
      });

      const result = await fileCenterApi.loadDirectory({
        libraryId: 'photo',
        parentId: 'photo-root-raw',
        page: 1,
        pageSize: 10,
        searchText: '',
        fileTypeFilter: '全部',
        statusFilter: '全部',
        sortValue: '修改时间',
        sortDirection: 'desc',
      });

      const names = result.items.slice(0, 2).map((item) => item.name);
      expect(names).toEqual(['beta.png', 'alpha.png']);
      expect(result.items[0]?.modifiedAt).toBe('今天 15:06');
      expect(result.items[1]?.modifiedAt).toBe('今天 15:06');
    } finally {
      vi.useRealTimers();
    }
  });

  it('支持从指定端点删除副本，并保留其它端点状态', async () => {
    const detail = await fileCenterApi.loadEntryDetail('photo-file-raw-001');
    expect(detail).not.toBeNull();

    const result = await fileCenterApi.deleteFromEndpoint('photo-file-raw-001', '影像NAS');
    expect(result.message).toBe('已提交端点删除请求');

    const next = await fileCenterApi.loadEntryDetail('photo-file-raw-001');
    expect(next?.lifecycleState).toBe('ACTIVE');
    expect(next?.endpoints.find((item) => item.name === '影像NAS')?.state).toBe('未同步');
    expect(next?.endpoints.some((item) => item.state === '已同步')).toBe(true);
  });

  it('文件夹同步会递归作用到所有可同步的后代文件', async () => {
    await fileCenterApi.syncToEndpoint('photo-root-raw', '115');

    const raw001 = await fileCenterApi.loadEntryDetail('photo-file-raw-001');
    const raw002 = await fileCenterApi.loadEntryDetail('photo-file-raw-002');

    expect(raw001?.endpoints.find((item) => item.name === '115')?.state).toBe('同步中');
    expect(raw002?.endpoints.find((item) => item.name === '115')?.state).toBe('同步中');
  });

  it('文件夹删除会递归作用到所有可删除的后代文件', async () => {
    await fileCenterApi.deleteFromEndpoint('photo-root-delivery', '115');

    const cover = await fileCenterApi.loadEntryDetail('photo-file-cover');
    expect(cover?.endpoints.find((item) => item.name === '115')?.state).toBe('未同步');
  });

  it('当各端都没有副本后会自动删除该资产', async () => {
    await fileCenterApi.deleteFromEndpoint('photo-file-raw-001', '本地NVMe');
    await fileCenterApi.deleteFromEndpoint('photo-file-raw-001', '影像NAS');

    const detail = await fileCenterApi.loadEntryDetail('photo-file-raw-001');
    expect(detail).toBeNull();

    const result = await fileCenterApi.loadDirectory({
      libraryId: 'photo',
      parentId: 'photo-root-raw',
      page: 1,
      pageSize: 100,
      searchText: '2026-03-29_上海发布会_A-cam_001.RAW',
      fileTypeFilter: '全部',
      statusFilter: '全部',
      sortValue: '修改时间',
    });

    expect(result.items.some((item) => item.id === 'photo-file-raw-001')).toBe(false);
  });

  it('支持将资产同步到指定端点，并更新端点状态', async () => {
    const result = await fileCenterApi.syncToEndpoint('photo-file-raw-001', '115');
    expect(result.message).toContain('已创建同步任务');

    const next = await fileCenterApi.loadEntryDetail('photo-file-raw-001');
    expect(next?.endpoints.find((item) => item.name === '115')?.state).toBe('同步中');
  });

  it('模拟同步任务会在 5 秒后自动完成并切换为已同步', async () => {
    vi.useFakeTimers();

    try {
      await fileCenterApi.syncToEndpoint('photo-file-raw-001', '115');

      let next = await fileCenterApi.loadEntryDetail('photo-file-raw-001');
      expect(next?.endpoints.find((item) => item.name === '115')?.state).toBe('同步中');

      await vi.advanceTimersByTimeAsync(5000);

      next = await fileCenterApi.loadEntryDetail('photo-file-raw-001');
      expect(next?.endpoints.find((item) => item.name === '115')?.state).toBe('已同步');
    } finally {
      vi.useRealTimers();
    }
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

  it('文件中心 mock 数据总量保持不变，且端点状态仅使用最新枚举', async () => {
    const [photoRoot, photoRaw, photoDelivery, videoRoot] = await Promise.all([
      fileCenterApi.loadDirectory({
        libraryId: 'photo',
        parentId: null,
        page: 1,
        pageSize: 100,
        searchText: '',
        fileTypeFilter: '全部',
        statusFilter: '全部',
        sortValue: '修改时间',
      }),
      fileCenterApi.loadDirectory({
        libraryId: 'photo',
        parentId: 'photo-root-raw',
        page: 1,
        pageSize: 100,
        searchText: '',
        fileTypeFilter: '全部',
        statusFilter: '全部',
        sortValue: '修改时间',
      }),
      fileCenterApi.loadDirectory({
        libraryId: 'photo',
        parentId: 'photo-root-delivery',
        page: 1,
        pageSize: 100,
        searchText: '',
        fileTypeFilter: '全部',
        statusFilter: '全部',
        sortValue: '修改时间',
      }),
      fileCenterApi.loadDirectory({
        libraryId: 'video',
        parentId: null,
        page: 1,
        pageSize: 100,
        searchText: '',
        fileTypeFilter: '全部',
        statusFilter: '全部',
        sortValue: '修改时间',
      }),
    ]);

    expect(photoRoot.total).toBe(46);
    expect(photoRaw.total).toBe(64);
    expect(photoDelivery.total).toBe(1);
    expect(videoRoot.total).toBe(2);

    const allEndpointStates = [
      ...photoRaw.items.flatMap((item) => item.endpoints.map((endpoint) => endpoint.state)),
      ...photoDelivery.items.flatMap((item) => item.endpoints.map((endpoint) => endpoint.state)),
      ...videoRoot.items.flatMap((item) => item.endpoints.map((endpoint) => endpoint.state)),
    ];

    expect(new Set(allEndpointStates)).toEqual(new Set(['已同步', '未同步', '同步中']));
  });
});
