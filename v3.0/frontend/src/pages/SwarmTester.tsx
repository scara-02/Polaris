import { useState, useRef } from 'react';
import type { LogEntry, TelemetryPayload } from '../types/polaris';

export default function SwarmTester() {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [totalLaunched, setTotalLaunched] = useState(0);
  const activeTimers = useRef<ReturnType<typeof setInterval>[]>([]);
  const terminalRef = useRef<HTMLDivElement | null>(null);

  const addLog = (msg: string, type: LogEntry['type'] = 'info') => {
    const time = new Date().toISOString().split('T')[1].slice(0, 12);
    setLogs((prev) => [{ time, msg, type }, ...prev].slice(0, 200));
  };

  const bootDrone = (nodeId: string) => {
    const ws = new WebSocket('ws://localhost:6080/ws/telemetry');
    // eslint-disable-next-line react-hooks/purity
    let lat = 13.04 + Math.random() * 0.05;
    // eslint-disable-next-line react-hooks/purity
    let lon = 80.24 + Math.random() * 0.05;
    let targetLat: number | null = null;
    let targetLon: number | null = null;

    ws.onopen = () => {
      addLog(`Uplink established: ${nodeId}`, 'success');

      const timer = setInterval(() => {
        if (ws.readyState === WebSocket.OPEN) {
          if (targetLat !== null && targetLon !== null) {
            lat += targetLat > lat ? 0.001 : -0.001;
            lon += targetLon > lon ? 0.001 : -0.001;
          }

          const payload: TelemetryPayload = {
            tenant_id: 'alpha_logistics',
            node_id: nodeId,
            asset_class: 16,
            lat,
            lon,
            status: 'idle',
            battery: 100,
          };
          ws.send(JSON.stringify(payload));
        }
      }, 1000);

      activeTimers.current.push(timer);
    };

    ws.onmessage = (event) => {
      const msg = JSON.parse(event.data);
      if (msg.directive === 'RELOCATE') {
        addLog(`⚡ DIRECTIVE: ${nodeId} → RELOCATE`, 'warning');
        targetLat = msg.target_lat;
        targetLon = msg.target_lon;
      }
    };

    ws.onerror = () => {
      addLog(`Connection failed: ${nodeId}`, 'danger');
    };
  };

  const launchSwarm = (count: number) => {
    addLog(`▶ Initiating swarm launch sequence — ${count} units`, 'info');
    setTotalLaunched((prev) => prev + count);
    for (let i = 1; i <= count; i++) {
      setTimeout(
        () => bootDrone(`DRONE-${Math.floor(Math.random() * 10000).toString().padStart(4, '0')}`),
        i * 50
      );
    }
  };

  const logColors: Record<LogEntry['type'], string> = {
    info: 'var(--accent-cyan)',
    success: 'var(--accent-violet)',
    warning: 'var(--accent-amber)',
    danger: 'var(--accent-coral)',
  };

  const logPrefixes: Record<LogEntry['type'], string> = {
    info: 'INF',
    success: 'ACK',
    warning: 'WRN',
    danger: 'ERR',
  };

  const buttons = [
    { count: 5, label: 'Launch 5', variant: 'default' as const },
    { count: 50, label: 'Launch 50', variant: 'elevated' as const },
    { count: 500, label: 'Stress Test (500)', variant: 'danger' as const },
  ];

  const buttonStyles: Record<string, React.CSSProperties> = {
    default: {
      background: 'linear-gradient(135deg, rgba(167, 139, 250, 0.2), rgba(129, 140, 248, 0.12))',
      border: '1px solid var(--border-accent)',
      color: 'var(--accent-violet)',
    },
    elevated: {
      background: 'linear-gradient(135deg, rgba(34, 211, 168, 0.2), rgba(52, 211, 153, 0.12))',
      border: '1px solid rgba(34, 211, 168, 0.3)',
      color: 'var(--accent-cyan)',
    },
    danger: {
      background: 'linear-gradient(135deg, rgba(255, 123, 123, 0.2), rgba(239, 68, 68, 0.12))',
      border: '1px solid rgba(255, 123, 123, 0.3)',
      color: 'var(--accent-coral)',
    },
  };

  return (
    <div
      id="swarm-tester-page"
      className="animate-fade-in"
      style={{
        padding: 32,
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        gap: 24,
        position: 'relative',
        zIndex: 1,
      }}
    >
      {/* ── Page Header ── */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <div>
          <h1
            style={{
              margin: 0,
              fontSize: 24,
              fontWeight: 700,
              color: 'var(--text-primary)',
              letterSpacing: '-0.5px',
            }}
          >
            Swarm Ingestion Test
          </h1>
          <p
            style={{
              margin: '4px 0 0',
              fontSize: 13,
              color: 'var(--text-secondary)',
            }}
          >
            Simulate drone fleet WebSocket connections against the IoT Gateway
          </p>
        </div>

        {/* Stats Badge */}
        <div
          id="swarm-launched-count"
          style={{
            background: 'var(--bg-surface)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 12,
            padding: '12px 20px',
            textAlign: 'center',
          }}
        >
          <div
            style={{
              fontSize: 28,
              fontWeight: 800,
              fontFamily: 'var(--font-mono)',
              color: 'var(--accent-violet)',
              lineHeight: 1,
            }}
          >
            {totalLaunched}
          </div>
          <div
            style={{
              fontSize: 9,
              fontWeight: 700,
              color: 'var(--text-secondary)',
              letterSpacing: '2px',
              textTransform: 'uppercase' as const,
              marginTop: 4,
            }}
          >
            Total Launched
          </div>
        </div>
      </div>

      {/* ── Launch Buttons ── */}
      <div
        id="swarm-launch-controls"
        className="stagger-children"
        style={{ display: 'flex', gap: 12 }}
      >
        {buttons.map(({ count, label, variant }) => (
          <button
            key={count}
            id={`btn-launch-${count}`}
            onClick={() => launchSwarm(count)}
            style={{
              ...buttonStyles[variant],
              padding: '12px 24px',
              borderRadius: 10,
              fontSize: 13,
              fontWeight: 700,
              fontFamily: 'var(--font-sans)',
              cursor: 'pointer',
              transition: 'all 0.2s cubic-bezier(0.16, 1, 0.3, 1)',
              letterSpacing: '0.5px',
            }}
            onMouseEnter={(e) => {
              (e.target as HTMLElement).style.transform = 'translateY(-2px)';
              (e.target as HTMLElement).style.boxShadow = '0 4px 20px rgba(0,0,0,0.3)';
            }}
            onMouseLeave={(e) => {
              (e.target as HTMLElement).style.transform = 'translateY(0)';
              (e.target as HTMLElement).style.boxShadow = 'none';
            }}
          >
            {label}
          </button>
        ))}
      </div>

      {/* ── Terminal Window ── */}
      <div
        id="swarm-terminal"
        style={{
          flex: 1,
          background: '#06080f',
          border: '1px solid var(--border-subtle)',
          borderRadius: 16,
          overflow: 'hidden',
          display: 'flex',
          flexDirection: 'column',
          minHeight: 0,
        }}
      >
        {/* Terminal Header */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 8,
            padding: '10px 16px',
            borderBottom: '1px solid var(--border-subtle)',
            background: 'var(--bg-surface)',
          }}
        >
          <div
            style={{
              display: 'flex',
              gap: 6,
            }}
          >
            <div style={{ width: 10, height: 10, borderRadius: '50%', background: '#ff5f57' }} />
            <div style={{ width: 10, height: 10, borderRadius: '50%', background: '#febc2e' }} />
            <div style={{ width: 10, height: 10, borderRadius: '50%', background: '#28c840' }} />
          </div>
          <span
            style={{
              fontSize: 11,
              fontWeight: 600,
              color: 'var(--text-tertiary)',
              fontFamily: 'var(--font-mono)',
              marginLeft: 8,
              letterSpacing: '0.5px',
            }}
          >
            polaris-swarm-terminal
          </span>
          <span
            style={{
              marginLeft: 'auto',
              fontSize: 10,
              fontFamily: 'var(--font-mono)',
              color: 'var(--text-tertiary)',
            }}
          >
            {logs.length} entries
          </span>
        </div>

        {/* Terminal Body */}
        <div
          ref={terminalRef}
          style={{
            flex: 1,
            overflowY: 'auto',
            padding: '12px 16px',
            fontFamily: 'var(--font-mono)',
            fontSize: 12,
            lineHeight: 1.7,
          }}
        >
          {logs.length === 0 && (
            <div style={{ color: 'var(--text-tertiary)', fontStyle: 'italic' }}>
              Waiting for swarm launch...
              <span style={{ animation: 'terminal-blink 1s step-end infinite' }}>▌</span>
            </div>
          )}
          {logs.map((l, i) => (
            <div
              key={i}
              style={{
                color: logColors[l.type],
                display: 'flex',
                gap: 8,
                animation: i === 0 ? 'fadeInUp 0.2s ease-out' : undefined,
              }}
            >
              <span style={{ color: 'var(--text-tertiary)', userSelect: 'none', flexShrink: 0 }}>
                {l.time}
              </span>
              <span
                style={{
                  color: logColors[l.type],
                  opacity: 0.6,
                  fontWeight: 600,
                  userSelect: 'none',
                  flexShrink: 0,
                  width: 28,
                }}
              >
                {logPrefixes[l.type]}
              </span>
              <span>{l.msg}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}