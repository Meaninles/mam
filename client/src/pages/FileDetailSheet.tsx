import type { FileNode } from '../data';
import { ActionButton, DenseRow, Sheet } from '../components/Shared';

export function FileDetailSheet({
  item,
  onClose,
  onDeleteAsset,
  onDeleteEndpoint,
}: {
  item: FileNode;
  onClose: () => void;
  onDeleteAsset?: (item: FileNode) => void;
  onDeleteEndpoint?: (item: FileNode, endpointName: string) => void;
}) {
  return (
    <Sheet onClose={onClose} title={item.name}>
      <div className="sheet-section">
        <DenseRow label="文件类型" value={item.displayType} />
        <DenseRow label="路径" value={item.path} />
        <DenseRow label="最近修改" value={item.modifiedAt} />
        <DenseRow label="大小" value={item.size} />
      </div>
      <div className="sheet-section">
        {item.metadata.map((row) => (
          <DenseRow key={row.label} label={row.label} value={row.value} />
        ))}
      </div>
      <div className="sheet-section">
        {item.endpoints.map((endpoint) => (
          <div key={endpoint.name} className="sheet-endpoint-row">
            <DenseRow label={endpoint.name} tone={endpoint.tone} value={endpoint.state} />
            {item.type === 'file' && endpoint.state !== '缺失' ? (
              <ActionButton ariaLabel={`从 ${endpoint.name} 删除`} onClick={() => onDeleteEndpoint?.(item, endpoint.name)}>
                从 {endpoint.name} 删除
              </ActionButton>
            ) : null}
          </div>
        ))}
      </div>
      {onDeleteAsset ? (
        <div className="sheet-actions right">
          <ActionButton ariaLabel="删除资产" tone="danger" onClick={() => onDeleteAsset(item)}>
            删除资产
          </ActionButton>
        </div>
      ) : null}
    </Sheet>
  );
}
