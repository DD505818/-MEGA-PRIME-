import { useState } from 'react';
import { Panel } from './Panel';

const agents = ['Execution Agent', 'Risk Agent', 'Hedging Agent', 'Sentiment Agent'];

export function AIAgentHub() {
  const [active, setActive] = useState<Record<string, boolean>>(
    Object.fromEntries(agents.map((name) => [name, true])),
  );

  return (
    <Panel title="AI Agent Hub" subtitle="Coordinated autonomous controls">
      <div className="agent-list">
        {agents.map((agent) => (
          <label key={agent} className="switch-row">
            <span>{agent}</span>
            <input
              type="checkbox"
              checked={active[agent]}
              onChange={() => setActive((prev) => ({ ...prev, [agent]: !prev[agent] }))}
            />
          </label>
        ))}
      </div>
    </Panel>
  );
}
