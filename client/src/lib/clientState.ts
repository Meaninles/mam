import type {
  FileNode,
  ImportBatch,
  ImportSourceFile,
  IssueRecord,
  Library,
  SettingSection,
  SettingsTab,
  Severity,
  StorageNode,
  TaskItemRecord,
  TaskRecord,
  ThemeMode,
} from '../data';
import {
  cloneSettingsContent,
  fileNodes,
  importBatches,
  importSourceFiles,
  issueRecords,
  libraries,
  storageNodes,
  taskItemRecords,
  taskRecords,
} from '../data';

export interface NotificationItem {
  id: string;
  title: string;
  detail: string;
  tone: Severity;
  createdAt: string;
  read?: boolean;
}

export interface PersistedState {
  fileNodes: FileNode[];
  importBatches: ImportBatch[];
  importSourceFiles: ImportSourceFile[];
  issueRecords: IssueRecord[];
  libraries: Library[];
  notifications: NotificationItem[];
  settings: Record<SettingsTab, SettingSection[]>;
  storageNodes: StorageNode[];
  taskItemRecords: TaskItemRecord[];
  taskRecords: TaskRecord[];
}

export const STORAGE_KEY = 'mare-client-state-v2';

export function createInitialState(): PersistedState {
  return {
    fileNodes: structuredClone(fileNodes),
    importBatches: structuredClone(importBatches),
    importSourceFiles: structuredClone(importSourceFiles),
    issueRecords: structuredClone(issueRecords),
    libraries: structuredClone(libraries),
    notifications: [
      {
        id: 'notice-removable-1',
        title: '检测到移动硬盘 SanDisk Extreme 2TB',
        detail: '可加入为存储节点，也可先标记已读，后续在存储节点页继续处理。',
        tone: 'info',
        createdAt: '刚刚',
        read: false,
      },
    ],
    settings: cloneSettingsContent(),
    storageNodes: structuredClone(storageNodes),
    taskItemRecords: structuredClone(taskItemRecords),
    taskRecords: structuredClone(taskRecords),
  };
}

export function loadPersistedState(): PersistedState {
  const raw = window.localStorage.getItem(STORAGE_KEY);
  if (!raw) {
    return createInitialState();
  }

  try {
    const parsed = JSON.parse(raw) as PersistedState;
    return {
      ...parsed,
      notifications: (parsed.notifications ?? []).map((item) => ({ ...item, read: item.read ?? true })),
      settings: parsed.settings ? cloneSettingsRecord(parsed.settings) : cloneSettingsContent(),
    };
  } catch {
    return createInitialState();
  }
}

export function cloneSettingsRecord(
  settings: Record<SettingsTab, SettingSection[]>,
): Record<SettingsTab, SettingSection[]> {
  const fallback = cloneSettingsContent();
  return {
    general: structuredClone(settings.general ?? fallback.general),
    'file-overview': structuredClone(settings['file-overview'] ?? fallback['file-overview']),
    'tag-management': structuredClone(settings['tag-management'] ?? fallback['tag-management']),
    verification: structuredClone(settings.verification ?? fallback.verification),
    performance: structuredClone(settings.performance ?? fallback.performance),
    appearance: structuredClone(settings.appearance ?? fallback.appearance),
  };
}

export function resolveThemeMode(settings: Record<SettingsTab, SettingSection[]>): ThemeMode {
  const value = findSettingValue(settings, 'appearance', 'appearance', 'theme');
  return value === '浅色主题' ? 'light' : 'dark';
}

export function getDefaultPageSize(settings: Record<SettingsTab, SettingSection[]>): 10 | 20 | 50 | 100 {
  const value = findSettingValue(settings, 'file-overview', 'overview', 'default-page-size');
  if (value === '10' || value === '20' || value === '50' || value === '100') {
    return Number(value) as 10 | 20 | 50 | 100;
  }
  return 20;
}

export function findSettingValue(
  settings: Record<SettingsTab, SettingSection[]>,
  tab: SettingsTab,
  sectionId: string,
  rowId: string,
): string {
  return settings[tab].find((section) => section.id === sectionId)?.rows.find((row) => row.id === rowId)?.value ?? '';
}

export function updateSettingValue(
  settings: Record<SettingsTab, SettingSection[]>,
  tab: SettingsTab,
  sectionId: string,
  rowId: string,
  value: string,
): Record<SettingsTab, SettingSection[]> {
  return {
    ...settings,
    [tab]: settings[tab].map((section) =>
      section.id === sectionId
        ? {
            ...section,
            rows: section.rows.map((row) => (row.id === rowId ? { ...row, value } : row)),
          }
        : section,
    ),
  };
}

export function getSortableSize(value: string): number {
  if (value.includes('项')) {
    return parseFloat(value.replace(/[^\d.]/g, ''));
  }
  const numeric = parseFloat(value.replace(/[^\d.]/g, ''));
  if (value.includes('TB')) return numeric * 1024 * 1024;
  if (value.includes('GB')) return numeric * 1024;
  return numeric;
}

export function collectNodeIds(nodes: FileNode[], targetIds: string[]): string[] {
  const queue = [...targetIds];
  const visited = new Set<string>();
  while (queue.length > 0) {
    const current = queue.shift();
    if (!current || visited.has(current)) {
      continue;
    }
    visited.add(current);
    nodes.filter((node) => node.parentId === current).forEach((child) => queue.push(child.id));
  }
  return Array.from(visited);
}

export function getDisplayTypeByFileKind(kind: FileNode['fileKind']): string {
  if (kind === '图片') return '图像文件';
  if (kind === '视频') return '视频文件';
  if (kind === '音频') return '音频文件';
  if (kind === '文档') return '文档文件';
  return '文件夹';
}

export function resolveLibraryForImport(batchId: string): string {
  return batchId === 'import-audio' ? 'video' : 'photo';
}

export function createId(prefix: string): string {
  return `${prefix}-${Math.random().toString(36).slice(2, 8)}`;
}
