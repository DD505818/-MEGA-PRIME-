import { useEffect, useState } from 'react';
import { Panel } from './Panel';

const clockZones = [
  { city: 'New York', zone: 'America/New_York' },
  { city: 'London', zone: 'Europe/London' },
  { city: 'Tokyo', zone: 'Asia/Tokyo' },
];

export function InternationalClocks() {
  const [now, setNow] = useState(new Date());

  useEffect(() => {
    const timer = setInterval(() => setNow(new Date()), 1000);
    return () => clearInterval(timer);
  }, []);

  return (
    <Panel title="International Clocks" subtitle="Global market sessions">
      <div className="clock-grid">
        {clockZones.map((zone) => (
          <article key={zone.city} className="clock-card">
            <h3>{zone.city}</h3>
            <p>
              {new Intl.DateTimeFormat('en-US', {
                timeZone: zone.zone,
                hour: '2-digit',
                minute: '2-digit',
                second: '2-digit',
                hour12: false,
              }).format(now)}
            </p>
          </article>
        ))}
      </div>
    </Panel>
  );
}
