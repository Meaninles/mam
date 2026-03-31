import type { FileNode } from '../data';
import { DenseRow, Sheet } from '../components/Shared';

export function FileDetailSheet({ item, onClose }: { item: FileNode; onClose: () => void }) {
  return (
    <Sheet onClose={onClose} title={item.name}>
      <div className="sheet-section">
        {item.metadata.map((row) => (
          <DenseRow key={row.label} label={row.label} value={row.value} />
        ))}
      </div>
      <div className="sheet-section">
        {item.endpoints.map((endpoint) => (
          <DenseRow key={endpoint.name} label={endpoint.name} tone={endpoint.tone} value={endpoint.state} />
        ))}
      </div>
    </Sheet>
  );
}
