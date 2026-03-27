import { anomalies } from '../data/mockData';
import { Panel } from './Panel';

export function AnomalyMonitor() {
  return (
    <Panel title="Anomaly Monitor" subtitle="Live alerting pipeline">
      <div className="alerts">
        {anomalies.map((anomaly) => (
          <article key={anomaly.id} className={`alert ${anomaly.severity}`}>
            <h4>{anomaly.label}</h4>
            <p>{anomaly.time}</p>
          </article>
        ))}
      </div>
    </Panel>
  );
}
