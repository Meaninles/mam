import type {
  ImportDeviceSessionRecord,
  ImportDeviceSessionStatus,
  ImportDraftRecord,
  ImportFileStatus,
  ImportReportSnapshot,
  ImportSourceNodeRecord,
} from '../data';

export type ImportCenterFilterValue = '全部' | '待扫描' | '可导入' | '导入中' | '异常';
export type ImportCenterSortValue = '最近接入' | '状态优先' | '名称' | '剩余空间';
export type ImportFileFilterValue = '全部' | '可导入' | '已导入' | '冲突' | '异常';

export type ImportCenterTab =
  | { id: 'pool'; type: 'pool'; label: '待导入端'; statusLabel: string; closeable: false }
  | { id: `device:${string}`; type: 'device'; deviceSessionId: string; label: string; statusLabel: string; closeable: true }
  | { id: `report:${string}`; type: 'report'; reportId: string; label: string; statusLabel: string; closeable: true };

export function resolveImportEntrySignal(devices: ImportDeviceSessionRecord[]) {
  const connectedDevices = devices.filter((device) => device.sessionStatus !== '已拔出');
  const attentionDeviceCount = connectedDevices.filter((device) =>
    ['异常待处理', '扫描中', '待识别'].includes(device.sessionStatus),
  ).length;
  const importingDeviceCount = connectedDevices.filter((device) => device.sessionStatus === '导入中').length;

  const label =
    connectedDevices.length === 0
      ? '导入'
      : connectedDevices.length === 1
        ? '已插入 1 个设备'
        : `已插入 ${connectedDevices.length} 个设备`;

  const tone =
    connectedDevices.length === 0 ? 'default' : attentionDeviceCount > 0 ? 'warning' : importingDeviceCount > 0 ? 'info' : 'success';

  return {
    attentionDeviceCount,
    connectedDeviceCount: connectedDevices.length,
    importingDeviceCount,
    label,
    tone,
  };
}

export function filterImportDevices(
  devices: ImportDeviceSessionRecord[],
  filterValue: ImportCenterFilterValue,
  keyword: string,
) {
  const normalizedKeyword = keyword.trim().toLowerCase();

  return devices.filter((device) => {
    const matchesFilter =
      filterValue === '全部'
        ? true
        : filterValue === '待扫描'
          ? ['待识别', '扫描中'].includes(device.sessionStatus)
          : filterValue === '可导入'
            ? ['可导入', '部分完成'].includes(device.sessionStatus)
            : filterValue === '导入中'
              ? device.sessionStatus === '导入中'
              : ['异常待处理', '已拔出'].includes(device.sessionStatus);

    if (!matchesFilter) {
      return false;
    }

    if (!normalizedKeyword) {
      return true;
    }

    return [
      device.deviceLabel,
      device.deviceType,
      device.mountPath,
      device.description,
      device.sessionStatus,
    ]
      .join(' ')
      .toLowerCase()
      .includes(normalizedKeyword);
  });
}

function resolveStatusPriority(status: ImportDeviceSessionStatus) {
  if (status === '异常待处理') return 0;
  if (status === '导入中') return 1;
  if (status === '可导入') return 2;
  if (status === '部分完成') return 3;
  if (status === '扫描中') return 4;
  if (status === '待识别') return 5;
  return 6;
}

function parseAvailableStorage(value: string) {
  const normalized = value.toUpperCase();
  const numeric = Number.parseFloat(normalized.replace(/[^\d.]/g, ''));
  if (!Number.isFinite(numeric)) {
    return 0;
  }
  if (normalized.includes('TB')) return numeric * 1024;
  if (normalized.includes('GB')) return numeric;
  if (normalized.includes('MB')) return numeric / 1024;
  return numeric;
}

export function sortImportDevices(devices: ImportDeviceSessionRecord[], sortValue: ImportCenterSortValue) {
  const next = [...devices];
  next.sort((left, right) => {
    if (sortValue === '名称') {
      return left.deviceLabel.localeCompare(right.deviceLabel, 'zh-CN');
    }

    if (sortValue === '状态优先') {
      const byStatus = resolveStatusPriority(left.sessionStatus) - resolveStatusPriority(right.sessionStatus);
      if (byStatus !== 0) {
        return byStatus;
      }
      return right.connectedAtSortKey - left.connectedAtSortKey;
    }

    if (sortValue === '剩余空间') {
      return parseAvailableStorage(right.capacitySummary.available) - parseAvailableStorage(left.capacitySummary.available);
    }

    return right.connectedAtSortKey - left.connectedAtSortKey;
  });

  return next;
}

export function resolveImportFileFilterMatch(file: ImportSourceNodeRecord, filterValue: ImportFileFilterValue, keyword: string) {
  const normalizedKeyword = keyword.trim().toLowerCase();
  const matchesKeyword =
    normalizedKeyword.length === 0
      ? true
      : [file.name, file.relativePath, file.status, file.note ?? ''].join(' ').toLowerCase().includes(normalizedKeyword);

  if (!matchesKeyword) {
    return false;
  }

  if (filterValue === '全部') {
    return true;
  }

  if (filterValue === '可导入') {
    return ['待导入', '已排队'].includes(file.status);
  }

  if (filterValue === '已导入') {
    return ['传输中', '校验中', '已完成'].includes(file.status);
  }

  if (filterValue === '冲突') {
    return file.status === '冲突';
  }

  return file.status === '失败';
}

export function resolveImportSubmitState(
  device: ImportDeviceSessionRecord,
  draft: ImportDraftRecord | undefined,
  sourceNodes: ImportSourceNodeRecord[],
) {
  if (!draft) {
    return {
      disabled: true,
      reason: '当前设备还没有可编辑的导入草稿。',
    };
  }

  if (device.sessionStatus === '已拔出') {
    return {
      disabled: true,
      reason: '设备已拔出，当前会话只支持回看草稿与结果。',
    };
  }

  if (device.sessionStatus === '导入中' || draft.status === '导入中') {
    return {
      disabled: true,
      reason: null,
      actionLabel: '查看任务',
    };
  }

  if (draft.hasBlockingIssues || draft.precheckSummary.blockingCount > 0) {
    return {
      disabled: true,
      reason: '仍存在阻塞项，请先处理路径冲突、目标不可写或设备离线问题。',
    };
  }

  if (!draft.libraryId) {
    return {
      disabled: true,
      reason: '请先选择导入目标资产库。',
    };
  }

  const missingTargetFile = sourceNodes.find((file) => file.targetEndpointIds.length === 0 && file.status !== '已跳过');
  if (missingTargetFile) {
    return {
      disabled: true,
      reason: `文件“${missingTargetFile.name}”还没有选择目标端。`,
    };
  }

  return {
    disabled: false,
    reason: null,
    actionLabel: '提交导入',
  };
}

export function buildImportDeviceTab(device: ImportDeviceSessionRecord): ImportCenterTab {
  return {
    id: `device:${device.id}`,
    type: 'device',
    deviceSessionId: device.id,
    label: device.deviceLabel,
    statusLabel: device.sessionStatus,
    closeable: true,
  };
}

export function buildImportReportTab(report: ImportReportSnapshot): ImportCenterTab {
  return {
    id: `report:${report.id}`,
    type: 'report',
    reportId: report.id,
    label: report.title,
    statusLabel: report.status,
    closeable: true,
  };
}

export function resolveImportFileStatusTone(status: ImportFileStatus) {
  if (status === '失败' || status === '冲突') return 'critical';
  if (status === '传输中' || status === '校验中') return 'warning';
  if (status === '已完成') return 'success';
  if (status === '已跳过') return 'info';
  return 'info';
}

export function sortImportReports(reports: ImportReportSnapshot[]) {
  return [...reports].sort((left, right) => right.latestUpdatedAt.localeCompare(left.latestUpdatedAt, 'zh-CN'));
}
