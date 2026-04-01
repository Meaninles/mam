import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { IconButton } from './Shared';

describe('IconButton', () => {
  it('使用自定义 tooltip 属性而不是浏览器原生 title', () => {
    render(
      <IconButton ariaLabel="连接测试 商业摄影原片库" tooltip="连接测试">
        <span>i</span>
      </IconButton>,
    );

    const button = screen.getByRole('button', { name: '连接测试 商业摄影原片库' });
    expect(button).toHaveAttribute('data-tooltip', '连接测试');
    expect(button).not.toHaveAttribute('title');
  });
});
