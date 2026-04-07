import { describe, expect, it } from 'vitest';
import { createInitialState } from './clientState';
import { filterImportDevices, resolveImportEntrySignal, resolveImportSubmitState, sortImportDevices } from './importCenter';

describe('importCenter helpers', () => {
  it('根据待导入设备生成动态页头入口状态', () => {
    const state = createInitialState();
    const signal = resolveImportEntrySignal(state.importDeviceSessions);

    expect(signal.connectedDeviceCount).toBe(5);
    expect(signal.label).toBe('已插入 5 个设备');
    expect(signal.tone).toBe('warning');
  });

  it('支持按状态和关键词筛选待导入端，并按状态优先排序', () => {
    const state = createInitialState();

    const filtered = filterImportDevices(state.importDeviceSessions, '异常', '访谈');
    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.deviceLabel).toBe('录音 U 盘（访谈音频）');

    const sorted = sortImportDevices(state.importDeviceSessions, '状态优先');
    expect(sorted[0]?.sessionStatus).toBe('异常待处理');
    expect(sorted[1]?.sessionStatus).toBe('导入中');
  });

  it('在存在阻塞项或未分配目标端时禁用提交导入', () => {
    const state = createInitialState();
    const audioDevice = state.importDeviceSessions.find((device) => device.id === 'import-device-audio');
    const audioDraft = state.importDrafts.find((draft) => draft.deviceSessionId === 'import-device-audio');
    const audioFiles = state.importSourceNodes.filter((file) => file.deviceSessionId === 'import-device-audio');

    expect(audioDevice).toBeTruthy();
    expect(audioDraft).toBeTruthy();

    const blocked = resolveImportSubmitState(audioDevice!, audioDraft, audioFiles);
    expect(blocked.disabled).toBe(true);
    expect(blocked.reason).toContain('阻塞项');

    const readyDevice = state.importDeviceSessions.find((device) => device.id === 'import-device-cfexpress-a');
    const readyDraft = state.importDrafts.find((draft) => draft.deviceSessionId === 'import-device-cfexpress-a');
    const readyFiles = state.importSourceNodes.filter((file) => file.deviceSessionId === 'import-device-cfexpress-a');
    const ready = resolveImportSubmitState(readyDevice!, readyDraft, readyFiles);

    expect(ready.disabled).toBe(false);
    expect(ready.actionLabel).toBe('提交导入');
  });
});
