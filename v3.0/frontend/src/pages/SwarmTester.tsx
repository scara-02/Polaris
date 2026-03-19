import { useState, useRef } from 'react';
import type { LogEntry, TelemetryPayload } from '../types/polaris';

export default function SwarmTester() {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  // Store interval IDs so we can clear them when unmounting or resetting
  const activeTimers = useRef<ReturnType<typeof setInterval>[]>([]);

  const addLog = (msg: string, type: LogEntry['type'] = 'info') => {
    const time = new Date().toISOString().split('T')[1].slice(0, 12);
    setLogs((prev) => [{ time, msg, type }, ...prev].slice(0, 100)); 
  };

  const bootDrone = (nodeId: string) => {
    const ws = new WebSocket("ws://localhost:6080/ws/telemetry");
    let lat = 13.04 + (Math.random() * 0.05);
    let lon = 80.24 + (Math.random() * 0.05);
    let targetLat: number | null = null;
    let targetLon: number | null = null;

    ws.onopen = () => {
      addLog(`Uplink established: ${nodeId}`, 'success');
      
      const timer = setInterval(() => {
        if (ws.readyState === WebSocket.OPEN) {
          if (targetLat !== null && targetLon !== null) {
            lat += (targetLat > lat) ? 0.001 : -0.001;
            lon += (targetLon > lon) ? 0.001 : -0.001;
          }

          const payload: TelemetryPayload = {
            tenant_id: "alpha_logistics",
            node_id: nodeId,
            asset_class: 16, 
            lat,
            lon,
            status: "idle",
            battery: 100
          };
          ws.send(JSON.stringify(payload));
        }
      }, 1000);
      
      activeTimers.current.push(timer);
    };

    ws.onmessage = (event) => {
      const msg = JSON.parse(event.data);
      if (msg.directive === "RELOCATE") {
        addLog(`⚠️ COMMAND RECEIVED: ${nodeId} relocating`, 'warning');
        targetLat = msg.target_lat;
        targetLon = msg.target_lon;
      }
    };
  };

  const launchSwarm = (count: number) => {
    addLog(`Initiating launch sequence for ${count} drones...`, 'info');
    for (let i = 1; i <= count; i++) {
      setTimeout(() => bootDrone(`DRONE-${Math.floor(Math.random() * 10000)}`), i * 50);
    }
  };

  return (
    <div className="p-8 max-w-4xl mx-auto h-full flex flex-col">
      <h2 className="text-2xl font-bold mb-6">🚁 Swarm Ingestion Test</h2>
      
      <div className="flex gap-4 mb-6">
        <button onClick={() => launchSwarm(5)} className="bg-blue-600 hover:bg-blue-500 px-6 py-2 rounded font-bold">Launch 5</button>
        <button onClick={() => launchSwarm(50)} className="bg-purple-600 hover:bg-purple-500 px-6 py-2 rounded font-bold">Launch 50</button>
        <button onClick={() => launchSwarm(500)} className="bg-red-600 hover:bg-red-500 px-6 py-2 rounded font-bold">Stress Test (500)</button>
      </div>

      <div className="flex-1 bg-black border border-slate-700 rounded p-4 overflow-y-auto font-mono text-sm">
        {logs.map((l, i) => (
          <div key={i} className={
            l.type === 'warning' ? 'text-amber-500' : 
            l.type === 'success' ? 'text-blue-400' : 
            l.type === 'danger' ? 'text-red-500' : 'text-emerald-500'
          }>
            <span className="text-slate-500">[{l.time}]</span> {l.msg}
          </div>
        ))}
      </div>
    </div>
  );
}