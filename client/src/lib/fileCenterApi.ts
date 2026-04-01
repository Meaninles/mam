import initSqlJs from 'sql.js/dist/sql-wasm-browser.js';
import sqlWasmUrl from 'sql.js/dist/sql-wasm.wasm?url';
import type { AssetLifecycleState, FileTypeFilter, Severity } from '../data';

export type FileCenterStatusFilter = '全部' | '待同步' | '有异常' | '多端齐全' | '待清理';
export type FileCenterSortValue = '修改时间' | '名称' | '大小';
export type FileCenterEndpointType = 'local' | 'nas' | 'cloud' | 'removable';
export type FileCenterColorLabel = '无' | '红标' | '黄标' | '绿标' | '蓝标' | '紫标';

export type FileCenterEndpoint = {
  name: string;
  state: string;
  tone: Severity;
  lastSyncAt: string;
  endpointType: FileCenterEndpointType;
};

export type FileCenterMetadataRow = {
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
  notes: string;
  lastTaskText: string;
  lastTaskTone: Severity;
  rating: number;
  colorLabel: FileCenterColorLabel;
  badges: string[];
  riskTags: string[];
  tags: string[];
  endpoints: FileCenterEndpoint[];
  metadata: FileCenterMetadataRow[];
};

export type FileCenterTagSuggestion = {
  name: string;
  count: number;
};

export type FileCenterDirectoryResult = {
  breadcrumbs: Array<{ id: string | null; label: string }>;
  items: FileCenterEntry[];
  total: number;
  currentPathChildren: number;
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
};

type SqlJsModule = Awaited<ReturnType<typeof initSqlJs>>;
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

type MetadataRow = {
  entry_id: string;
  order_index: number;
  label: string;
  value: string;
};

type TagRow = {
  entry_id: string;
  order_index: number;
  tag: string;
  kind: 'badge' | 'risk' | 'tag';
};

const FILE_CENTER_DB_KEY = 'mare-file-center-sqlite-v3';
let sqlModulePromise: Promise<SqlJsModule> | null = null;
let databasePromise: Promise<SqlDatabase> | null = null;

export const fileCenterApi = {
  async loadDirectory(params: FileCenterLoadParams): Promise<FileCenterDirectoryResult> {
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
    const total =
      querySingle<{ total: number }>(
        db,
        `SELECT COUNT(*) AS total FROM entries WHERE ${filter.whereClause}`,
        filter.parameters,
      )?.total ?? 0;

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
        notes,
        last_task_text,
        last_task_tone,
        rating,
        color_label
      FROM entries
      WHERE ${filter.whereClause}
      ORDER BY ${buildOrderClause(params.sortValue)}
      LIMIT $limit OFFSET $offset`,
      { ...filter.parameters, $limit: params.pageSize, $offset: offset },
    );

    return {
      breadcrumbs,
      items: hydrateEntries(db, rows),
      total,
      currentPathChildren,
    };
  },

  async loadEntryDetail(id: string): Promise<FileCenterEntry | null> {
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
        notes,
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

    return hydrateEntries(db, [row])[0] ?? null;
  },

  async createFolder(input: {
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
    insertDefaultEndpoints(db, id, input.libraryId);
    insertMetadataRows(db, id, [
      { label: '来源', value: '客户端新建' },
      { label: '路径', value: path },
      { label: '子项目', value: '0' },
      { label: '生命周期', value: 'ACTIVE' },
    ]);
    insertTagRows(db, id, [{ kind: 'badge', value: '新建目录' }]);
    persistDatabase(db);

    const item = await this.loadEntryDetail(id);
    if (!item) {
      throw new Error('目录创建成功，但读取结果失败');
    }
    return { message: '目录已创建', item };
  },

  async deleteAssets(ids: string[]): Promise<{ message: string }> {
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
  },

  async deleteFromEndpoint(id: string, endpointName: string): Promise<{ message: string }> {
    const db = await getDatabase();
    db.run(
      `UPDATE entry_endpoints
       SET state = '未同步', tone = 'critical', last_sync_at = '刚刚'
       WHERE entry_id = $id AND name = $endpointName`,
      { $id: id, $endpointName: endpointName },
    );

    const remaining = querySingle<{ total: number }>(
      db,
      "SELECT COUNT(*) AS total FROM entry_endpoints WHERE entry_id = $id AND state = '已同步'",
      { $id: id },
    )?.total ?? 0;
    const lifecycleState: AssetLifecycleState = remaining > 0 ? 'ACTIVE' : 'PENDING_DELETE';

    db.run(
      `UPDATE entries
       SET lifecycle_state = $lifecycleState, last_task_text = $lastTaskText, last_task_tone = $lastTaskTone
       WHERE id = $id`,
      {
        $id: id,
        $lifecycleState: lifecycleState,
        $lastTaskText: remaining > 0 ? '已提交端点删除请求' : '等待后台清理',
        $lastTaskTone: remaining > 0 ? 'info' : 'warning',
      },
    );

    syncLifecycleMetadata(db, [id], lifecycleState);
    persistDatabase(db);
    return { message: '已提交端点删除请求' };
  },

  async syncToEndpoint(id: string, endpointName: string): Promise<{ message: string }> {
    const db = await getDatabase();
    db.run(
      `UPDATE entry_endpoints
       SET state = '同步中', tone = 'warning', last_sync_at = '刚刚'
       WHERE entry_id = $id AND name = $endpointName`,
      { $id: id, $endpointName: endpointName },
    );
    db.run(
      `UPDATE entries
       SET last_task_text = $lastTaskText, last_task_tone = 'warning'
       WHERE id = $id`,
      { $id: id, $lastTaskText: `已创建同步任务到 ${endpointName}` },
    );
    persistDatabase(db);
    return { message: `已创建同步任务到 ${endpointName}` };
  },

  async updateAnnotations(
    id: string,
    input: { rating: number; colorLabel: FileCenterColorLabel; tags: string[] },
  ): Promise<{ message: string }> {
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
    db.run("DELETE FROM entry_tags WHERE entry_id = $id AND kind = 'tag'", { $id: id });
    insertTagRows(
      db,
      id,
      input.tags
        .map((tag) => tag.trim())
        .filter(Boolean)
        .map((tag) => ({ kind: 'tag' as const, value: tag })),
    );
    persistDatabase(db);
    return { message: '资产标记已更新' };
  },

  async loadTagSuggestions(searchText = ''): Promise<FileCenterTagSuggestion[]> {
    const db = await getDatabase();
    const keyword = searchText.trim();
    const rows = queryAll<{ tag: string; count: number }>(
      db,
      keyword
        ? `SELECT tag, COUNT(*) AS count
           FROM entry_tags
           WHERE kind = 'tag' AND tag LIKE $keyword
           GROUP BY tag
           ORDER BY count DESC, tag ASC`
        : `SELECT tag, COUNT(*) AS count
           FROM entry_tags
           WHERE kind = 'tag'
           GROUP BY tag
           ORDER BY count DESC, tag ASC`,
      keyword ? { $keyword: `%${keyword}%` } : {},
    );
    return rows.map((row) => ({ name: row.tag, count: Number(row.count) }));
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
  if (typeof window !== 'undefined') {
    window.localStorage.removeItem(FILE_CENTER_DB_KEY);
  }
}

async function getSqlModule() {
  if (!sqlModulePromise) {
    sqlModulePromise = (async () => {
      const isNodeRuntime = typeof process !== 'undefined' && Boolean(process.versions?.node);
      if (isNodeRuntime) {
        const { readFile } = await import('node:fs/promises');
        const { resolve } = await import('node:path');
        const wasmBinary = await readFile(resolve(process.cwd(), 'node_modules/sql.js/dist/sql-wasm.wasm'));
        return initSqlJs({ wasmBinary });
      }
      return initSqlJs({
        locateFile: () => sqlWasmUrl,
      });
    })();
  }
  return sqlModulePromise;
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
    return new SQL.Database(stored);
  }
  const db = new SQL.Database();
  createSchema(db);
  seedDatabase(db);
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

function getDefaultEndpoints(libraryId: string): FileCenterEndpoint[] {
  if (libraryId === 'video') {
    return [
      { name: '本地NVMe', state: '已同步', tone: 'success', lastSyncAt: '刚刚', endpointType: 'local' },
      { name: '影像NAS', state: '未同步', tone: 'critical', lastSyncAt: '尚未开始', endpointType: 'nas' },
    ];
  }
  if (libraryId === 'family') {
    return [
      { name: '本地NVMe', state: '已同步', tone: 'success', lastSyncAt: '刚刚', endpointType: 'local' },
      { name: '影像NAS', state: '已同步', tone: 'success', lastSyncAt: '刚刚', endpointType: 'nas' },
      { name: '115', state: '已同步', tone: 'success', lastSyncAt: '刚刚', endpointType: 'cloud' },
    ];
  }
  return [
    { name: '本地NVMe', state: '已同步', tone: 'success', lastSyncAt: '刚刚', endpointType: 'local' },
    { name: '影像NAS', state: '已同步', tone: 'success', lastSyncAt: '刚刚', endpointType: 'nas' },
    { name: '115', state: '未同步', tone: 'critical', lastSyncAt: '尚未开始', endpointType: 'cloud' },
  ];
}

function insertDefaultEndpoints(db: SqlDatabase, entryId: string, libraryId: string) {
  getDefaultEndpoints(libraryId).forEach((endpoint, orderIndex) => {
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
    insertEntry(
      db,
      createEntryRow({
        id,
        libraryId: 'photo',
        parentId: null,
        type: index % 4 === 0 ? 'file' : 'folder',
        name: index % 4 === 0 ? `拍摄计划_${String(index).padStart(2, '0')}.pdf` : `归档分组 ${String(index).padStart(2, '0')}`,
        fileKind: index % 4 === 0 ? '文档' : '文件夹',
        displayType: index % 4 === 0 ? 'PDF 文档' : '文件夹',
        modifiedAt: `2026-03-${String((index % 9) + 20).padStart(2, '0')} 1${index % 10}:00`,
        modifiedAtSort: Number(`202603${String((index % 9) + 20).padStart(2, '0')}1${index % 10}00`),
        createdAt: `2026-03-${String((index % 9) + 12).padStart(2, '0')} 09:00`,
        sizeLabel: index % 4 === 0 ? `${(index * 1.4 + 2).toFixed(1)} MB` : `${index * 7 + 12} 项`,
        sizeBytes: index % 4 === 0 ? Math.round((index * 1.4 + 2) * 1024 * 1024) : index * 7 + 12,
        path:
          index % 4 === 0
            ? `商业摄影资产库 / 合同文档 / 拍摄计划_${String(index).padStart(2, '0')}.pdf`
            : `商业摄影资产库 / 归档分组 ${String(index).padStart(2, '0')}`,
        sourceLabel: index % 4 === 0 ? '文档录入' : '自动归档',
        notes: '',
      }),
      index % 5 === 0 ? '等待补齐到 115' : '目录已索引',
      index % 5 === 0 ? 'warning' : 'success',
    );
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
        { name: '本地NVMe', state: '已同步', tone: 'success', lastSyncAt: '今天 09:18', endpointType: 'local' },
        { name: '影像NAS', state: '已同步', tone: 'success', lastSyncAt: '今天 09:18', endpointType: 'nas' },
        { name: '115', state: '未同步', tone: 'critical', lastSyncAt: '尚未开始', endpointType: 'cloud' },
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
        { name: '本地NVMe', state: '已同步', tone: 'success', lastSyncAt: '今天 09:18', endpointType: 'local' },
        { name: '影像NAS', state: '同步中', tone: 'warning', lastSyncAt: '刚刚', endpointType: 'nas' },
        { name: '115', state: '未同步', tone: 'critical', lastSyncAt: '尚未开始', endpointType: 'cloud' },
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
        { name: '本地NVMe', state: '已同步', tone: 'success', lastSyncAt: '昨天 22:18', endpointType: 'local' },
        { name: '影像NAS', state: '已同步', tone: 'success', lastSyncAt: '昨天 22:20', endpointType: 'nas' },
        { name: '115', state: '已同步', tone: 'success', lastSyncAt: '昨天 22:28', endpointType: 'cloud' },
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
        { name: '本地NVMe', state: '已同步', tone: 'success', lastSyncAt: '今天 10:02', endpointType: 'local' },
        { name: '影像NAS', state: '同步中', tone: 'warning', lastSyncAt: '刚刚', endpointType: 'nas' },
        { name: '115', state: '未同步', tone: 'critical', lastSyncAt: '尚未开始', endpointType: 'cloud' },
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
      taskText: '校验失败待处理',
      taskTone: 'critical',
      endpoints: [
        { name: '本地NVMe', state: '已同步', tone: 'success', lastSyncAt: '2026-03-28 10:06', endpointType: 'local' },
        { name: '影像NAS', state: '未同步', tone: 'critical', lastSyncAt: '2026-03-28 10:12', endpointType: 'nas' },
        { name: '115', state: '已同步', tone: 'success', lastSyncAt: '2026-03-28 10:20', endpointType: 'cloud' },
      ],
      metadata: [
        { label: '采样率', value: '48 kHz' },
        { label: '位深', value: '24 bit' },
        { label: '时长', value: '3 分 42 秒' },
        { label: '声道', value: '立体声' },
        { label: '生命周期', value: 'ACTIVE' },
      ],
      tags: [
        { kind: 'risk', value: '校验失败' },
        { kind: 'badge', value: '音频母版' },
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
      index % 6 === 0 ? '等待补齐到 115' : '索引已更新',
      index % 6 === 0 ? 'warning' : 'success',
    );
    db.run(
      `INSERT INTO entry_endpoints (entry_id, order_index, name, state, tone, last_sync_at, endpoint_type) VALUES
        ($id, 0, '本地NVMe', '已同步', 'success', '今天 09:18', 'local'),
        ($id, 1, '影像NAS', $nasState, $nasTone, $nasTime, 'nas'),
        ($id, 2, '115', $cloudState, $cloudTone, $cloudTime, 'cloud')`,
      {
        $id: id,
        $nasState: index % 7 === 0 ? '同步中' : '已同步',
        $nasTone: index % 7 === 0 ? 'warning' : 'success',
        $nasTime: index % 7 === 0 ? '刚刚' : '今天 09:20',
        $cloudState: index % 6 === 0 ? '未同步' : '已同步',
        $cloudTone: index % 6 === 0 ? 'critical' : 'success',
        $cloudTime: index % 6 === 0 ? '尚未开始' : '昨天 23:12',
      },
    );
    insertMetadataRows(db, id, [
      { label: '设备', value: camera },
      { label: '镜头', value: index % 2 === 0 ? '24-70mm F2.8 GM II' : '70-200mm F2.8 GM II' },
      { label: '分辨率', value: index % 2 === 0 ? '9504 × 6336' : '7008 × 4672' },
      { label: '生命周期', value: 'ACTIVE' },
    ]);
    insertTagRows(db, id, [
      { kind: 'badge', value: 'RAW' },
      ...(index % 10 === 0 ? [{ kind: 'tag' as const, value: '客户精选' }] : []),
      ...(index % 7 === 0 ? [{ kind: 'tag' as const, value: '待修图' }] : []),
      ...(index % 6 === 0 ? [{ kind: 'risk' as const, value: '待同步' }] : []),
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

  if (params.statusFilter === '待同步') {
    where.push("id IN (SELECT entry_id FROM entry_endpoints WHERE state IN ('未同步', '同步中'))");
  }
  if (params.statusFilter === '有异常') {
    where.push(
      "(id IN (SELECT entry_id FROM entry_endpoints WHERE tone IN ('warning', 'critical')) OR id IN (SELECT entry_id FROM entry_tags WHERE kind = 'risk'))",
    );
  }
  if (params.statusFilter === '多端齐全') {
    where.push(`id IN (
      SELECT entry_id
      FROM entry_endpoints
      GROUP BY entry_id
      HAVING COUNT(*) >= 2
        AND SUM(CASE WHEN state <> '已同步' OR tone IN ('warning', 'critical') THEN 1 ELSE 0 END) = 0
    )`);
  }
  if (params.statusFilter === '待清理') {
    where.push("lifecycle_state = 'PENDING_DELETE'");
  }

  return {
    whereClause: where.join(' AND '),
    parameters,
  };
}

function buildOrderClause(sortValue: FileCenterSortValue) {
  if (sortValue === '名称') {
    return "CASE WHEN type = 'folder' THEN 0 ELSE 1 END, name COLLATE NOCASE ASC";
  }
  if (sortValue === '大小') {
    return "CASE WHEN type = 'folder' THEN 0 ELSE 1 END, size_bytes DESC, modified_at_sort DESC";
  }
  return "CASE WHEN type = 'folder' THEN 0 ELSE 1 END, modified_at_sort DESC, name COLLATE NOCASE ASC";
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
  const metadataRows = queryAll<MetadataRow>(
    db,
    `SELECT entry_id, order_index, label, value
     FROM entry_metadata
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
  const metadataMap = groupBy(metadataRows, (row) => row.entry_id);
  const tagMap = groupBy(tagRows, (row) => row.entry_id);

  return rows.map((row) => {
    const tags = tagMap.get(row.id) ?? [];
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
      notes: row.notes,
      lastTaskText: row.last_task_text,
      lastTaskTone: row.last_task_tone,
      rating: Number(row.rating),
      colorLabel: row.color_label,
      badges: tags.filter((item) => item.kind === 'badge').map((item) => item.tag),
      riskTags: tags.filter((item) => item.kind === 'risk').map((item) => item.tag),
      tags: tags.filter((item) => item.kind === 'tag').map((item) => item.tag),
      endpoints: (endpointMap.get(row.id) ?? []).map((item) => ({
        name: item.name,
        state: item.state,
        tone: item.tone,
        lastSyncAt: item.last_sync_at,
        endpointType: item.endpoint_type,
      })),
      metadata: (metadataMap.get(row.id) ?? []).map((item) => ({
        label: item.label,
        value: item.value,
      })),
    };
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
