import { describe, expect, it } from 'vitest';
import { filterImportDevices, resolveImportEntrySignal, resolveImportSubmitState, sortImportDevices } from './importCenter';
import { importFixtureDevices, importFixtureDrafts, importFixtureSourceNodes } from '../test/importFixtures';

describe('importCenter helpers', () => {
  it('根据待导入设备生成动态页头入口状态', () => {
    const signal = resolveImportEntrySignal(importFixtureDevices);

    expect(signal.connectedDeviceCount).toBe(2);
    expect(signal.label).toBe('已插入 2 个设备');
    expect(signal.tone).toBe('warning');
  });

  it('支持按状态和关键词筛选待导入端，并按状态优先排序', () => {
    const filtered = filterImportDevices(importFixtureDevices, '异常', '访谈');
    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.deviceLabel).toBe('录音 U 盘（访谈音频）');

    const sorted = sortImportDevices(importFixtureDevices, '状态优先');
    expect(sorted[0]?.sessionStatus).toBe('异常待处理');
    expect(sorted[1]?.sessionStatus).toBe('可导入');
  });

  it('在存在阻塞项或未分配目标端时禁用提交导入', () => {
    const audioDevice = importFixtureDevices.find((device) => device.id === 'device-audio');
    const audioDraft = importFixtureDrafts.find((draft) => draft.deviceSessionId === 'device-audio');
    const audioFiles = importFixtureSourceNodes.filter((file) => file.deviceSessionId === 'device-audio');

    expect(audioDevice).toBeTruthy();
    expect(audioDraft).toBeTruthy();

    const blocked = resolveImportSubmitState(audioDevice!, audioDraft, audioFiles);
    expect(blocked.disabled).toBe(true);
    expect(blocked.reason).toContain('阻塞项');

    const readyDevice = importFixtureDevices.find((device) => device.id === 'device-cfexpress-a');
    const readyDraft = importFixtureDrafts.find((draft) => draft.deviceSessionId === 'device-cfexpress-a');
    const readyFiles = importFixtureSourceNodes.filter((file) => file.deviceSessionId === 'device-cfexpress-a');
    const ready = resolveImportSubmitState(readyDevice!, readyDraft, readyFiles);

    expect(ready.disabled).toBe(false);
    expect(ready.actionLabel).toBe('提交导入');
  });
});
