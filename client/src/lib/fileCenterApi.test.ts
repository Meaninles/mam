// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { __FILE_CENTER_TESTING__, fileCenterApi, resetFileCenterMock } from './fileCenterApi';

vi.mock('./runtimeConfig', () => ({
  getRuntimeConfig: () => ({
    centerBaseUrl: 'http://127.0.0.1:8080',
  }),
}));

describe('fileCenterApi', () => {
  afterEach(async () => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    await resetFileCenterMock();
  });

  it('从中心服务读取目录列表并缓存端点名', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          data: {
            breadcrumbs: [{ id: null, label: '商业摄影资产库' }],
            items: [
              {
                id: 'dir-raw',
                libraryId: 'photo',
                parentId: null,
                type: 'folder',
                lifecycleState: 'ACTIVE',
                name: '原片',
                fileKind: '文件夹',
                displayType: '文件夹',
                modifiedAt: '2026-04-10 12:20',
                createdAt: '2026-04-10 12:20',
                size: '2 项',
                path: '商业摄影资产库 / 原片',
                sourceLabel: '统一目录树',
                lastTaskText: '暂无任务',
                lastTaskTone: 'info',
                rating: 0,
                colorLabel: '无',
                badges: [],
                riskTags: [],
                tags: [],
                endpoints: [
                  {
                    mountId: 'mount-photo-raw',
                    name: '商业摄影原片库',
                    state: '已同步',
                    tone: 'success',
                    lastSyncAt: '2026-04-10 12:20',
                    endpointType: 'local',
                  },
                ],
              },
            ],
            total: 1,
            currentPathChildren: 1,
            endpointNames: ['商业摄影原片库'],
          },
        }),
      }),
    );

    const result = await fileCenterApi.loadDirectory({
      libraryId: 'photo',
      parentId: null,
      page: 1,
      pageSize: 20,
      searchText: '',
      fileTypeFilter: '全部',
      statusFilter: '全部',
      sortValue: '修改时间',
      sortDirection: 'desc',
      partialSyncEndpointNames: [],
    });

    expect(result.breadcrumbs[0]?.label).toBe('商业摄影资产库');
    expect(result.items[0]?.name).toBe('原片');
    expect(fileCenterApi.listLibraryEndpointNames('photo')).toEqual(['商业摄影原片库']);
  });

  it('目录项会清空星级和色标', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          data: {
            breadcrumbs: [{ id: null, label: '商业摄影资产库' }],
            items: [
              {
                id: 'dir-raw',
                libraryId: 'photo',
                parentId: null,
                type: 'folder',
                lifecycleState: 'ACTIVE',
                name: '拍摄原片',
                fileKind: '文件夹',
                displayType: '文件夹',
                modifiedAt: '2026-04-10 12:20',
                createdAt: '2026-04-10 12:20',
                size: '2 项',
                path: '商业摄影资产库 / 拍摄原片',
                sourceLabel: '统一目录',
                lastTaskText: '暂无任务',
                lastTaskTone: 'info',
                rating: 5,
                colorLabel: '红标',
                badges: [],
                riskTags: [],
                tags: [],
                endpoints: [],
              },
            ],
            total: 1,
            currentPathChildren: 1,
            endpointNames: [],
          },
        }),
      }),
    );

    const result = await fileCenterApi.loadDirectory({
      libraryId: 'photo',
      parentId: null,
      page: 1,
      pageSize: 20,
      searchText: '',
      fileTypeFilter: '全部',
      statusFilter: '全部',
      sortValue: '修改时间',
      sortDirection: 'desc',
      partialSyncEndpointNames: [],
    });

    expect(result.items[0]?.type).toBe('folder');
    expect(result.items[0]?.rating).toBe(0);
    expect(result.items[0]?.colorLabel).toBe('无');
  });

  it('按星级排序时目录保持原顺序', () => {
    const rows: Parameters<typeof __FILE_CENTER_TESTING__.sortEntryRowsForDisplay>[0] = [
      {
        id: 'dir-b',
        library_id: 'photo',
        parent_id: null,
        type: 'folder',
        lifecycle_state: 'ACTIVE',
        name: 'B目录',
        file_kind: '文件夹',
        display_type: '文件夹',
        modified_at: '2026-04-10 10:00',
        modified_at_sort: 10,
        created_at: '2026-04-10 10:00',
        size_label: '0 项',
        size_bytes: 0,
        path: 'B',
        source_label: '',
        notes: '',
        last_task_text: '',
        last_task_tone: 'info',
        rating: 5,
        color_label: '红标',
      },
      {
        id: 'dir-a',
        library_id: 'photo',
        parent_id: null,
        type: 'folder',
        lifecycle_state: 'ACTIVE',
        name: 'A目录',
        file_kind: '文件夹',
        display_type: '文件夹',
        modified_at: '2026-04-10 09:00',
        modified_at_sort: 9,
        created_at: '2026-04-10 09:00',
        size_label: '0 项',
        size_bytes: 0,
        path: 'A',
        source_label: '',
        notes: '',
        last_task_text: '',
        last_task_tone: 'info',
        rating: 1,
        color_label: '无',
      },
      {
        id: 'file-1',
        library_id: 'photo',
        parent_id: null,
        type: 'file',
        lifecycle_state: 'ACTIVE',
        name: 'one.jpg',
        file_kind: '图片',
        display_type: 'JPG 图片',
        modified_at: '2026-04-10 09:00',
        modified_at_sort: 9,
        created_at: '2026-04-10 09:00',
        size_label: '1 KB',
        size_bytes: 1,
        path: 'one.jpg',
        source_label: '',
        notes: '',
        last_task_text: '',
        last_task_tone: 'info',
        rating: 1,
        color_label: '无',
      },
      {
        id: 'file-2',
        library_id: 'photo',
        parent_id: null,
        type: 'file',
        lifecycle_state: 'ACTIVE',
        name: 'two.jpg',
        file_kind: '图片',
        display_type: 'JPG 图片',
        modified_at: '2026-04-10 10:00',
        modified_at_sort: 10,
        created_at: '2026-04-10 10:00',
        size_label: '1 KB',
        size_bytes: 1,
        path: 'two.jpg',
        source_label: '',
        notes: '',
        last_task_text: '',
        last_task_tone: 'info',
        rating: 5,
        color_label: '无',
      },
    ];

    const sorted = __FILE_CENTER_TESTING__.sortEntryRowsForDisplay(rows, '星级', 'desc');
    expect(sorted.map((row) => row.id)).toEqual(['dir-b', 'dir-a', 'file-2', 'file-1']);
  });

  it('会把同步到 115 识别为 CloudDrive2 上传并标记支持断点续传', () => {
    const entry = {
      id: 'asset-1',
      libraryId: 'photo',
      parentId: 'root',
      type: 'file' as const,
      lifecycleState: 'ACTIVE' as const,
      name: 'A001.RAW',
      fileKind: '图片' as const,
      displayType: 'RAW 图像',
      modifiedAt: '2026-04-10 12:20',
      createdAt: '2026-04-10 12:20',
      size: '47.8 MB',
      path: '/A001.RAW',
      sourceLabel: 'Sony A7R V',
      lastTaskText: '等待补齐到 115',
      lastTaskTone: 'warning' as const,
      rating: 0,
      colorLabel: '无' as const,
      badges: [],
      riskTags: [],
      tags: [],
      endpoints: [
        { name: '本地NVMe', state: '已同步' as const, tone: 'success' as const, lastSyncAt: '今天 09:18', endpointType: 'local' as const },
        { name: '115', state: '未同步' as const, tone: 'critical' as const, lastSyncAt: '尚未开始', endpointType: 'cloud' as const },
      ],
    };

    const plan = __FILE_CENTER_TESTING__.resolveSyncPlan(entry, '115');

    expect(plan).toMatchObject({
      routeType: 'UPLOAD',
      engine: 'CD2_REMOTE_UPLOAD',
      supportsResume: true,
      usesCloud: true,
      targetEndpointName: '115',
      sourceEndpointName: '本地NVMe',
    });
    expect(plan?.summary).toContain('CloudDrive2');
    expect(plan?.summary).toContain('断点续传');
  });

  it('会把从 115 同步到本地识别为 aria2 下载并要求 aria2 在线', () => {
    const entry = {
      id: 'asset-2',
      libraryId: 'photo',
      parentId: 'root',
      type: 'file' as const,
      lifecycleState: 'ACTIVE' as const,
      name: 'A002.RAW',
      fileKind: '图片' as const,
      displayType: 'RAW 图像',
      modifiedAt: '2026-04-10 12:20',
      createdAt: '2026-04-10 12:20',
      size: '47.8 MB',
      path: '/A002.RAW',
      sourceLabel: 'Sony A7R V',
      lastTaskText: '等待下载到本地',
      lastTaskTone: 'warning' as const,
      rating: 0,
      colorLabel: '无' as const,
      badges: [],
      riskTags: [],
      tags: [],
      endpoints: [
        { name: '本地NVMe', state: '未同步' as const, tone: 'critical' as const, lastSyncAt: '尚未开始', endpointType: 'local' as const },
        { name: '115', state: '已同步' as const, tone: 'success' as const, lastSyncAt: '今天 09:18', endpointType: 'cloud' as const },
      ],
    };

    const plan = __FILE_CENTER_TESTING__.resolveSyncPlan(entry, '本地NVMe');
    const availability = __FILE_CENTER_TESTING__.resolveSyncAvailability(entry, '本地NVMe', {
      cd2Online: true,
      aria2Online: false,
      cloudAuthReady: true,
      aria2Message: 'aria2 未就绪',
    });

    expect(plan).toMatchObject({
      routeType: 'DOWNLOAD',
      engine: 'ARIA2',
      supportsResume: true,
      usesCloud: true,
      sourceEndpointName: '115',
      targetEndpointName: '本地NVMe',
    });
    expect(plan?.summary).toContain('aria2');
    expect(plan?.summary).toContain('断点续传');
    expect(availability).toMatchObject({
      enabled: false,
      reason: 'aria2 当前不可用，请先前往设置页处理',
    });
  });

  it('删除 115 副本会要求 CloudDrive2 在线且 115 节点鉴权可用', () => {
    const entry = {
      id: 'asset-3',
      libraryId: 'photo',
      parentId: 'root',
      type: 'file' as const,
      lifecycleState: 'ACTIVE' as const,
      name: 'A003.RAW',
      fileKind: '图片' as const,
      displayType: 'RAW 图像',
      modifiedAt: '2026-04-10 12:20',
      createdAt: '2026-04-10 12:20',
      size: '47.8 MB',
      path: '/A003.RAW',
      sourceLabel: 'Sony A7R V',
      lastTaskText: '云端已归档',
      lastTaskTone: 'success' as const,
      rating: 0,
      colorLabel: '无' as const,
      badges: [],
      riskTags: [],
      tags: [],
      endpoints: [
        { name: '115', state: '已同步' as const, tone: 'success' as const, lastSyncAt: '今天 09:18', endpointType: 'cloud' as const },
      ],
    };

    const availability = __FILE_CENTER_TESTING__.resolveDeleteAvailability(entry, '115', {
      cd2Online: true,
      aria2Online: true,
      cloudAuthReady: false,
      cloudAuthMessage: '115 节点鉴权未就绪，请先前往存储节点页处理',
    });

    expect(availability).toMatchObject({
      enabled: false,
      reason: '115 节点鉴权未就绪，请先前往存储节点页处理',
    });
    expect(availability.execution?.summary).toContain('CloudDrive2');
  });

  it('从中心服务读取文件详情', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          data: {
            id: 'asset-cover',
            libraryId: 'photo',
            parentId: 'dir-raw',
            type: 'file',
            lifecycleState: 'ACTIVE',
            name: 'cover.jpg',
            fileKind: '图片',
            displayType: 'JPG 图片',
            modifiedAt: '2026-04-10 12:20',
            createdAt: '2026-04-10 12:20',
            size: '1.2 MB',
            path: '商业摄影资产库 / 原片 / cover.jpg',
            sourceLabel: '统一资产',
            lastTaskText: '暂无任务',
            lastTaskTone: 'info',
            rating: 0,
            colorLabel: '无',
            badges: [],
            riskTags: [],
            tags: [],
            endpoints: [
              {
                mountId: 'mount-photo-raw',
                name: '商业摄影原片库',
                state: '已同步',
                tone: 'success',
                lastSyncAt: '2026-04-10 12:20',
                endpointType: 'local',
              },
            ],
          },
        }),
      }),
    );

    const result = await fileCenterApi.loadEntryDetail('asset-cover');

    expect(result?.name).toBe('cover.jpg');
  });

  it('目录查询会透传分页和筛选参数', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          breadcrumbs: [],
          items: [],
          total: 0,
          currentPathChildren: 0,
          endpointNames: ['商业摄影原片库', '影像 NAS'],
        },
      }),
    });
    vi.stubGlobal('fetch', fetchMock);

    await fileCenterApi.loadDirectory({
      libraryId: 'photo',
      parentId: 'dir-raw',
      page: 2,
      pageSize: 50,
      searchText: 'cover',
      fileTypeFilter: '图片',
      statusFilter: '部分同步',
      sortValue: '名称',
      sortDirection: 'asc',
      partialSyncEndpointNames: ['影像 NAS'],
    });

    const requestUrl = String(fetchMock.mock.calls[0]?.[0] ?? '');
    expect(requestUrl).toContain('/api/libraries/photo/browse?');
    expect(requestUrl).toContain('parentId=dir-raw');
    expect(requestUrl).toContain('page=2');
    expect(requestUrl).toContain('pageSize=50');
    expect(requestUrl).toContain('searchText=cover');
    expect(requestUrl).toContain('fileTypeFilter=%E5%9B%BE%E7%89%87');
    expect(requestUrl).toContain('statusFilter=%E9%83%A8%E5%88%86%E5%90%8C%E6%AD%A5');
    expect(requestUrl).toContain('sortValue=%E5%90%8D%E7%A7%B0');
    expect(requestUrl).toContain('sortDirection=asc');
    expect(requestUrl).toContain('partialSyncEndpointName=%E5%BD%B1%E5%83%8F+NAS');
  });

  it('删除资产走中心服务作业接口', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          message: '已创建资产删除作业',
          jobId: 'job-delete-asset-legacy',
        },
      }),
    });
    vi.stubGlobal('fetch', fetchMock);

    await expect(fileCenterApi.deleteAssets(['dir-new'])).resolves.toEqual({
      message: '已创建资产删除作业',
      jobId: 'job-delete-asset-legacy',
    });

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:8080/api/file-entries/delete-assets',
      expect.objectContaining({
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          entryIds: ['dir-new'],
        }),
      }),
    );
  });

  it('同步到端点会创建真实作业', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          message: '已创建同步作业',
          jobId: 'job-sync-1',
        },
      }),
    });
    vi.stubGlobal('fetch', fetchMock);

    await expect(
      fileCenterApi.syncToEndpoint({
        entryIds: ['asset-cover', 'asset-final'],
        endpointName: '影像 NAS',
      }),
    ).resolves.toEqual({
      message: '已创建同步作业',
      jobId: 'job-sync-1',
    });

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:8080/api/file-entries/replicate',
      expect.objectContaining({
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          entryIds: ['asset-cover', 'asset-final'],
          endpointName: '影像 NAS',
        }),
      }),
    );
  });

  it('删除端点副本会创建真实作业', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          message: '已创建副本删除作业',
          jobId: 'job-delete-replica-1',
        },
      }),
    });
    vi.stubGlobal('fetch', fetchMock);

    await expect(
      fileCenterApi.deleteFromEndpoint({
        entryIds: ['asset-cover'],
        endpointName: '商业摄影原片库',
      }),
    ).resolves.toEqual({
      message: '已创建副本删除作业',
      jobId: 'job-delete-replica-1',
    });

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:8080/api/file-entries/delete-replicas',
      expect.objectContaining({
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          entryIds: ['asset-cover'],
          endpointName: '商业摄影原片库',
        }),
      }),
    );
  });

  it('删除资产会创建真实作业', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          message: '已创建资产删除作业',
          jobId: 'job-delete-asset-1',
        },
      }),
    });
    vi.stubGlobal('fetch', fetchMock);

    await expect(fileCenterApi.deleteAssets(['asset-cover', 'asset-final'])).resolves.toEqual({
      message: '已创建资产删除作业',
      jobId: 'job-delete-asset-1',
    });

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:8080/api/file-entries/delete-assets',
      expect.objectContaining({
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          entryIds: ['asset-cover', 'asset-final'],
        }),
      }),
    );
  });

  it('进入目录后的自动扫描会带上当前目录 id', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          message: '当前目录扫描已完成',
        },
      }),
    });
    vi.stubGlobal('fetch', fetchMock);

    await expect(
      fileCenterApi.scanDirectory({
        libraryId: 'photo',
        parentId: 'dir-raw',
      }),
    ).resolves.toEqual({
      message: '当前目录扫描已完成',
    });

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:8080/api/libraries/photo/scan',
      expect.objectContaining({
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          parentId: 'dir-raw',
        }),
      }),
    );
  });

  it('上传会通过 FormData 把文件和清单发送到中心服务', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          message: '已上传 1 个文件',
          createdCount: 1,
        },
      }),
    });
    vi.stubGlobal('fetch', fetchMock);

    const file = new File(['cover-image'], 'cover.jpg', { type: 'image/jpeg' });
    const result = await fileCenterApi.uploadSelection({
      libraryId: 'photo',
      parentId: 'dir-raw',
      mode: 'files',
      items: [
        {
          file,
          name: 'cover.jpg',
          size: file.size,
          relativePath: 'cover.jpg',
        },
      ],
    });

    expect(result).toEqual({
      message: '已上传 1 个文件',
      createdCount: 1,
    });

    const requestUrl = String(fetchMock.mock.calls[0]?.[0] ?? '');
    const requestInit = fetchMock.mock.calls[0]?.[1] as RequestInit | undefined;
    expect(requestUrl).toBe('http://127.0.0.1:8080/api/libraries/photo/uploads');
    expect(requestInit?.method).toBe('POST');
    expect(requestInit?.body).toBeInstanceOf(FormData);

    const formData = requestInit?.body as FormData;
    expect(formData.get('mode')).toBe('files');
    expect(formData.get('parentId')).toBe('dir-raw');
    const uploadedFile = formData.get('file0');
    expect(uploadedFile).toBeInstanceOf(File);
    expect((uploadedFile as File).name).toBe('cover.jpg');
    expect((uploadedFile as File).size).toBe(file.size);
    expect(formData.get('manifest')).toBe(
      JSON.stringify([
        {
          field: 'file0',
          name: 'cover.jpg',
          relativePath: 'cover.jpg',
        },
      ]),
    );
  });

  it('详情页保存星级和色标会调用中心服务注解接口', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          message: '资产标记已更新',
        },
      }),
    });
    vi.stubGlobal('fetch', fetchMock);

    await expect(
      fileCenterApi.updateAnnotations('asset-cover', {
        rating: 5,
        colorLabel: '红标',
        tags: ['发布会'],
      }),
    ).resolves.toEqual({
      message: '资产标记已更新',
    });

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:8080/api/file-entries/asset-cover/annotations',
      expect.objectContaining({
        method: 'PATCH',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          rating: 5,
          colorLabel: '红标',
          tags: ['发布会'],
        }),
      }),
    );
  });

  it('标签管理快照走中心服务接口', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          overview: {
            totalTags: 1,
            usedTagCount: 1,
            ungroupedTagCount: 1,
            crossLibraryTagCount: 0,
          },
          groups: [
            {
              id: 'tag-group-ungrouped',
              name: '未分组',
              orderIndex: 0,
              tagCount: 1,
              usedTagCount: 1,
            },
          ],
          tags: [
            {
              id: 'tag-1',
              name: '直播切片',
              normalizedName: '直播切片',
              groupId: 'tag-group-ungrouped',
              groupName: '未分组',
              orderIndex: 0,
              isPinned: false,
              usageCount: 1,
              libraryIds: ['photo'],
              linkedLibraryIds: ['photo'],
              outOfScopeUsageCount: 0,
              createdAt: '2026-04-11 10:00',
              updatedAt: '2026-04-11 10:00',
            },
          ],
          libraries: [{ id: 'photo', name: '商业摄影资产库' }],
        },
      }),
    });
    vi.stubGlobal('fetch', fetchMock);

    const snapshot = await fileCenterApi.loadTagManagementSnapshot('直播');

    expect(snapshot.tags[0]?.name).toBe('直播切片');
    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:8080/api/tags/management?searchText=%E7%9B%B4%E6%92%AD',
      undefined,
    );
  });

  it('标签建议走中心服务接口', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: [
          {
            id: 'tag-1',
            name: '直播切片',
            count: 3,
            groupName: '未分组',
            isPinned: true,
            libraryIds: ['photo'],
          },
        ],
      }),
    });
    vi.stubGlobal('fetch', fetchMock);

    const suggestions = await fileCenterApi.loadTagSuggestions('直播', 'photo');

    expect(suggestions[0]?.name).toBe('直播切片');
    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:8080/api/tags/suggestions?searchText=%E7%9B%B4%E6%92%AD&libraryId=photo',
      undefined,
    );
  });
});
