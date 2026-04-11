import { useEffect, useState } from 'react';
import { Star } from 'lucide-react';
import type { FileCenterColorLabel, FileCenterEntry } from '../lib/fileCenterApi';
import { ActionButton, DenseRow, Sheet, TonePill } from '../components/Shared';

const COLOR_OPTIONS: FileCenterColorLabel[] = ['无', '红标', '黄标', '绿标', '蓝标', '紫标'];

export function FileDetailSheet({
  item,
  onClose,
  onSaveAnnotations,
}: {
  item: FileCenterEntry;
  onClose: () => void;
  onSaveAnnotations: (input: {
    id: string;
    rating: number;
    colorLabel: FileCenterColorLabel;
    tags: string[];
  }) => Promise<void> | void;
}) {
  const [rating, setRating] = useState(item.rating);
  const [colorLabel, setColorLabel] = useState<FileCenterColorLabel>(item.colorLabel);

  useEffect(() => {
    setRating(item.rating);
    setColorLabel(item.colorLabel);
  }, [item]);

  const hasChanged = rating !== item.rating || colorLabel !== item.colorLabel;

  const handleSaveAnnotations = async () => {
    await onSaveAnnotations({
      id: item.id,
      rating,
      colorLabel,
      tags: item.tags,
    });
    onClose();
  };

  return (
    <Sheet onClose={onClose} title={item.name}>
      <div className="sheet-section">
        <DenseRow label="文件类型" value={item.displayType} />
        <DenseRow label="路径" value={item.path} />
        <DenseRow label="来源" value={item.sourceLabel || '统一资产'} />
        <DenseRow label="创建时间" value={item.createdAt} />
        <DenseRow label="最近修改" value={item.modifiedAt} />
        <DenseRow label="大小" value={item.size} />
      </div>

      <div className="sheet-section">
        <DenseRow label="最近任务" tone={item.lastTaskTone} value={item.lastTaskText} />
        <div className="endpoint-row">
          {item.badges.map((badge) => (
            <TonePill key={badge} tone="info">
              {badge}
            </TonePill>
          ))}
          {item.riskTags.map((tag) => (
            <TonePill key={tag} tone="warning">
              {tag}
            </TonePill>
          ))}
        </div>
      </div>

      <div className="sheet-section">
        {item.metadata.map((row) => (
          <DenseRow key={row.label} label={row.label} value={row.value} />
        ))}
      </div>

      {item.type === 'file' ? (
        <div className="sheet-section">
          <div className="annotation-section">
            <strong>星级</strong>
            <div className="rating-editor" role="group" aria-label="资产星级">
              <button
                aria-label="无评级"
                className={rating === 0 ? 'active' : ''}
                type="button"
                onClick={() => setRating(0)}
              >
                无评级
              </button>
              {Array.from({ length: 5 }, (_, index) => {
                const value = index + 1;
                return (
                  <button
                    key={value}
                    aria-label={`${value} 星`}
                    className={rating === value ? 'active' : ''}
                    type="button"
                    onClick={() => setRating(value)}
                  >
                    <span className="file-rating">
                      {Array.from({ length: value }, (_, starIndex) => (
                        <Star key={`${value}-${starIndex}`} size={12} fill="currentColor" />
                      ))}
                    </span>
                  </button>
                );
              })}
            </div>
          </div>

          <div className="annotation-section">
            <strong>色标</strong>
            <div className="color-label-editor">
              {COLOR_OPTIONS.map((option) => (
                <button
                  key={option}
                  className={option === colorLabel ? 'active' : ''}
                  type="button"
                  onClick={() => setColorLabel(option)}
                >
                  <span className={`color-label-dot ${resolveColorClass(option)}`} />
                  <span>{option}</span>
                </button>
              ))}
            </div>
          </div>

          <div className="sheet-actions right">
            <ActionButton tone="primary" onClick={() => void handleSaveAnnotations()}>
              {hasChanged ? '保存标记' : '已保存'}
            </ActionButton>
          </div>
        </div>
      ) : null}

      {item.notes ? (
        <div className="sheet-section">
          <DenseRow label="备注" value={item.notes} />
        </div>
      ) : null}
    </Sheet>
  );
}

function resolveColorClass(colorLabel: FileCenterColorLabel) {
  if (colorLabel === '红标') return 'red';
  if (colorLabel === '黄标') return 'yellow';
  if (colorLabel === '绿标') return 'green';
  if (colorLabel === '蓝标') return 'blue';
  if (colorLabel === '紫标') return 'purple';
  return 'none';
}
