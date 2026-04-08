import type {
  FileNode,
  HeaderSignal,
  ImportBatch,
  ImportDeviceSessionRecord,
  ImportDraftRecord,
  ImportReportSnapshot,
  ImportSourceFile,
  ImportSourceNodeRecord,
  ImportTargetEndpointRecord,
  IssueRecord,
  Library,
  NoticeRecord,
  SettingSection,
  SettingsTab,
  StorageNode,
  TaskItemRecord,
  TaskRecord,
  MainView,
  ThemeMode,
} from '../data';
import {
  cloneSettingsContent,
  fileNodes,
  headerSignals,
  importBatches,
  importDeviceSessions,
  importDrafts,
  importReports,
  importSourceFiles,
  importSourceNodes,
  importTargetEndpoints,
  issueRecords,
  libraries,
  noticeRecords,
  storageNodes,
  taskItemRecords,
  taskRecords,
} from '../data';

export interface PersistedState {
  fileNodes: FileNode[];
  headerSignals: HeaderSignal[];
  importBatches: ImportBatch[];
  importDeviceSessions: ImportDeviceSessionRecord[];
  importDrafts: ImportDraftRecord[];
  importReports: ImportReportSnapshot[];
  importSourceFiles: ImportSourceFile[];
  importSourceNodes: ImportSourceNodeRecord[];
  importTargetEndpoints: ImportTargetEndpointRecord[];
  issueRecords: IssueRecord[];
  libraries: Library[];
  noticeRecords: NoticeRecord[];
  settings: Record<SettingsTab, SettingSection[]>;
  storageNodes: StorageNode[];
  taskItemRecords: TaskItemRecord[];
  taskRecords: TaskRecord[];
}

export const STORAGE_KEY = 'mare-client-state-v4';

export function createInitialState(): PersistedState {
  return {
    fileNodes: structuredClone(fileNodes),
    headerSignals: structuredClone(headerSignals),
    importBatches: structuredClone(importBatches),
    importDeviceSessions: structuredClone(importDeviceSessions),
    importDrafts: structuredClone(importDrafts),
    importReports: structuredClone(importReports),
    importSourceFiles: structuredClone(importSourceFiles),
    importSourceNodes: structuredClone(importSourceNodes),
    importTargetEndpoints: structuredClone(importTargetEndpoints),
    issueRecords: structuredClone(issueRecords),
    libraries: structuredClone(libraries),
    noticeRecords: structuredClone(noticeRecords),
    settings: cloneSettingsContent(),
    storageNodes: structuredClone(storageNodes),
    taskItemRecords: structuredClone(taskItemRecords),
    taskRecords: structuredClone(taskRecords),
  };
}

function createTaskStateSeeds() {
  return {
    importBatches: structuredClone(importBatches),
    importDeviceSessions: structuredClone(importDeviceSessions),
    importDrafts: structuredClone(importDrafts),
    importReports: structuredClone(importReports),
    importSourceFiles: structuredClone(importSourceFiles),
    importSourceNodes: structuredClone(importSourceNodes),
    importTargetEndpoints: structuredClone(importTargetEndpoints),
    issueRecords: structuredClone(issueRecords),
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
    const taskSeeds = createTaskStateSeeds();
    return {
      ...parsed,
      headerSignals: structuredClone(parsed.headerSignals ?? headerSignals),
      importBatches: taskSeeds.importBatches,
      importDeviceSessions: taskSeeds.importDeviceSessions,
      importDrafts: taskSeeds.importDrafts,
      importReports: taskSeeds.importReports,
      importSourceFiles: taskSeeds.importSourceFiles,
      importSourceNodes: taskSeeds.importSourceNodes,
      importTargetEndpoints: taskSeeds.importTargetEndpoints,
      issueRecords: taskSeeds.issueRecords,
      noticeRecords: structuredClone(parsed.noticeRecords ?? noticeRecords),
      settings: parsed.settings ? cloneSettingsRecord(parsed.settings) : cloneSettingsContent(),
      taskItemRecords: taskSeeds.taskItemRecords,
      taskRecords: taskSeeds.taskRecords,
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
    workspace: structuredClone(settings.workspace ?? fallback.workspace),
    'file-overview': structuredClone(settings['file-overview'] ?? fallback['file-overview']),
    'tag-management': structuredClone(settings['tag-management'] ?? fallback['tag-management']),
    'import-archive': structuredClone(settings['import-archive'] ?? fallback['import-archive']),
    notifications: structuredClone(settings.notifications ?? fallback.notifications),
    'issue-governance': structuredClone(settings['issue-governance'] ?? fallback['issue-governance']),
    verification: structuredClone(settings.verification ?? fallback.verification),
    'background-tasks': structuredClone(settings['background-tasks'] ?? fallback['background-tasks']),
    appearance: structuredClone(settings.appearance ?? fallback.appearance),
  };
}

export function resolveThemeMode(settings: Record<SettingsTab, SettingSection[]>): ThemeMode {
  const value = findSettingValue(settings, 'appearance', 'appearance', 'theme');
  return value === '浅色主题' ? 'light' : 'dark';
}

export function resolveStartupWorkspace(settings: Record<SettingsTab, SettingSection[]>): MainView {
  const value = findSettingValue(settings, 'workspace', 'workspace-defaults', 'startup-page');
  if (value === '任务中心') return 'task-center';
  if (value === '异常中心') return 'issues';
  if (value === '存储节点') return 'storage-nodes';
  if (value === '设置') return 'settings';
  if (value === '导入中心') return 'import-center';
  return 'file-center';
}

export function resolveDefaultLibraryId(
  settings: Record<SettingsTab, SettingSection[]>,
  availableLibraries: Library[],
): string {
  const value = findSettingValue(settings, 'general', 'launch', 'default-library');
  if (value === '上次使用') {
    return availableLibraries[0]?.id ?? 'photo';
  }
  return availableLibraries.find((library) => library.name === value)?.id ?? availableLibraries[0]?.id ?? 'photo';
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
