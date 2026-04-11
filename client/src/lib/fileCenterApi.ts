import type { AssetLifecycleState, FileTypeFilter, Severity } from '../data';
import { getRuntimeConfig } from './runtimeConfig';

export type FileCenterStatusFilter = '全部' | '完全同步' | '部分同步' | '未同步';
export type FileCenterSortValue = '修改时间' | '名称' | '大小' | '星级';
export type FileCenterSortDirection = 'asc' | 'desc';
export type FileCenterEndpointType = 'local' | 'nas' | 'cloud' | 'removable';
export type FileCenterEndpointState = '已同步' | '未同步' | '同步中' | '部分同步';
export type FileCenterColorLabel = '无' | '红标' | '黄标' | '绿标' | '蓝标' | '紫标';

export type FileCenterEndpoint = {
  name: string;
  state: FileCenterEndpointState;
  tone: Severity;
  lastSyncAt: string;
  endpointType: FileCenterEndpointType;
};

type FileCenterMetadataRow = {
  label: string;
  value: string;
};

export type FileCenterEntry = {
  id: string;
  libraryId: string;
  parentId: string | null;
  type: 'folder' | 'file';
  lifecycleState: AssetLifecycleState;
  name: string;
  fileKind: FileTypeFilter;
  displayType: string;
  modifiedAt: string;
  createdAt: string;
  size: string;
  path: string;
  sourceLabel: string;
  lastTaskText: string;
  lastTaskTone: Severity;
  rating: number;
  colorLabel: FileCenterColorLabel;
  badges: string[];
  riskTags: string[];
  tags: string[];
  endpoints: FileCenterEndpoint[];
};

export type FileCenterTagSuggestion = {
  id: string;
  name: string;
  count: number;
  groupName: string;
  isPinned: boolean;
  libraryIds: string[];
};

export type ManagedTagGroup = {
  id: string;
  name: string;
  orderIndex: number;
  tagCount: number;
  usedTagCount: number;
};

export type ManagedTag = {
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

export type TagManagementOverview = {
  totalTags: number;
  usedTagCount: number;
  ungroupedTagCount: number;
  crossLibraryTagCount: number;
};

export type TagManagementSnapshot = {
  overview: TagManagementOverview;
  groups: ManagedTagGroup[];
  tags: ManagedTag[];
  libraries: Array<{ id: string; name: string }>;
};

export type FileCenterDirectoryResult = {
  breadcrumbs: Array<{ id: string | null; label: string }>;
  items: FileCenterEntry[];
  total: number;
  currentPathChildren: number;
  endpointNames?: string[];
};

export type FileCenterUploadItem = {
  file: File;
  name: string;
  size: number;
  relativePath?: string;
};

export type FileCenterLoadParams = {
  libraryId: string;
  parentId: string | null;
  page: number;
  pageSize: number;
  searchText: string;
  fileTypeFilter: FileTypeFilter;
  statusFilter: FileCenterStatusFilter;
  sortValue: FileCenterSortValue;
  sortDirection?: FileCenterSortDirection;
  partialSyncEndpointNames?: string[];
};

type SqlJsModule = Awaited<ReturnType<typeof createSqlModule>>;
type SqlDatabase = InstanceType<SqlJsModule['Database']>;

type EntryRow = {
  id: string;
  library_id: string;
  parent_id: string | null;
  type: 'folder' | 'file';
  lifecycle_state: AssetLifecycleState;
  name: string;
  file_kind: FileTypeFilter;
  display_type: string;
  modified_at: string;
  modified_at_sort: number;
  created_at: string;
  size_label: string;
  size_bytes: number;
  path: string;
  source_label: string;
  notes: string;
  last_task_text: string;
  last_task_tone: Severity;
  rating: number;
  color_label: FileCenterColorLabel;
};

type EndpointRow = {
  entry_id: string;
  order_index: number;
  name: string;
  state: string;
  tone: Severity;
  last_sync_at: string;
  endpoint_type: FileCenterEndpointType;
};

type TagRow = {
  entry_id: string;
  order_index: number;
  tag: string;
  kind: 'badge' | 'risk' | 'tag';
};

const FILE_CENTER_DB_KEY = 'mare-file-center-sqlite-v4';
let sqlModulePromise: Promise<SqlJsModule> | null = null;
let databasePromise: Promise<SqlDatabase> | null = null;
const fileCenterListeners = new Set<() => void>();
const libraryEndpointNamesCache = new Map<string, string[]>();
const fileNameCollator = new Intl.Collator('zh-CN-u-co-pinyin', { numeric: true, sensitivity: 'base' });

export const fileCenterApi = {
  listLibraryEndpointNames(libraryId: string) {
    return [...(libraryEndpointNamesCache.get(libraryId) ?? [])];
  },

  subscribe(listener: () => void) {
    fileCenterListeners.add(listener);
    return () => {
      fileCenterListeners.delete(listener);
    };
  },

  async loadDirectory(params: FileCenterLoadParams): Promise<FileCenterDirectoryResult> {
    try {
      const query = new URLSearchParams({
        page: String(params.page),
        pageSize: String(params.pageSize),
        searchText: params.searchText,
        fileTypeFilter: params.fileTypeFilter,
        statusFilter: params.statusFilter,
        sortValue: params.sortValue,
        sortDirection: params.sortDirection ?? 'desc',
      });
      if (params.parentId) {
        query.set('parentId', params.parentId);
      }
      for (const endpointName of params.partialSyncEndpointNames ?? []) {
        query.append('partialSyncEndpointName', endpointName);
      }

      const result = await fetchFileCenterData<FileCenterDirectoryResult>(
        `/api/libraries/${params.libraryId}/browse?${query.toString()}`,
      );
      if (!Array.isArray(result.breadcrumbs) || !Array.isArray(result.items)) {
        throw new Error('file center payload shape invalid');
      }
      result.items = result.items.map(normalizeDirectoryAnnotations);
      libraryEndpointNamesCache.set(params.libraryId, [...(result.endpointNames ?? [])]);
      return result;
    } catch (error) {
      if (isTestRuntime()) {
        return loadDirectoryFromLocalDatabase(params);
      }
      throw error;
    }
  },

  async loadEntryDetail(id: string): Promise<FileCenterEntry | null> {
    try {
      const result = await fetchFileCenterData<FileCenterEntry>(`/api/file-entries/${id}`);
      if (!result || typeof result !== 'object' || !('id' in result)) {
        throw new Error('file entry payload shape invalid');
      }
      return normalizeDirectoryAnnotations(result);
    } catch (error) {
      if (isTestRuntime()) {
        return loadEntryDetailFromLocalDatabase(id);
      }
      if (error instanceof Error && error.message.includes('404')) {
        return null;
      }
      return null;
    }
  },

  async createFolder(input: {
    libraryId: string;
    parentId: string | null;
    name: string;
  }): Promise<{ message: string; item: FileCenterEntry }> {
    const result = await fetchFileCenterData<{ message: string; entry: FileCenterEntry }>(
      `/api/libraries/${input.libraryId}/directories`,
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          parentId: input.parentId,
          name: input.name,
        }),
      },
    );
    return {
      message: result.message,
      item: result.entry,
    };
  },

  async scanDirectory(input: { libraryId: string; parentId: string | null }): Promise<{ message: string; jobId?: string }> {
    return await fetchFileCenterData<{ message: string; jobId?: string }>(`/api/libraries/${input.libraryId}/scan`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        parentId: input.parentId,
      }),
    });
  },

  async uploadSelection(input: {
    libraryId: string;
    parentId: string | null;
    mode: 'files' | 'folder';
    items: FileCenterUploadItem[];
  }): Promise<{ message: string; createdCount: number }> {
    if (input.items.length === 0) {
      return { message: '未选择任何上传内容', createdCount: 0 };
    }

    const formData = new FormData();
    formData.set('mode', input.mode);
    if (input.parentId) {
      formData.set('parentId', input.parentId);
    }
    const manifest = input.items.map((item, index) => {
      const field = `file${index}`;
      formData.append(field, item.file, item.name);
      return {
        field,
        name: item.name,
        relativePath: item.relativePath ?? item.name,
      };
    });
    formData.set('manifest', JSON.stringify(manifest));
    return await fetchFileCenterData<{ message: string; createdCount: number }>(
      `/api/libraries/${input.libraryId}/uploads`,
      {
        method: 'POST',
        body: formData,
      },
    );
  },

  async deleteAssets(ids: string[]): Promise<{ message: string; jobId?: string }> {
    if (ids.length === 0) {
      return { message: '未找到需要删除的条目' };
    }
    return fetchFileCenterData<{ message: string; jobId?: string }>(`/api/file-entries/delete-assets`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        entryIds: ids,
      }),
    });
  },

  async deleteFromEndpoint(input: {
    entryIds: string[];
    endpointName: string;
  }): Promise<{ message: string; jobId?: string }> {
    if (input.entryIds.length === 0) {
      return { message: '当前没有可删除副本' };
    }
    return fetchFileCenterData<{ message: string; jobId?: string }>(`/api/file-entries/delete-replicas`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        entryIds: input.entryIds,
        endpointName: input.endpointName,
      }),
    });
  },

  async syncToEndpoint(input: {
    entryIds: string[];
    endpointName: string;
  }): Promise<{ message: string; jobId?: string }> {
    if (input.entryIds.length === 0) {
      return { message: '当前没有可同步内容' };
    }
    return fetchFileCenterData<{ message: string; jobId?: string }>(`/api/file-entries/replicate`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        entryIds: input.entryIds,
        endpointName: input.endpointName,
      }),
    });
  },

  async loadTagManagementSnapshot(searchText = ''): Promise<TagManagementSnapshot> {
    const query = new URLSearchParams();
    if (searchText.trim()) {
      query.set('searchText', searchText.trim());
    }
    return fetchFileCenterData<TagManagementSnapshot>(
      `/api/tags/management${query.size > 0 ? `?${query.toString()}` : ''}`,
    );
  },

  async createTagGroup(name: string): Promise<{ message: string; groupId: string }> {
    const result = await fetchFileCenterData<{ message: string; groupId: string }>('/api/tags/groups', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ name: name.trim() }),
    });
    emitFileCenterUpdate();
    return result;
  },

  async updateTagGroup(id: string, name: string): Promise<{ message: string }> {
    const result = await fetchFileCenterData<{ message: string }>(`/api/tags/groups/${id}`, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ name: name.trim() }),
    });
    emitFileCenterUpdate();
    return result;
  },

  async moveTagGroup(id: string, direction: 'up' | 'down'): Promise<{ message: string }> {
    const result = await fetchFileCenterData<{ message: string }>(`/api/tags/groups/${id}/move`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ direction }),
    });
    emitFileCenterUpdate();
    return result;
  },

  async createManagedTag(input: {
    groupId: string;
    isPinned: boolean;
    libraryIds: string[];
    name: string;
  }): Promise<{ message: string; tagId: string }> {
    const result = await fetchFileCenterData<{ message: string; tagId: string }>('/api/tags', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(input),
    });
    emitFileCenterUpdate();
    return result;
  },

  async updateManagedTag(
    id: string,
    input: {
      groupId: string;
      isPinned: boolean;
      libraryIds: string[];
      name: string;
    },
  ): Promise<{ message: string }> {
    const result = await fetchFileCenterData<{ message: string }>(`/api/tags/${id}`, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(input),
    });
    emitFileCenterUpdate();
    return result;
  },

  async moveManagedTag(id: string, direction: 'up' | 'down'): Promise<{ message: string }> {
    const result = await fetchFileCenterData<{ message: string }>(`/api/tags/${id}/move`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ direction }),
    });
    emitFileCenterUpdate();
    return result;
  },

  async mergeManagedTag(sourceId: string, targetId: string): Promise<{ message: string }> {
    const result = await fetchFileCenterData<{ message: string }>(`/api/tags/${sourceId}/merge`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ targetId }),
    });
    emitFileCenterUpdate();
    return result;
  },

  async deleteManagedTag(id: string): Promise<{ message: string }> {
    const result = await fetchFileCenterData<{ message: string }>(`/api/tags/${id}`, {
      method: 'DELETE',
    });
    emitFileCenterUpdate();
    return result;
  },

  async updateAnnotations(
    id: string,
    input: {
      rating: number;
      colorLabel: FileCenterColorLabel;
      tags: string[];
    },
  ): Promise<{ message: string }> {
    const result = await fetchFileCenterData<{ message: string }>(`/api/file-entries/${id}/annotations`, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        rating: Math.max(0, Math.min(5, input.rating)),
        colorLabel: input.colorLabel,
        tags: input.tags,
      }),
    });
    if (isTestRuntime() && shouldUseLegacyPrototypeStore()) {
      const db = await getDatabase();
      db.run(
        `UPDATE entries
         SET rating = $rating, color_label = $colorLabel
         WHERE id = $id`,
        {
          $id: id,
          $rating: Math.max(0, Math.min(5, input.rating)),
          $colorLabel: input.colorLabel,
        },
      );
      replaceEntryDisplayTagsInDatabase(db, id, input.tags);
      persistDatabase(db);
      emitFileCenterUpdate();
    }
    return result;
  },

  async replaceEntryTagsByNames(
    entryId: string,
    tags: string[],
  ): Promise<{ message: string }> {
    const detail = await fileCenterApi.loadEntryDetail(entryId);
    if (!detail) {
      throw new Error('未找到资产');
    }
    const result = await fileCenterApi.updateAnnotations(entryId, {
      rating: detail.rating,
      colorLabel: detail.colorLabel,
      tags,
    });
    emitFileCenterUpdate();
    return result;
  },

  async loadTagSuggestions(searchText = '', libraryId?: string): Promise<FileCenterTagSuggestion[]> {
    const query = new URLSearchParams();
    if (searchText.trim()) {
      query.set('searchText', searchText.trim());
    }
    if (libraryId?.trim()) {
      query.set('libraryId', libraryId.trim());
    }
    return fetchFileCenterData<FileCenterTagSuggestion[]>(
      `/api/tags/suggestions${query.size > 0 ? `?${query.toString()}` : ''}`,
    );
  },

  async refreshIndex(): Promise<{ message: string }> {
    const db = await getDatabase();
    db.run(
      "UPDATE entries SET last_task_text = '索引刷新已排队', last_task_tone = 'info' WHERE parent_id IS NULL",
    );
    persistDatabase(db);
    return { message: '已发起索引刷新' };
  },
};

export async function resetFileCenterMock() {
  databasePromise = null;
  libraryEndpointNamesCache.clear();
  if (typeof window !== 'undefined') {
    window.localStorage.removeItem(FILE_CENTER_DB_KEY);
  }
}

async function fetchFileCenterData<T>(path: string, init?: RequestInit): Promise<T> {
  const { centerBaseUrl } = getRuntimeConfig();
  const response = await fetch(`${centerBaseUrl}${path}`, init);
  if (!response.ok) {
    const payload = (await response.json().catch(() => null)) as { error?: { message?: string } } | null;
    throw new Error(payload?.error?.message ?? `center service returned status ${response.status}`);
  }

  const payload = (await response.json()) as { data: T };
  return payload.data;
}

async function loadDirectoryFromLocalDatabase(params: FileCenterLoadParams): Promise<FileCenterDirectoryResult> {
  const db = await getDatabase();
  const library = querySingle<{ id: string; name: string }>(
    db,
    'SELECT id, name FROM libraries WHERE id = $libraryId',
    { $libraryId: params.libraryId },
  );

  if (!library) {
    throw new Error('未找到指定资产库');
  }

  const breadcrumbs = buildBreadcrumbs(db, params.libraryId, params.parentId, library.name);
  const currentPathChildren =
    querySingle<{ total: number }>(
      db,
      params.parentId === null
        ? 'SELECT COUNT(*) AS total FROM entries WHERE library_id = $libraryId AND parent_id IS NULL'
        : 'SELECT COUNT(*) AS total FROM entries WHERE library_id = $libraryId AND parent_id = $parentId',
      params.parentId === null ? { $libraryId: params.libraryId } : { $libraryId: params.libraryId, $parentId: params.parentId },
    )?.total ?? 0;

  const filter = buildEntryFilter(params);
  const offset = Math.max(0, (params.page - 1) * params.pageSize);
  const rows = queryAll<EntryRow>(
    db,
    `SELECT
      id,
      library_id,
      parent_id,
      type,
      lifecycle_state,
      name,
      file_kind,
      display_type,
      modified_at,
      modified_at_sort,
      created_at,
      size_label,
      size_bytes,
      path,
      source_label,
      last_task_text,
      last_task_tone,
      rating,
      color_label
    FROM entries
    WHERE ${filter.whereClause}`,
    filter.parameters,
  );
  const normalizedRows = applyDerivedFolderCounts(db, rows);
  const sortedRows = sortEntryRowsForDisplay(normalizedRows, params.sortValue, params.sortDirection ?? 'desc');
  const hydratedItems = hydrateEntries(db, sortedRows);
  const filteredItems = filterEntriesByStatus(
    hydratedItems,
    params.statusFilter,
    params.partialSyncEndpointNames ?? [],
  );
  const pagedItems = filteredItems.slice(offset, offset + params.pageSize);
  const endpointNames = getLibraryEndpointTemplates(params.libraryId).map((endpoint) => endpoint.name);
  libraryEndpointNamesCache.set(params.libraryId, endpointNames);

  return {
    breadcrumbs,
    items: pagedItems.map(normalizeDirectoryAnnotations),
    total: filteredItems.length,
    currentPathChildren,
    endpointNames,
  };
}

async function loadEntryDetailFromLocalDatabase(id: string): Promise<FileCenterEntry | null> {
  const db = await getDatabase();
  const row = querySingle<EntryRow>(
    db,
    `SELECT
      id,
      library_id,
      parent_id,
      type,
      lifecycle_state,
      name,
      file_kind,
      display_type,
      modified_at,
      modified_at_sort,
      created_at,
      size_label,
      size_bytes,
      path,
      source_label,
      last_task_text,
      last_task_tone,
      rating,
      color_label
    FROM entries
    WHERE id = $id`,
    { $id: id },
  );

  if (!row) {
    return null;
  }

  const entry = hydrateEntries(db, [row])[0] ?? null;
  return entry ? normalizeDirectoryAnnotations(entry) : null;
}

async function createFolderFromLocalDatabase(input: {
  libraryId: string;
  parentId: string | null;
  name: string;
}): Promise<{ message: string; item: FileCenterEntry }> {
  const db = await getDatabase();
  const library = querySingle<{ name: string }>(db, 'SELECT name FROM libraries WHERE id = $libraryId', {
    $libraryId: input.libraryId,
  });
  if (!library) {
    throw new Error('未找到指定资产库');
  }

  const parent = input.parentId
    ? querySingle<{ path: string }>(db, 'SELECT path FROM entries WHERE id = $id', { $id: input.parentId })
    : null;
  const id = `folder-${Math.random().toString(36).slice(2, 10)}`;
  const path = parent ? `${parent.path} / ${input.name.trim()}` : `${library.name} / ${input.name.trim()}`;
  const now = Date.now();

  insertEntry(
    db,
    createEntryRow({
      id,
      libraryId: input.libraryId,
      parentId: input.parentId,
      type: 'folder',
      name: input.name.trim(),
      fileKind: '文件夹',
      displayType: '文件夹',
      modifiedAt: '刚刚',
      modifiedAtSort: now,
      createdAt: '刚刚',
      sizeLabel: '0 项',
      sizeBytes: 0,
      path,
      sourceLabel: '客户端新建',
      notes: '',
    }),
    '目录已创建',
    'success',
  );
  insertLocalOnlyEndpoints(db, id, input.libraryId);
  insertMetadataRows(db, id, [
    { label: '来源', value: '客户端新建' },
    { label: '路径', value: path },
    { label: '子项目', value: '0' },
    { label: '生命周期', value: 'ACTIVE' },
  ]);
  insertTagRows(db, id, [{ kind: 'badge', value: '新建目录' }]);
  persistDatabase(db);

  const item = await loadEntryDetailFromLocalDatabase(id);
  if (!item) {
    throw new Error('目录创建成功，但读取结果失败');
  }
  return { message: '目录已创建', item };
}

async function scanDirectoryFromLocalDatabase(_input: {
  libraryId: string;
  parentId: string | null;
}): Promise<{ message: string; jobId?: string }> {
  return { message: '当前目录扫描已完成' };
}

function normalizeDirectoryAnnotations<T extends FileCenterEntry>(entry: T): T {
  if (entry.type !== 'folder') {
    return entry;
  }
  return {
    ...entry,
    rating: 0,
    colorLabel: '无',
  };
}

async function deleteAssetsFromLocalDatabase(ids: string[]): Promise<{ message: string }> {
  const db = await getDatabase();
  const allIds = collectEntryIds(db, ids);
  if (allIds.length === 0) {
    return { message: '未找到需要删除的条目' };
  }

  const placeholders = createPlaceholders(allIds, 'entryId');
  db.run(
    `UPDATE entries
     SET lifecycle_state = 'PENDING_DELETE', last_task_text = '等待后台清理', last_task_tone = 'warning'
     WHERE id IN (${placeholders.sql})`,
    placeholders.parameters,
  );
  db.run(
    `UPDATE entry_endpoints
     SET state = '未同步', tone = 'critical', last_sync_at = '刚刚'
     WHERE entry_id IN (${placeholders.sql})`,
    placeholders.parameters,
  );
  syncLifecycleMetadata(db, allIds, 'PENDING_DELETE');
  persistDatabase(db);
  return { message: '删除请求已提交，资产进入等待清理' };
}

function isTestRuntime() {
  return typeof import.meta !== 'undefined' && import.meta.env?.MODE === 'test';
}

function shouldUseLegacyPrototypeStore() {
  if (typeof window === 'undefined') {
    return false;
  }
  return Boolean((window as Window & { __MARE_ENABLE_FILE_CENTER_MOCK_SYNC__?: boolean }).__MARE_ENABLE_FILE_CENTER_MOCK_SYNC__);
}

async function getSqlModule() {
  if (!sqlModulePromise) {
    sqlModulePromise = createSqlModule();
  }
  return sqlModulePromise;
}

async function createSqlModule() {
  const [{ default: initSqlJs }, { default: sqlWasmUrl }] = await Promise.all([
    import('sql.js/dist/sql-wasm-browser.js'),
    import('sql.js/dist/sql-wasm.wasm?url'),
  ]);
  return initSqlJs({
    locateFile: () => sqlWasmUrl,
  });
}

async function getDatabase() {
  if (!databasePromise) {
    databasePromise = initializeDatabase();
  }
  return databasePromise;
}

async function initializeDatabase() {
  const SQL = await getSqlModule();
  const stored = readStoredDatabase();
  if (stored) {
    const db = new SQL.Database(stored);
    if (migrateFileCenterDatabase(db)) {
      persistDatabase(db);
    }
    return db;
  }
  const db = new SQL.Database();
  createSchema(db);
  seedDatabase(db);
  migrateFileCenterDatabase(db);
  persistDatabase(db);
  return db;
}

function readStoredDatabase() {
  if (typeof window === 'undefined') {
    return null;
  }
  const raw = window.localStorage.getItem(FILE_CENTER_DB_KEY);
  if (!raw) {
    return null;
  }
  try {
    return decodeBase64(raw);
  } catch {
    window.localStorage.removeItem(FILE_CENTER_DB_KEY);
    return null;
  }
}

function persistDatabase(db: SqlDatabase) {
  if (typeof window === 'undefined') {
    return;
  }
  const payload = db.export();
  window.localStorage.setItem(FILE_CENTER_DB_KEY, encodeBase64(payload));
}

function emitFileCenterUpdate() {
  fileCenterListeners.forEach((listener) => listener());
}

function createSchema(db: SqlDatabase) {
  db.run(`
    CREATE TABLE libraries (
      id TEXT PRIMARY KEY,
      name TEXT NOT NULL
    );

    CREATE TABLE entries (
      id TEXT PRIMARY KEY,
      library_id TEXT NOT NULL,
      parent_id TEXT NULL,
      type TEXT NOT NULL,
      lifecycle_state TEXT NOT NULL,
      name TEXT NOT NULL,
      file_kind TEXT NOT NULL,
      display_type TEXT NOT NULL,
      modified_at TEXT NOT NULL,
      modified_at_sort INTEGER NOT NULL,
      created_at TEXT NOT NULL,
      size_label TEXT NOT NULL,
      size_bytes INTEGER NOT NULL,
      path TEXT NOT NULL,
      source_label TEXT NOT NULL,
      notes TEXT NOT NULL,
      last_task_text TEXT NOT NULL,
      last_task_tone TEXT NOT NULL,
      rating INTEGER NOT NULL DEFAULT 0,
      color_label TEXT NOT NULL DEFAULT '无'
    );

    CREATE TABLE entry_endpoints (
      entry_id TEXT NOT NULL,
      order_index INTEGER NOT NULL,
      name TEXT NOT NULL,
      state TEXT NOT NULL,
      tone TEXT NOT NULL,
      last_sync_at TEXT NOT NULL,
      endpoint_type TEXT NOT NULL,
      PRIMARY KEY (entry_id, name)
    );

    CREATE TABLE entry_metadata (
      entry_id TEXT NOT NULL,
      order_index INTEGER NOT NULL,
      label TEXT NOT NULL,
      value TEXT NOT NULL,
      PRIMARY KEY (entry_id, label)
    );

    CREATE TABLE entry_tags (
      entry_id TEXT NOT NULL,
      order_index INTEGER NOT NULL,
      tag TEXT NOT NULL,
      kind TEXT NOT NULL,
      PRIMARY KEY (entry_id, kind, tag)
    );

    CREATE INDEX idx_entries_library_parent ON entries(library_id, parent_id);
    CREATE INDEX idx_entries_modified ON entries(library_id, modified_at_sort DESC);
    CREATE INDEX idx_entry_endpoints_entry ON entry_endpoints(entry_id, order_index);
    CREATE INDEX idx_entry_tags_entry ON entry_tags(entry_id, order_index);
  `);
}

export function normalizeFileCenterEndpointState(state: string): FileCenterEndpointState {
  if (state === '已同步' || state === '已存在') {
    return '已同步';
  }
  if (state === '部分同步') {
    return '部分同步';
  }
  if (state === '同步中') {
    return '同步中';
  }
  return '未同步';
}

export function resolveFileCenterEndpointTone(state: string): Severity {
  const normalizedState = normalizeFileCenterEndpointState(state);
  if (normalizedState === '已同步') {
    return 'success';
  }
  if (normalizedState === '部分同步') {
    return 'warning';
  }
  if (normalizedState === '同步中') {
    return 'warning';
  }
  return 'critical';
}

export function canSyncFileCenterEndpoint(endpoint: Pick<FileCenterEndpoint, 'state'> | string) {
  const state = typeof endpoint === 'string' ? endpoint : endpoint.state;
  return ['未同步', '部分同步'].includes(normalizeFileCenterEndpointState(state));
}

export function canDeleteFileCenterEndpoint(endpoint: Pick<FileCenterEndpoint, 'state'> | string) {
  const state = typeof endpoint === 'string' ? endpoint : endpoint.state;
  return ['已同步', '部分同步'].includes(normalizeFileCenterEndpointState(state));
}

function migrateFileCenterDatabase(db: SqlDatabase) {
  const rows = queryAll<{ entry_id: string; name: string; state: string; tone: Severity }>(
    db,
    'SELECT entry_id, name, state, tone FROM entry_endpoints',
    {},
  );
  let changed = false;

  rows.forEach((row) => {
    const normalizedState = normalizeFileCenterEndpointState(row.state);
    const normalizedTone = resolveFileCenterEndpointTone(row.state);
    if (row.state === normalizedState && row.tone === normalizedTone) {
      return;
    }

    db.run(
      `UPDATE entry_endpoints
       SET state = $state, tone = $tone
       WHERE entry_id = $entryId AND name = $name`,
      {
        $entryId: row.entry_id,
        $name: row.name,
        $state: normalizedState,
        $tone: normalizedTone,
      },
    );
    changed = true;
  });

  return changed;
}

function createEntryRow(input: {
  id: string;
  libraryId: string;
  parentId: string | null;
  type: 'folder' | 'file';
  name: string;
  fileKind: FileTypeFilter;
  displayType: string;
  modifiedAt: string;
  modifiedAtSort: number;
  createdAt: string;
  sizeLabel: string;
  sizeBytes: number;
  path: string;
  sourceLabel: string;
  notes: string;
  rating?: number;
  colorLabel?: FileCenterColorLabel;
}): Omit<EntryRow, 'last_task_text' | 'last_task_tone'> {
  return {
    id: input.id,
    library_id: input.libraryId,
    parent_id: input.parentId,
    type: input.type,
    lifecycle_state: 'ACTIVE',
    name: input.name,
    file_kind: input.fileKind,
    display_type: input.displayType,
    modified_at: input.modifiedAt,
    modified_at_sort: input.modifiedAtSort,
    created_at: input.createdAt,
    size_label: input.sizeLabel,
    size_bytes: input.sizeBytes,
    path: input.path,
    source_label: input.sourceLabel,
    notes: input.notes,
    rating: input.rating ?? 0,
    color_label: input.colorLabel ?? '无',
  };
}

function insertEntry(
  db: SqlDatabase,
  row: Omit<EntryRow, 'last_task_text' | 'last_task_tone'>,
  lastTaskText: string,
  lastTaskTone: Severity,
) {
  db.run(
    `INSERT INTO entries (
      id,
      library_id,
      parent_id,
      type,
      lifecycle_state,
      name,
      file_kind,
      display_type,
      modified_at,
      modified_at_sort,
      created_at,
      size_label,
      size_bytes,
      path,
      source_label,
      notes,
      last_task_text,
      last_task_tone,
      rating,
      color_label
    ) VALUES (
      $id,
      $libraryId,
      $parentId,
      $type,
      $lifecycleState,
      $name,
      $fileKind,
      $displayType,
      $modifiedAt,
      $modifiedAtSort,
      $createdAt,
      $sizeLabel,
      $sizeBytes,
      $path,
      $sourceLabel,
      $notes,
      $lastTaskText,
      $lastTaskTone,
      $rating,
      $colorLabel
    )`,
    {
      $id: row.id,
      $libraryId: row.library_id,
      $parentId: row.parent_id,
      $type: row.type,
      $lifecycleState: row.lifecycle_state,
      $name: row.name,
      $fileKind: row.file_kind,
      $displayType: row.display_type,
      $modifiedAt: row.modified_at,
      $modifiedAtSort: row.modified_at_sort,
      $createdAt: row.created_at,
      $sizeLabel: row.size_label,
      $sizeBytes: row.size_bytes,
      $path: row.path,
      $sourceLabel: row.source_label,
      $notes: row.notes,
      $lastTaskText: lastTaskText,
      $lastTaskTone: lastTaskTone,
      $rating: row.rating,
      $colorLabel: row.color_label,
    },
  );
}

function insertMetadataRows(db: SqlDatabase, entryId: string, rows: FileCenterMetadataRow[]) {
  rows.forEach((row, orderIndex) => {
    db.run(
      `INSERT OR REPLACE INTO entry_metadata (entry_id, order_index, label, value)
       VALUES ($entryId, $orderIndex, $label, $value)`,
      {
        $entryId: entryId,
        $orderIndex: orderIndex,
        $label: row.label,
        $value: row.value,
      },
    );
  });
}

function insertTagRows(
  db: SqlDatabase,
  entryId: string,
  rows: Array<{ kind: 'badge' | 'risk' | 'tag'; value: string }>,
) {
  rows.forEach((row, orderIndex) => {
    db.run(
      `INSERT OR REPLACE INTO entry_tags (entry_id, order_index, tag, kind)
       VALUES ($entryId, $orderIndex, $tag, $kind)`,
      {
        $entryId: entryId,
        $orderIndex: orderIndex,
        $tag: row.value,
        $kind: row.kind,
      },
    );
  });
}

function createSeedEndpoint(
  name: string,
  state: FileCenterEndpointState,
  lastSyncAt: string,
  endpointType: FileCenterEndpointType,
): FileCenterEndpoint {
  return {
    name,
    state,
    tone: resolveFileCenterEndpointTone(state),
    lastSyncAt,
    endpointType,
  };
}

function getLibraryEndpointTemplates(libraryId: string): Array<Pick<FileCenterEndpoint, 'name' | 'endpointType'>> {
  return getLocalOnlyEndpoints(libraryId).map((endpoint) => ({
    name: endpoint.name,
    endpointType: endpoint.endpointType,
  }));
}

function getLocalOnlyEndpoints(libraryId: string): FileCenterEndpoint[] {
  if (libraryId === 'video') {
    return [
      createSeedEndpoint('本地NVMe', '已同步', '刚刚', 'local'),
      createSeedEndpoint('影像NAS', '未同步', '尚未开始', 'nas'),
    ];
  }
  if (libraryId === 'family') {
    return [
      createSeedEndpoint('本地NVMe', '已同步', '刚刚', 'local'),
      createSeedEndpoint('影像NAS', '未同步', '尚未开始', 'nas'),
      createSeedEndpoint('115', '未同步', '尚未开始', 'cloud'),
    ];
  }
  return [
    createSeedEndpoint('本地NVMe', '已同步', '刚刚', 'local'),
    createSeedEndpoint('影像NAS', '未同步', '尚未开始', 'nas'),
    createSeedEndpoint('115', '未同步', '尚未开始', 'cloud'),
  ];
}

function insertLocalOnlyEndpoints(db: SqlDatabase, entryId: string, libraryId: string) {
  getLocalOnlyEndpoints(libraryId).forEach((endpoint, orderIndex) => {
    db.run(
      `INSERT INTO entry_endpoints (entry_id, order_index, name, state, tone, last_sync_at, endpoint_type)
       VALUES ($entryId, $orderIndex, $name, $state, $tone, $lastSyncAt, $endpointType)`,
      {
        $entryId: entryId,
        $orderIndex: orderIndex,
        $name: endpoint.name,
        $state: endpoint.state,
        $tone: endpoint.tone,
        $lastSyncAt: endpoint.lastSyncAt,
        $endpointType: endpoint.endpointType,
      },
    );
  });
}

function seedDatabase(db: SqlDatabase) {
  const libraries = [
    { id: 'photo', name: '商业摄影资产库' },
    { id: 'video', name: '视频工作流资产库' },
    { id: 'family', name: '家庭照片资产库' },
  ];

  libraries.forEach((library) => {
    db.run('INSERT INTO libraries (id, name) VALUES ($id, $name)', {
      $id: library.id,
      $name: library.name,
    });
  });

  const rootFolders: Array<Omit<EntryRow, 'last_task_text' | 'last_task_tone'>> = [
    createEntryRow({
      id: 'photo-root-raw',
      libraryId: 'photo',
      parentId: null,
      type: 'folder',
      name: '拍摄原片',
      fileKind: '文件夹',
      displayType: '文件夹',
      modifiedAt: '今天 09:18',
      modifiedAtSort: 202604010918,
      createdAt: '2026-03-29 08:00',
      sizeLabel: '2,184 项',
      sizeBytes: 2184,
      path: '商业摄影资产库 / 拍摄原片',
      sourceLabel: '本地导入',
      notes: '商业摄影主原片目录',
    }),
    createEntryRow({
      id: 'photo-root-delivery',
      libraryId: 'photo',
      parentId: null,
      type: 'folder',
      name: '精选交付',
      fileKind: '文件夹',
      displayType: '文件夹',
      modifiedAt: '昨天 21:10',
      modifiedAtSort: 202603312110,
      createdAt: '2026-03-30 00:20',
      sizeLabel: '136 项',
      sizeBytes: 136,
      path: '商业摄影资产库 / 精选交付',
      sourceLabel: '交付整理',
      notes: '交付物目录',
    }),
    createEntryRow({
      id: 'photo-root-retouch',
      libraryId: 'photo',
      parentId: null,
      type: 'folder',
      name: '修图工程',
      fileKind: '文件夹',
      displayType: '文件夹',
      modifiedAt: '昨天 17:40',
      modifiedAtSort: 202603311740,
      createdAt: '2026-03-28 16:00',
      sizeLabel: '84 项',
      sizeBytes: 84,
      path: '商业摄影资产库 / 修图工程',
      sourceLabel: '后期工程',
      notes: 'PS / Capture One 项目',
    }),
    createEntryRow({
      id: 'photo-root-docs',
      libraryId: 'photo',
      parentId: null,
      type: 'folder',
      name: '合同文档',
      fileKind: '文件夹',
      displayType: '文件夹',
      modifiedAt: '2026-03-27 14:22',
      modifiedAtSort: 202603271422,
      createdAt: '2026-03-21 10:00',
      sizeLabel: '32 项',
      sizeBytes: 32,
      path: '商业摄影资产库 / 合同文档',
      sourceLabel: '协作资料',
      notes: '报价单、合同、交付确认',
    }),
  ];

  rootFolders.forEach((row) => insertEntry(db, row, '目录已索引', 'success'));

  for (let index = 1; index <= 42; index += 1) {
    const id = `photo-root-extra-${index}`;
    const isDocument = index % 4 === 0;
    const folderName = ['客户归档', '品牌分组', '待交付素材', '修图返修', '历史项目', '社媒素材'][index % 6];
    const documentName = ['拍摄计划', '报价单', '授权清单', '交付确认'][index % 4];
    const itemName = isDocument ? `${documentName}_${String(index).padStart(2, '0')}.pdf` : `${folderName} ${String(index).padStart(2, '0')}`;
    const taskText = isDocument
      ? index % 6 === 0
        ? '等待补齐到 115'
        : index % 5 === 0
          ? '影像NAS 同步中'
          : '文档索引已更新'
      : index % 7 === 0
        ? '目录待复核'
        : '目录已索引';
    const taskTone = isDocument
      ? index % 6 === 0
        ? 'warning'
        : index % 5 === 0
          ? 'warning'
          : 'success'
      : index % 7 === 0
        ? 'info'
        : 'success';

    insertEntry(
      db,
      createEntryRow({
        id,
        libraryId: 'photo',
        parentId: null,
        type: isDocument ? 'file' : 'folder',
        name: itemName,
        fileKind: isDocument ? '文档' : '文件夹',
        displayType: isDocument ? 'PDF 文档' : '文件夹',
        modifiedAt: `2026-03-${String((index % 9) + 20).padStart(2, '0')} 1${index % 10}:00`,
        modifiedAtSort: Number(`202603${String((index % 9) + 20).padStart(2, '0')}1${index % 10}00`),
        createdAt: `2026-03-${String((index % 9) + 12).padStart(2, '0')} 09:00`,
        sizeLabel: isDocument ? `${(index * 1.4 + 2).toFixed(1)} MB` : `${index * 7 + 12} 项`,
        sizeBytes: isDocument ? Math.round((index * 1.4 + 2) * 1024 * 1024) : index * 7 + 12,
        path:
          isDocument
            ? `商业摄影资产库 / 合同文档 / ${itemName}`
            : `商业摄影资产库 / ${itemName}`,
        sourceLabel: isDocument ? '商务资料' : '自动归档',
        notes: '',
      }),
      taskText,
      taskTone,
    );

    if (isDocument) {
      const endpoints = [
        createSeedEndpoint('本地NVMe', '已同步', '今天 08:40', 'local'),
        createSeedEndpoint(
          '影像NAS',
          index % 5 === 0 ? '同步中' : '已同步',
          index % 5 === 0 ? '刚刚' : '今天 08:48',
          'nas',
        ),
        createSeedEndpoint(
          '115',
          index % 6 === 0 ? '未同步' : '已同步',
          index % 6 === 0 ? '尚未开始' : '昨天 22:16',
          'cloud',
        ),
      ];

      endpoints.forEach((endpoint, orderIndex) => {
        db.run(
          `INSERT INTO entry_endpoints (entry_id, order_index, name, state, tone, last_sync_at, endpoint_type)
           VALUES ($entryId, $orderIndex, $name, $state, $tone, $lastSyncAt, $endpointType)`,
          {
            $entryId: id,
            $orderIndex: orderIndex,
            $name: endpoint.name,
            $state: endpoint.state,
            $tone: endpoint.tone,
            $lastSyncAt: endpoint.lastSyncAt,
            $endpointType: endpoint.endpointType,
          },
        );
      });

      insertMetadataRows(db, id, [
        { label: '文档类型', value: documentName },
        { label: '归档分组', value: folderName },
        { label: '项目', value: index % 2 === 0 ? '上海发布会' : '品牌棚拍' },
        { label: '生命周期', value: 'ACTIVE' },
      ]);
      insertTagRows(db, id, [
        { kind: 'badge', value: documentName },
        { kind: 'tag', value: index % 2 === 0 ? '商务资料' : '归档资料' },
        ...(index % 6 === 0 ? [{ kind: 'risk' as const, value: '待同步' }] : []),
      ]);
      continue;
    }

    insertMetadataRows(db, id, [
      { label: '分组类型', value: folderName },
      { label: '来源', value: '自动归档' },
      { label: '项目数', value: String(index * 7 + 12) },
      { label: '生命周期', value: 'ACTIVE' },
    ]);
    insertTagRows(db, id, [
      { kind: 'badge', value: '归档目录' },
      { kind: 'tag', value: index % 3 === 0 ? '待整理' : '已归档' },
      ...(index % 7 === 0 ? [{ kind: 'risk' as const, value: '待复核' }] : []),
    ]);
  }

  insertSeedDetails(db);
}

function insertSeedDetails(db: SqlDatabase) {
  const seedEntries: Array<{
    row: Omit<EntryRow, 'last_task_text' | 'last_task_tone'>;
    taskText: string;
    taskTone: Severity;
    endpoints: FileCenterEndpoint[];
    metadata: FileCenterMetadataRow[];
    tags?: Array<{ kind: 'badge' | 'risk' | 'tag'; value: string }>;
  }> = [
    {
      row: createEntryRow({
        id: 'photo-file-raw-001',
        libraryId: 'photo',
        parentId: 'photo-root-raw',
        type: 'file',
        name: '2026-03-29_上海发布会_A-cam_001.RAW',
        fileKind: '图片',
        displayType: 'RAW 图像',
        modifiedAt: '2026-03-29 18:42',
        modifiedAtSort: 202603291842,
        createdAt: '2026-03-29 18:42',
        sizeLabel: '48.2 MB',
        sizeBytes: 50541363,
        path: '商业摄影资产库 / 拍摄原片 / 2026-03-29_上海发布会_A-cam_001.RAW',
        sourceLabel: 'Sony A7R V',
        notes: '主机位原片',
        rating: 4,
        colorLabel: '红标',
      }),
      taskText: '等待补齐到 115',
      taskTone: 'warning',
      endpoints: [
        createSeedEndpoint('本地NVMe', '已同步', '今天 09:18', 'local'),
        createSeedEndpoint('影像NAS', '已同步', '今天 09:18', 'nas'),
        createSeedEndpoint('115', '未同步', '尚未开始', 'cloud'),
      ],
      metadata: [
        { label: '设备', value: 'Sony A7R V' },
        { label: '镜头', value: '24-70mm F2.8 GM II' },
        { label: '分辨率', value: '9504 × 6336' },
        { label: '曝光', value: '1/250 · f/2.8 · ISO 640' },
        { label: '生命周期', value: 'ACTIVE' },
      ],
      tags: [
        { kind: 'badge', value: 'RAW' },
        { kind: 'badge', value: '主机位' },
        { kind: 'tag', value: '发布会' },
        { kind: 'tag', value: '社媒候选' },
      ],
    },
    {
      row: createEntryRow({
        id: 'photo-file-raw-002',
        libraryId: 'photo',
        parentId: 'photo-root-raw',
        type: 'file',
        name: '2026-03-29_上海发布会_B-cam_018.RAW',
        fileKind: '图片',
        displayType: 'RAW 图像',
        modifiedAt: '2026-03-29 18:49',
        modifiedAtSort: 202603291849,
        createdAt: '2026-03-29 18:49',
        sizeLabel: '47.8 MB',
        sizeBytes: 50121984,
        path: '商业摄影资产库 / 拍摄原片 / 2026-03-29_上海发布会_B-cam_018.RAW',
        sourceLabel: 'Sony A7 IV',
        notes: '副机位原片',
        colorLabel: '无',
      }),
      taskText: 'NAS 补齐进行中',
      taskTone: 'warning',
      endpoints: [
        createSeedEndpoint('本地NVMe', '已同步', '今天 09:18', 'local'),
        createSeedEndpoint('影像NAS', '同步中', '刚刚', 'nas'),
        createSeedEndpoint('115', '未同步', '尚未开始', 'cloud'),
      ],
      metadata: [
        { label: '设备', value: 'Sony A7 IV' },
        { label: '镜头', value: '70-200mm F2.8 GM II' },
        { label: '分辨率', value: '7008 × 4672' },
        { label: '曝光', value: '1/200 · f/2.8 · ISO 800' },
        { label: '生命周期', value: 'ACTIVE' },
      ],
      tags: [
        { kind: 'badge', value: 'RAW' },
        { kind: 'risk', value: '待同步' },
        { kind: 'tag', value: '发布会' },
        { kind: 'tag', value: '待修图' },
      ],
    },
    {
      row: createEntryRow({
        id: 'photo-file-cover',
        libraryId: 'photo',
        parentId: 'photo-root-delivery',
        type: 'file',
        name: '上海发布会_精选封面.jpg',
        fileKind: '图片',
        displayType: 'JPEG 图像',
        modifiedAt: '2026-03-30 00:24',
        modifiedAtSort: 202603300024,
        createdAt: '2026-03-30 00:24',
        sizeLabel: '12.4 MB',
        sizeBytes: 13002342,
        path: '商业摄影资产库 / 精选交付 / 上海发布会_精选封面.jpg',
        sourceLabel: '精选交付',
        notes: '首页封面图',
        rating: 5,
        colorLabel: '黄标',
      }),
      taskText: '已同步到全部端点',
      taskTone: 'success',
      endpoints: [
        createSeedEndpoint('本地NVMe', '已同步', '昨天 22:18', 'local'),
        createSeedEndpoint('影像NAS', '已同步', '昨天 22:20', 'nas'),
        createSeedEndpoint('115', '已同步', '昨天 22:28', 'cloud'),
      ],
      metadata: [
        { label: '设备', value: 'Sony A7R V' },
        { label: '分辨率', value: '4096 × 2731' },
        { label: '评级', value: '五星' },
        { label: '颜色标记', value: '黄色' },
        { label: '生命周期', value: 'ACTIVE' },
      ],
      tags: [
        { kind: 'badge', value: '精选交付' },
        { kind: 'tag', value: '封面图' },
        { kind: 'tag', value: '客户精选' },
      ],
    },
    {
      row: createEntryRow({
        id: 'video-file-final',
        libraryId: 'video',
        parentId: null,
        type: 'file',
        name: '客户访谈_第一机位_精编版.mov',
        fileKind: '视频',
        displayType: 'ProRes 视频',
        modifiedAt: '2026-03-30 22:14',
        modifiedAtSort: 202603302214,
        createdAt: '2026-03-30 18:00',
        sizeLabel: '12.8 GB',
        sizeBytes: 13743895347,
        path: '视频工作流资产库 / 客户访谈_第一机位_精编版.mov',
        sourceLabel: '剪辑导出',
        notes: '待归档母版',
        rating: 3,
        colorLabel: '蓝标',
      }),
      taskText: '等待补齐到 115',
      taskTone: 'warning',
      endpoints: [
        createSeedEndpoint('本地NVMe', '已同步', '今天 10:02', 'local'),
        createSeedEndpoint('影像NAS', '同步中', '刚刚', 'nas'),
        createSeedEndpoint('115', '未同步', '尚未开始', 'cloud'),
      ],
      metadata: [
        { label: '分辨率', value: '3840 × 2160' },
        { label: '帧率', value: '25 fps' },
        { label: '时长', value: '23 分 14 秒' },
        { label: '编码', value: 'Apple ProRes 422 HQ' },
        { label: '生命周期', value: 'ACTIVE' },
      ],
      tags: [
        { kind: 'risk', value: '待同步' },
        { kind: 'badge', value: '母版' },
        { kind: 'tag', value: '访谈项目' },
      ],
    },
    {
      row: createEntryRow({
        id: 'video-file-audio',
        libraryId: 'video',
        parentId: null,
        type: 'file',
        name: '片头配乐_v4_master.wav',
        fileKind: '音频',
        displayType: 'WAV 音频',
        modifiedAt: '2026-03-28 10:05',
        modifiedAtSort: 202603281005,
        createdAt: '2026-03-28 09:40',
        sizeLabel: '148 MB',
        sizeBytes: 155189248,
        path: '视频工作流资产库 / 片头配乐_v4_master.wav',
        sourceLabel: '音频母版',
        notes: 'NAS 校验失败待修复',
        rating: 2,
        colorLabel: '绿标',
      }),
      taskText: 'NAS 复核待处理',
      taskTone: 'critical',
      endpoints: [
        createSeedEndpoint('本地NVMe', '已同步', '2026-03-28 10:06', 'local'),
        createSeedEndpoint('影像NAS', '未同步', '2026-03-28 10:12', 'nas'),
        createSeedEndpoint('115', '已同步', '2026-03-28 10:20', 'cloud'),
      ],
      metadata: [
        { label: '采样率', value: '48 kHz' },
        { label: '位深', value: '24 bit' },
        { label: '时长', value: '3 分 42 秒' },
        { label: '声道', value: '立体声' },
        { label: '生命周期', value: 'ACTIVE' },
      ],
      tags: [
        { kind: 'risk', value: '待复核' },
        { kind: 'badge', value: '音频母版' },
        { kind: 'tag', value: '访谈项目' },
      ],
    },
  ];

  seedEntries.forEach((item) => {
    insertEntry(db, item.row, item.taskText, item.taskTone);
    item.endpoints.forEach((endpoint, orderIndex) => {
      db.run(
        `INSERT INTO entry_endpoints (entry_id, order_index, name, state, tone, last_sync_at, endpoint_type)
         VALUES ($entryId, $orderIndex, $name, $state, $tone, $lastSyncAt, $endpointType)`,
        {
          $entryId: item.row.id,
          $orderIndex: orderIndex,
          $name: endpoint.name,
          $state: endpoint.state,
          $tone: endpoint.tone,
          $lastSyncAt: endpoint.lastSyncAt,
          $endpointType: endpoint.endpointType,
        },
      );
    });
    insertMetadataRows(db, item.row.id, item.metadata);
    insertTagRows(db, item.row.id, item.tags ?? []);
  });

  for (let index = 3; index <= 64; index += 1) {
    const id = `photo-file-raw-${String(index).padStart(3, '0')}`;
    const camera = index % 2 === 0 ? 'Sony A7R V' : 'Sony A7 IV';
    const isNasSyncing = index % 7 === 0;
    const isCloudPending = index % 6 === 0;
    const isClientSelect = index % 10 === 0;
    const needsRetouch = index % 7 === 0;
    const exposure = index % 2 === 0 ? '1/250 · f/2.8 · ISO 640' : '1/200 · f/2.8 · ISO 800';

    insertEntry(
      db,
      createEntryRow({
        id,
        libraryId: 'photo',
        parentId: 'photo-root-raw',
        type: 'file',
        name: `2026-03-29_上海发布会_A-cam_${String(index).padStart(3, '0')}.RAW`,
        fileKind: '图片',
        displayType: 'RAW 图像',
        modifiedAt: `2026-03-29 18:${String((index + 12) % 60).padStart(2, '0')}`,
        modifiedAtSort: Number(`2026032918${String((index + 12) % 60).padStart(2, '0')}`),
        createdAt: `2026-03-29 18:${String((index + 12) % 60).padStart(2, '0')}`,
        sizeLabel: `${(46 + (index % 5) * 0.8).toFixed(1)} MB`,
        sizeBytes: Math.round((46 + (index % 5) * 0.8) * 1024 * 1024),
        path: `商业摄影资产库 / 拍摄原片 / 2026-03-29_上海发布会_A-cam_${String(index).padStart(3, '0')}.RAW`,
        sourceLabel: camera,
        notes: '',
        rating: index % 9 === 0 ? 5 : index % 5 === 0 ? 3 : 0,
        colorLabel:
          index % 11 === 0
            ? '紫标'
            : index % 8 === 0
              ? '蓝标'
              : index % 6 === 0
                ? '黄标'
                : '无',
      }),
      isNasSyncing ? '影像NAS 补齐进行中' : isCloudPending ? '等待补齐到 115' : '索引已更新',
      isNasSyncing || isCloudPending ? 'warning' : 'success',
    );
    [
      createSeedEndpoint('本地NVMe', '已同步', '今天 09:18', 'local'),
      createSeedEndpoint('影像NAS', isNasSyncing ? '同步中' : '已同步', isNasSyncing ? '刚刚' : '今天 09:20', 'nas'),
      createSeedEndpoint('115', isCloudPending ? '未同步' : '已同步', isCloudPending ? '尚未开始' : '昨天 23:12', 'cloud'),
    ].forEach((endpoint, orderIndex) => {
      db.run(
        `INSERT INTO entry_endpoints (entry_id, order_index, name, state, tone, last_sync_at, endpoint_type)
         VALUES ($entryId, $orderIndex, $name, $state, $tone, $lastSyncAt, $endpointType)`,
        {
          $entryId: id,
          $orderIndex: orderIndex,
          $name: endpoint.name,
          $state: endpoint.state,
          $tone: endpoint.tone,
          $lastSyncAt: endpoint.lastSyncAt,
          $endpointType: endpoint.endpointType,
        },
      );
    });
    insertMetadataRows(db, id, [
      { label: '设备', value: camera },
      { label: '镜头', value: index % 2 === 0 ? '24-70mm F2.8 GM II' : '70-200mm F2.8 GM II' },
      { label: '分辨率', value: index % 2 === 0 ? '9504 × 6336' : '7008 × 4672' },
      { label: '曝光', value: exposure },
      { label: '生命周期', value: 'ACTIVE' },
    ]);
    insertTagRows(db, id, [
      { kind: 'badge', value: 'RAW' },
      ...(index % 2 === 0 ? [{ kind: 'badge' as const, value: '主机位' }] : [{ kind: 'badge' as const, value: '副机位' }]),
      { kind: 'tag', value: '发布会' },
      ...(isClientSelect ? [{ kind: 'tag' as const, value: '客户精选' }] : []),
      ...(needsRetouch ? [{ kind: 'tag' as const, value: '待修图' }] : []),
      ...(index % 8 === 0 ? [{ kind: 'tag' as const, value: '社媒候选' }] : []),
      ...(isNasSyncing ? [{ kind: 'risk' as const, value: '同步中' }] : []),
      ...(isCloudPending ? [{ kind: 'risk' as const, value: '待同步' }] : []),
    ]);
  }
}

function buildEntryFilter(params: FileCenterLoadParams) {
  const where: string[] = ['library_id = $libraryId'];
  const parameters: Record<string, string | number | null> = {
    $libraryId: params.libraryId,
  };

  if (params.parentId === null) {
    where.push('parent_id IS NULL');
  } else {
    where.push('parent_id = $parentId');
    parameters.$parentId = params.parentId;
  }

  const keyword = params.searchText.trim();
  if (keyword) {
    where.push('(name LIKE $keyword OR path LIKE $keyword OR display_type LIKE $keyword OR source_label LIKE $keyword)');
    parameters.$keyword = `%${keyword}%`;
  }

  if (params.fileTypeFilter !== '全部') {
    if (params.fileTypeFilter === '文件夹') {
      where.push("type = 'folder'");
    } else {
      where.push('file_kind = $fileKind');
      parameters.$fileKind = params.fileTypeFilter;
    }
  }

  return {
    whereClause: where.join(' AND '),
    parameters,
  };
}

function buildBreadcrumbs(
  db: SqlDatabase,
  libraryId: string,
  parentId: string | null,
  libraryName: string,
) {
  const breadcrumbs: Array<{ id: string | null; label: string }> = [{ id: null, label: libraryName }];
  const chain: Array<{ id: string; label: string }> = [];
  let cursorId = parentId;

  while (cursorId) {
    const row = querySingle<{ id: string; name: string; parent_id: string | null }>(
      db,
      'SELECT id, name, parent_id FROM entries WHERE id = $id AND library_id = $libraryId',
      { $id: cursorId, $libraryId: libraryId },
    );
    if (!row) {
      break;
    }
    chain.unshift({ id: row.id, label: row.name });
    cursorId = row.parent_id;
  }

  chain.forEach((item) => breadcrumbs.push(item));
  return breadcrumbs;
}

function hydrateEntries(db: SqlDatabase, rows: EntryRow[]): FileCenterEntry[] {
  if (rows.length === 0) {
    return [];
  }

  const ids = rows.map((row) => row.id);
  const placeholders = createPlaceholders(ids, 'rowId');
  const endpointRows = queryAll<EndpointRow>(
    db,
    `SELECT entry_id, order_index, name, state, tone, last_sync_at, endpoint_type
     FROM entry_endpoints
     WHERE entry_id IN (${placeholders.sql})
     ORDER BY entry_id ASC, order_index ASC`,
    placeholders.parameters,
  );
  const tagRows = queryAll<TagRow>(
    db,
    `SELECT entry_id, order_index, tag, kind
     FROM entry_tags
     WHERE entry_id IN (${placeholders.sql})
     ORDER BY entry_id ASC, order_index ASC`,
    placeholders.parameters,
  );
  const endpointMap = groupBy(endpointRows, (row) => row.entry_id);
  const tagMap = groupBy(tagRows, (row) => row.entry_id);
  const folderEndpointStateCache = new Map<string, FileCenterEndpointState>();

  return rows.map((row) => {
    const tags = tagMap.get(row.id) ?? [];
    const endpoints =
      row.type === 'folder'
        ? deriveFolderEndpoints(db, row.id, row.library_id, folderEndpointStateCache)
        : (endpointMap.get(row.id) ?? []).map((item) => ({
            name: item.name,
            state: normalizeFileCenterEndpointState(item.state),
            tone: resolveFileCenterEndpointTone(item.state),
            lastSyncAt: item.last_sync_at,
            endpointType: item.endpoint_type,
          }));
    return {
      id: row.id,
      libraryId: row.library_id,
      parentId: row.parent_id,
      type: row.type,
      lifecycleState: row.lifecycle_state,
      name: row.name,
      fileKind: row.file_kind,
      displayType: row.display_type,
      modifiedAt: row.modified_at,
      createdAt: row.created_at,
      size: row.size_label,
      path: row.path,
      sourceLabel: row.source_label,
      lastTaskText: row.last_task_text,
      lastTaskTone: row.last_task_tone,
      rating: Number(row.rating),
      colorLabel: row.color_label,
      badges: tags.filter((item) => item.kind === 'badge').map((item) => item.tag),
      riskTags: tags.filter((item) => item.kind === 'risk').map((item) => item.tag),
      tags: tags.filter((item) => item.kind === 'tag').map((item) => item.tag),
      endpoints,
    };
  });
}

function deriveFolderEndpoints(
  db: SqlDatabase,
  folderId: string,
  libraryId: string,
  cache: Map<string, FileCenterEndpointState>,
): FileCenterEndpoint[] {
  return getLibraryEndpointTemplates(libraryId).map((endpoint) => {
    const state = deriveFolderEndpointState(db, folderId, endpoint.name, libraryId, cache);
    return {
      name: endpoint.name,
      state,
      tone: resolveFileCenterEndpointTone(state),
      lastSyncAt: '按内容派生',
      endpointType: endpoint.endpointType,
    };
  });
}

function deriveFolderEndpointState(
  db: SqlDatabase,
  folderId: string,
  endpointName: string,
  libraryId: string,
  cache: Map<string, FileCenterEndpointState>,
): FileCenterEndpointState {
  const cacheKey = `${folderId}:${endpointName}`;
  const cached = cache.get(cacheKey);
  if (cached) {
    return cached;
  }

  const children = queryAll<{ id: string; type: 'folder' | 'file' }>(
    db,
    'SELECT id, type FROM entries WHERE parent_id = $parentId ORDER BY type ASC, id ASC',
    { $parentId: folderId },
  );

  if (children.length === 0) {
    cache.set(cacheKey, '未同步');
    return '未同步';
  }

  const childStates = children.map((child) => {
    if (child.type === 'folder') {
      return deriveFolderEndpointState(db, child.id, endpointName, libraryId, cache);
    }

    const endpoint = querySingle<{ state: string }>(
      db,
      'SELECT state FROM entry_endpoints WHERE entry_id = $id AND name = $endpointName',
      { $id: child.id, $endpointName: endpointName },
    );
    return normalizeFileCenterEndpointState(endpoint?.state ?? '未同步');
  });

  let state: FileCenterEndpointState;
  if (childStates.some((childState) => childState === '同步中')) {
    state = '未同步';
  } else if (childStates.every((childState) => childState === '已同步')) {
    state = '已同步';
  } else if (childStates.some((childState) => childState === '已同步' || childState === '部分同步')) {
    state = '部分同步';
  } else {
    state = '未同步';
  }

  cache.set(cacheKey, state);
  return state;
}

function applyDerivedFolderCounts(db: SqlDatabase, rows: EntryRow[]) {
  const folderIds = rows.filter((row) => row.type === 'folder').map((row) => row.id);
  if (folderIds.length === 0) {
    return rows;
  }

  const placeholders = createPlaceholders(folderIds, 'folderId');
  const childCounts = queryAll<{ parent_id: string; total: number }>(
    db,
    `SELECT parent_id, COUNT(*) AS total
     FROM entries
     WHERE parent_id IN (${placeholders.sql})
     GROUP BY parent_id`,
    placeholders.parameters,
  );
  const childCountMap = new Map(childCounts.map((row) => [row.parent_id, Number(row.total)]));

  return rows.map((row) =>
    row.type === 'folder'
      ? {
          ...row,
          size_label: `${childCountMap.get(row.id) ?? 0} 项`,
          size_bytes: childCountMap.get(row.id) ?? 0,
        }
      : row,
  );
}

function sortEntryRows(
  rows: EntryRow[],
  sortValue: FileCenterSortValue,
  sortDirection: FileCenterSortDirection,
) {
  const direction = sortDirection === 'asc' ? 1 : -1;

  return [...rows].sort((left, right) => {
    if (left.type !== right.type) {
      return left.type === 'folder' ? -1 : 1;
    }

    if (sortValue === '名称') {
      const compared = fileNameCollator.compare(left.name, right.name) * direction;
      if (compared !== 0) {
        return compared;
      }
      return right.modified_at_sort - left.modified_at_sort;
    }

    if (sortValue === '大小') {
      const compared = (left.size_bytes - right.size_bytes) * direction;
      if (compared !== 0) {
        return compared;
      }
      return right.modified_at_sort - left.modified_at_sort;
    }

    if (sortValue === '星级') {
      const compared = (left.rating - right.rating) * direction;
      if (compared !== 0) {
        return compared;
      }
      return right.modified_at_sort - left.modified_at_sort;
    }

    const compared = (left.modified_at_sort - right.modified_at_sort) * direction;
    if (compared !== 0) {
      return compared;
    }
    return fileNameCollator.compare(left.name, right.name);
  });
}

function sortEntryRowsForDisplay(
  rows: EntryRow[],
  sortValue: FileCenterSortValue,
  sortDirection: FileCenterSortDirection,
) {
  if (sortValue !== '星级') {
    return sortEntryRows(rows, sortValue, sortDirection);
  }

  const direction = sortDirection === 'asc' ? 1 : -1;
  return [...rows].sort((left, right) => {
    if (left.type !== right.type) {
      return left.type === 'folder' ? -1 : 1;
    }
    if (left.type === 'folder') {
      return 0;
    }

    const compared = (left.rating - right.rating) * direction;
    if (compared !== 0) {
      return compared;
    }
    return right.modified_at_sort - left.modified_at_sort;
  });
}

function filterEntriesByStatus(
  entries: FileCenterEntry[],
  statusFilter: FileCenterStatusFilter,
  partialSyncEndpointNames: string[],
) {
  if (statusFilter === '全部') {
    return entries;
  }

  return entries.filter((entry) => {
    const normalizedStates = entry.endpoints.map((endpoint) => normalizeFileCenterEndpointState(endpoint.state));
    const partialEndpointNames = entry.endpoints
      .filter((endpoint) => normalizeFileCenterEndpointState(endpoint.state) === '部分同步')
      .map((endpoint) => endpoint.name);
    const syncedEndpointNames = entry.endpoints
      .filter((endpoint) => normalizeFileCenterEndpointState(endpoint.state) === '已同步')
      .map((endpoint) => endpoint.name);
    const isFullySynced = normalizedStates.length > 0 && normalizedStates.every((state) => state === '已同步');
    const isPartiallySynced =
      partialEndpointNames.length > 0 &&
      !isFullySynced;
    const isUnsynced = syncedEndpointNames.length === 0;

    if (statusFilter === '完全同步') {
      return isFullySynced;
    }

    if (statusFilter === '未同步') {
      return isUnsynced;
    }

    if (!isPartiallySynced) {
      return false;
    }

    if (partialSyncEndpointNames.length === 0) {
      return true;
    }

    return partialSyncEndpointNames.every((endpointName) => partialEndpointNames.includes(endpointName));
  });
}

const FILE_CENTER_LEGACY_QUERY_REFERENCES = [
  buildEntryFilter,
  buildBreadcrumbs,
  hydrateEntries,
  applyDerivedFolderCounts,
  sortEntryRows,
  sortEntryRowsForDisplay,
  filterEntriesByStatus,
  createFolderFromLocalDatabase,
  scanDirectoryFromLocalDatabase,
  deleteAssetsFromLocalDatabase,
  replaceEntryDisplayTagsInDatabase,
  formatUploadSize,
  formatRecentTimeLabel,
  formatDetailedTimestamp,
  inferFileKindFromName,
  inferDisplayTypeFromName,
];
void FILE_CENTER_LEGACY_QUERY_REFERENCES;

export const __FILE_CENTER_TESTING__ = {
  sortEntryRowsForDisplay,
};

function replaceEntryDisplayTagsInDatabase(db: SqlDatabase, entryId: string, names: string[]) {
  const uniqueNames = Array.from(new Set(names.map((name) => name.trim()).filter(Boolean)));
  db.run("DELETE FROM entry_tags WHERE entry_id = $entryId AND kind = 'tag'", { $entryId: entryId });
  uniqueNames.forEach((name, index) => {
    db.run(
      `INSERT INTO entry_tags (entry_id, order_index, tag, kind)
       VALUES ($entryId, $orderIndex, $tag, 'tag')`,
      {
        $entryId: entryId,
        $orderIndex: index,
        $tag: name,
      },
    );
  });
}

function syncLifecycleMetadata(db: SqlDatabase, ids: string[], lifecycleState: AssetLifecycleState) {
  ids.forEach((id) => {
    const existing = querySingle<{ total: number }>(
      db,
      "SELECT COUNT(*) AS total FROM entry_metadata WHERE entry_id = $id AND label = '生命周期'",
      { $id: id },
    )?.total;
    if (existing) {
      db.run(
        "UPDATE entry_metadata SET value = $value WHERE entry_id = $id AND label = '生命周期'",
        { $id: id, $value: lifecycleState },
      );
      return;
    }
    db.run(
      `INSERT INTO entry_metadata (entry_id, order_index, label, value)
       VALUES ($id, 999, '生命周期', $value)`,
      { $id: id, $value: lifecycleState },
    );
  });
}

function collectEntryIds(db: SqlDatabase, ids: string[]) {
  const visited = new Set<string>();
  const queue = [...ids];
  while (queue.length > 0) {
    const current = queue.shift();
    if (!current || visited.has(current)) {
      continue;
    }
    visited.add(current);
    const children = queryAll<{ id: string }>(db, 'SELECT id FROM entries WHERE parent_id = $parentId', {
      $parentId: current,
    });
    children.forEach((child) => queue.push(child.id));
  }
  return Array.from(visited);
}

function createPlaceholders(values: string[], prefix: string) {
  const parameters: Record<string, string> = {};
  const keys = values.map((value, index) => {
    const key = `$${prefix}${index}`;
    parameters[key] = value;
    return key;
  });
  return {
    sql: keys.join(', '),
    parameters,
  };
}

function querySingle<T extends Record<string, unknown>>(
  db: SqlDatabase,
  sql: string,
  params: Record<string, string | number | null>,
): T | null {
  return queryAll<T>(db, sql, params)[0] ?? null;
}

function queryAll<T extends Record<string, unknown>>(
  db: SqlDatabase,
  sql: string,
  params: Record<string, string | number | null>,
) : T[] {
  const result = db.exec(sql, params);
  if (result.length === 0) {
    return [] as T[];
  }
  const [{ columns, values }] = result as Array<{ columns: string[]; values: unknown[][] }>;
  return values.map((valueRow: unknown[]) =>
    Object.fromEntries(columns.map((column, index) => [column, valueRow[index]])) as T,
  );
}

function groupBy<T>(rows: T[], keySelector: (row: T) => string): Map<string, T[]> {
  const map = new Map<string, T[]>();
  rows.forEach((row) => {
    const key = keySelector(row);
    const current = map.get(key);
    if (current) {
      current.push(row);
      return;
    }
    map.set(key, [row]);
  });
  return map;
}

function encodeBase64(bytes: Uint8Array) {
  let binary = '';
  const chunkSize = 0x8000;
  for (let index = 0; index < bytes.length; index += chunkSize) {
    binary += String.fromCharCode(...bytes.subarray(index, index + chunkSize));
  }
  return btoa(binary);
}

function decodeBase64(base64: string) {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index);
  }
  return bytes;
}

function inferFileKindFromName(name: string): FileTypeFilter {
  const extension = name.toLowerCase().split('.').pop() ?? '';
  if (['jpg', 'jpeg', 'png', 'gif', 'webp', 'heic', 'raw', 'arw', 'cr2', 'dng'].includes(extension)) {
    return '图片';
  }
  if (['mp4', 'mov', 'mkv', 'avi', 'mxf'].includes(extension)) {
    return '视频';
  }
  if (['wav', 'mp3', 'aac', 'flac', 'm4a'].includes(extension)) {
    return '音频';
  }
  return '文档';
}

function inferDisplayTypeFromName(name: string, fileKind: FileTypeFilter) {
  const extension = name.toLowerCase().split('.').pop() ?? '';
  if (fileKind === '图片') {
    if (['raw', 'arw', 'cr2', 'dng'].includes(extension)) {
      return 'RAW 图像';
    }
    return `${extension.toUpperCase() || '图片'} 图像`;
  }
  if (fileKind === '视频') {
    return `${extension.toUpperCase() || '视频'} 视频`;
  }
  if (fileKind === '音频') {
    return `${extension.toUpperCase() || '音频'} 音频`;
  }
  return `${extension.toUpperCase() || '文档'} 文档`;
}

function formatUploadSize(size: number) {
  if (size >= 1024 * 1024 * 1024) {
    return `${(size / (1024 * 1024 * 1024)).toFixed(1)} GB`;
  }
  if (size >= 1024 * 1024) {
    return `${(size / (1024 * 1024)).toFixed(1)} MB`;
  }
  if (size >= 1024) {
    return `${Math.max(1, Math.round(size / 1024))} KB`;
  }
  return `${size} B`;
}

function formatRecentTimeLabel(timestamp: number) {
  const date = new Date(timestamp);
  const now = new Date();
  const hh = String(date.getHours()).padStart(2, '0');
  const mm = String(date.getMinutes()).padStart(2, '0');

  if (
    date.getFullYear() === now.getFullYear() &&
    date.getMonth() === now.getMonth() &&
    date.getDate() === now.getDate()
  ) {
    return `今天 ${hh}:${mm}`;
  }

  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${hh}:${mm}`;
}

function formatDetailedTimestamp(timestamp: number) {
  const date = new Date(timestamp);
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}:${String(date.getSeconds()).padStart(2, '0')}`;
}
