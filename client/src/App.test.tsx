import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import App from './App';

describe('统一文件管理系统客户端', () => {
  it('默认进入文件中心并显示资产库下拉', () => {
    render(<App />);

    expect(screen.getByText('MARE')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /商业摄影资产库/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '文件中心' })).toBeInTheDocument();
  });

  it('点击资产库管理按钮后打开管理面板', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: /商业摄影资产库/i }));
    await user.click(screen.getByRole('button', { name: /管理 商业摄影资产库/i }));

    expect(screen.getByRole('region', { name: '商业摄影资产库' })).toBeInTheDocument();
    expect(screen.getByText('本地 + NAS + 115')).toBeInTheDocument();
  });

  it('设置中包含文件总览子页面', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '设置' }));
    await user.click(screen.getByRole('button', { name: '文件总览' }));

    expect(screen.getByText('默认列集')).toBeInTheDocument();
    expect(screen.getByText('名称 / 修改日期 / 类型 / 大小 / 多端状态')).toBeInTheDocument();
  });
});
