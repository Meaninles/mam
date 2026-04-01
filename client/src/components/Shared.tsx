import { Pencil, Plus, Trash2, X } from 'lucide-react';
import type { Library, SettingControlType, Severity } from '../data';

export function IconButton({
  active,
  ariaLabel,
  children,
  onClick,
  tooltip,
}: {
  active?: boolean;
  ariaLabel: string;
  children: React.ReactNode;
  onClick?: React.MouseEventHandler<HTMLButtonElement>;
  tooltip?: string;
}) {
  return (
    <button
      aria-label={ariaLabel}
      className={`icon-button${active ? ' active' : ''}${tooltip || ariaLabel ? ' has-tooltip' : ''}`}
      data-tooltip={tooltip ?? ariaLabel}
      type="button"
      onClick={onClick}
    >
      {children}
    </button>
  );
}

export function ActionButton({
  ariaLabel,
  className,
  children,
  onClick,
  tone = 'default',
  type = 'button',
}: {
  ariaLabel?: string;
  className?: string;
  children: React.ReactNode;
  onClick?: () => void;
  tone?: 'default' | 'primary' | 'danger';
  type?: 'button' | 'submit';
}) {
  return (
    <button
      aria-label={ariaLabel}
      className={`action-button ${tone}${className ? ` ${className}` : ''}`}
      type={type}
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
  value: React.ReactNode;
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

export function InlineSettingControl({
  control,
  label,
  onChange,
  options,
  value,
}: {
  control: SettingControlType;
  label: string;
  onChange: (value: string) => void;
  options?: string[];
  value: string;
}) {
  if (control === 'input') {
    return (
      <input
        aria-label={label}
        className="setting-input"
        type="text"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    );
  }

  if (control === 'select') {
    return (
      <select aria-label={label} className="setting-select" value={value} onChange={(event) => onChange(event.target.value)}>
        {options?.map((option) => (
          <option key={option} value={option}>
            {option}
          </option>
        ))}
      </select>
    );
  }

  if (control === 'toggle') {
    const toggleOptions = options ?? ['关闭', '开启'];
    return (
      <button
        aria-label={label}
        className={`toggle-button${value === toggleOptions[toggleOptions.length - 1] ? ' active' : ''}`}
        type="button"
        onClick={() => onChange(value === toggleOptions[0] ? toggleOptions[1] : toggleOptions[0])}
      >
        {value}
      </button>
    );
  }

  return (
    <div className="mini-segmented" role="group" aria-label={label}>
      {(options ?? []).map((option) => (
        <button
          key={option}
          className={option === value ? 'active' : ''}
          type="button"
          onClick={() => onChange(option)}
        >
          {option}
        </button>
      ))}
    </div>
  );
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
    <div className="sheet-backdrop" role="presentation" onClick={onClose}>
      <section
        aria-label={title}
        className="sheet-panel"
        role="region"
        onClick={(event) => event.stopPropagation()}
      >
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

export function StatCard({
  label,
  tone,
  value,
}: {
  label: string;
  tone?: Severity | 'default';
  value: string;
}) {
  return (
    <div className={`stat-card${tone && tone !== 'default' ? ` ${tone}` : ''}`}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

export function EmptyState({
  action,
  description,
  title,
}: {
  action?: React.ReactNode;
  description: string;
  title: string;
}) {
  return (
    <div className="empty-state">
      <strong>{title}</strong>
      <p>{description}</p>
      {action}
    </div>
  );
}

export function FeedbackBanner({
  message,
  tone,
}: {
  message: string;
  tone: Severity;
}) {
  return (
    <div className={`feedback-banner ${tone}`} role="alert">
      {message}
    </div>
  );
}

export function LibraryManagerSheet({
  library,
  onClose,
}: {
  library: Library;
  onClose: () => void;
}) {
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
        <ActionButton>
          <Plus size={14} />
          新建分组
        </ActionButton>
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
