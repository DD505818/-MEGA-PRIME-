import { useEffect, useRef } from 'react';
import { GridStack } from 'gridstack';
import 'gridstack/dist/gridstack.min.css';
import { Panel } from './Panel';

const cards = [
  { id: 'alpha', title: 'Alpha Router', value: 'Online' },
  { id: 'risk', title: 'Risk Engine', value: 'Healthy' },
  { id: 'latency', title: 'Latency', value: '3.8ms' },
  { id: 'fill', title: 'Fill Rate', value: '97.4%' },
];

export function DragAndDropLayout() {
  const gridRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!gridRef.current) return;
    const grid = GridStack.init(
      { float: true, cellHeight: 80, margin: 8, disableResize: true, acceptWidgets: false },
      gridRef.current,
    );

    return () => {
      grid.destroy(false);
    };
  }, []);

  return (
    <Panel title="Drag & Drop Layout" subtitle="GridStack powered operations layout">
      <div className="grid-stack" ref={gridRef}>
        {cards.map((card, index) => (
          <div
            key={card.id}
            className="grid-stack-item"
            gs-w="3"
            gs-h="1"
            gs-x={(index % 2) * 3}
            gs-y={Math.floor(index / 2)}
          >
            <div className="grid-stack-item-content widget-card">
              <span>{card.title}</span>
              <strong>{card.value}</strong>
            </div>
          </div>
        ))}
      </div>
    </Panel>
  );
}
