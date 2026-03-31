import { Trash2, Pencil, X } from 'lucide-react';
import type { Library, Severity } from '../data';

export function IconButton({
  active,
  ariaLabel,
  children,
  onClick,
}: {
  active?: boolean;
  ariaLabel: string;
  children: React.ReactNode;
  onClick?: () => void;
}) {
  return (
    <button
      aria-label={ariaLabel}
      className={`icon-button${active ? ' active' : ''}`}
      type="button"
      onClick={onClick}
    >
      {children}
    </button>
  );
}

export function TonePill({ children, tone }: { children: React.ReactNode; tone: Severity }) {
  return <span className={`tone-pill ${tone}`}>{children}</span>;
}

export function ProgressBar({ value }: { value: number }) {
  return (
    <div aria-valuemax={100} aria-valuemin={0} aria-valuenow={value} className="progress" role="progressbar">
      <div className="progress-fill" style={{ width: `${value}%` }} />
    </div>
  );
}

export function DenseRow({
  label,
  tone,
  value,
}: {
  label: string;
  tone?: Severity;
  value: string;
}) {
  return (
    <div className="dense-row">
      <span>{label}</span>
      <strong className={tone ? `tone-text-${tone}` : undefined}>{value}</strong>
    </div>
  );
}

export function SelectPill({
  ariaLabel,
  onChange,
  options,
  value,
}: {
  ariaLabel: string;
  onChange: (value: string) => void;
  options: string[];
  value: string;
}) {
  return (
    <label className="select-pill">
      <select aria-label={ariaLabel} value={value} onChange={(event) => onChange(event.target.value)}>
        {options.map((option) => (
          <option key={option} value={option}>
            {option}
          </option>
        ))}
      </select>
    </label>
  );
}

export function TabSwitch({
  items,
  onChange,
  value,
}: {
  items: Array<{ id: string; label: string }>;
  onChange: (value: string) => void;
  value: string;
}) {
  return (
    <div className="tab-switch">
      {items.map((item) => (
        <button
          key={item.id}
          className={value === item.id ? 'active' : ''}
          type="button"
          onClick={() => onChange(item.id)}
        >
          {item.label}
        </button>
      ))}
    </div>
  );
}

export function SettingControl({
  control,
  value,
}: {
  control: 'toggle' | 'select' | 'input' | 'segmented';
  value: string;
}) {
  return <span className={`setting-control ${control}`}>{value}</span>;
}

export function Sheet({
  children,
  onClose,
  title,
}: {
  children: React.ReactNode;
  onClose: () => void;
  title: string;
}) {
  return (
    <div className="sheet-backdrop" role="presentation">
      <section aria-label={title} className="sheet-panel">
        <div className="sheet-header">
          <strong>{title}</strong>
          <IconButton ariaLabel="关闭" onClick={onClose}>
            <X size={16} />
          </IconButton>
        </div>
        {children}
      </section>
    </div>
  );
}

export function LibraryManagerSheet({ library, onClose }: { library: Library; onClose: () => void }) {
  return (
    <Sheet onClose={onClose} title={library.name}>
      <div className="sheet-section">
        <DenseRow label="名称" value={library.name} />
        <DenseRow label="默认路径" value={library.rootLabel} />
        <DenseRow label="文件数" value={library.itemCount} />
        <DenseRow label="健康度" value={library.health} />
        <DenseRow label="策略" value={library.storagePolicy} />
      </div>
      <div className="sheet-actions right">
        <IconButton ariaLabel="修改">
          <Pencil size={16} />
        </IconButton>
        <IconButton ariaLabel="删除">
          <Trash2 size={16} />
        </IconButton>
      </div>
    </Sheet>
  );
}
