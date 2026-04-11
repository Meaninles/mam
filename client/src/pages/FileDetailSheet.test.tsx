import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import type { FileCenterEntry } from '../lib/fileCenterApi';
import { FileDetailSheet } from './FileDetailSheet';

const baseItem: FileCenterEntry = {
  id: 'photo-file-raw-001',
  libraryId: 'photo',
  parentId: 'photo-root-raw',
  type: 'file',
  lifecycleState: 'ACTIVE',
  name: '2026-03-29_上海发布会_A-cam_001.RAW',
  fileKind: '图片',
  displayType: 'RAW 图像',
  modifiedAt: '2026-03-29 18:42',
  createdAt: '2026-03-29 18:42',
  size: '48.6 MB',
  path: '商业摄影资产库 / 拍摄原片 / 2026-03-29_上海发布会_A-cam_001.RAW',
  sourceLabel: 'Sony A7R V',
  lastTaskText: '等待补齐到 115',
  lastTaskTone: 'warning',
  rating: 4,
  colorLabel: '无',
  badges: ['RAW'],
  riskTags: [],
  tags: ['发布会'],
  endpoints: [{ name: '本地NVMe', state: '已同步', tone: 'success', lastSyncAt: '今天 09:18', endpointType: 'local' }],
};

describe('FileDetailSheet', () => {
  it('目录详情页不显示星级和色标编辑', () => {
    const folderItem: FileCenterEntry = {
      ...baseItem,
      id: 'photo-folder-raw',
      type: 'folder',
      name: '拍摄原片',
      displayType: '文件夹',
      fileKind: '文件夹',
      size: '12 项',
    };

    render(<FileDetailSheet item={folderItem} onClose={vi.fn()} onSaveAnnotations={vi.fn()} />);

    expect(screen.queryByRole('group', { name: '资产星级' })).toBeNull();
    expect(screen.queryByRole('button', { name: '保存标记' })).toBeNull();
    expect(screen.queryByRole('button', { name: '红标' })).toBeNull();
  });

  it('可以在详情页修改星级后保存', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const onSaveAnnotations = vi.fn().mockResolvedValue(undefined);

    render(<FileDetailSheet item={baseItem} onClose={onClose} onSaveAnnotations={onSaveAnnotations} />);

    await user.click(screen.getByRole('button', { name: '5 星' }));
    await user.click(screen.getByRole('button', { name: '保存标记' }));

    await waitFor(() =>
      expect(onSaveAnnotations).toHaveBeenCalledWith({
        id: baseItem.id,
        rating: 5,
        colorLabel: baseItem.colorLabel,
        tags: baseItem.tags,
      }),
    );
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
  });

  it('保存色标后会自动关闭详情页', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const onSaveAnnotations = vi.fn().mockResolvedValue(undefined);

    render(<FileDetailSheet item={baseItem} onClose={onClose} onSaveAnnotations={onSaveAnnotations} />);

    await user.click(screen.getByRole('button', { name: '红标' }));
    await user.click(screen.getByRole('button', { name: '保存标记' }));

    await waitFor(() =>
      expect(onSaveAnnotations).toHaveBeenCalledWith({
        id: baseItem.id,
        rating: baseItem.rating,
        colorLabel: '红标',
        tags: baseItem.tags,
      }),
    );
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
  });
});
