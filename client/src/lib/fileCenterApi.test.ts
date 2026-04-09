// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { fileCenterApi, resetFileCenterMock } from './fileCenterApi';

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
                notes: '',
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
                metadata: [],
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
            notes: '',
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
            metadata: [{ label: '逻辑路径', value: '/原片/cover.jpg' }],
          },
        }),
      }),
    );

    const result = await fileCenterApi.loadEntryDetail('asset-cover');

    expect(result?.name).toBe('cover.jpg');
    expect(result?.metadata[0]?.value).toBe('/原片/cover.jpg');
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

  it('删除条目走中心服务接口', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          message: '条目已删除',
        },
      }),
    });
    vi.stubGlobal('fetch', fetchMock);

    await expect(fileCenterApi.deleteAssets(['dir-new'])).resolves.toEqual({
      message: '条目已删除',
    });

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:8080/api/file-entries/dir-new',
      expect.objectContaining({
        method: 'DELETE',
      }),
    );
  });
});
