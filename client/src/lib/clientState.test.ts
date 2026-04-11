import { describe, expect, it } from 'vitest';
import { createInitialState, resolveDefaultLibraryId } from './clientState';

describe('clientState', () => {
  it('会优先恢复上次选择的资产库，而不是设置中的默认资产库', () => {
    const state = createInitialState();
    const libraries = [
      { id: 'photo', name: '商业摄影资产库' },
      { id: 'video', name: '视频工作流资产库' },
      { id: 'family', name: '家庭照片资产库' },
    ];

    const result = resolveDefaultLibraryId(state.settings, libraries, 'video');

    expect(result).toBe('video');
  });
});
