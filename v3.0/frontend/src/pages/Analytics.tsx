import { useEffect, useState } from 'react';
import { Line } from 'react-chartjs-2';
import type { ChartData, ChartOptions } from 'chart.js';
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Filler,
  Legend,
} from 'chart.js';

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Title, Tooltip, Filler, Legend);

export default function Analytics() {
  const [uplinks, setUplinks] = useState<number>(0);
  const [peak, setPeak] = useState<number>(0);

  const [chartData, setChartData] = useState<ChartData<'line'>>({
    labels: [],
    datasets: [
      {
        label: 'Concurrent IoT Uplinks',
        data: [],
        borderColor: '#22d3a8',
        backgroundColor: 'rgba(34, 211, 168, 0.08)',
        borderWidth: 2,
        fill: true,
        tension: 0.4,
        pointRadius: 0,
        pointHoverRadius: 6,
        pointHoverBackgroundColor: '#22d3a8',
        pointHoverBorderColor: '#0a0e1a',
        pointHoverBorderWidth: 3,
      },
    ],
  });

  const chartOptions: ChartOptions<'line'> = {
    responsive: true,
    maintainAspectRatio: false,
    animation: false,
    interaction: {
      intersect: false,
      mode: 'index',
    },
    plugins: {
      legend: {
        display: true,
        labels: {
          color: '#a0aec8',
          font: { family: "'Inter', sans-serif", size: 12, weight: 500 },
          usePointStyle: true,
          pointStyleWidth: 8,
          padding: 20,
        },
      },
      tooltip: {
        backgroundColor: '#151c30',
        titleColor: '#f0f4ff',
        bodyColor: '#a0aec8',
        borderColor: '#242f52',
        borderWidth: 1,
        cornerRadius: 10,
        padding: 12,
        titleFont: { family: "'Inter', sans-serif", size: 13, weight: 600 },
        bodyFont: { family: "'JetBrains Mono', monospace", size: 12 },
        caretSize: 6,
        displayColors: false,
      },
    },
    scales: {
      x: {
        display: true,
        grid: { color: 'rgba(36, 47, 82, 0.5)', lineWidth: 1 },
        ticks: {
          color: '#6a7a96',
          font: { family: "'Inter', sans-serif", size: 10 },
          maxTicksLimit: 10,
        },
        border: { color: 'transparent' },
      },
      y: {
        beginAtZero: true,
        grid: { color: 'rgba(36, 47, 82, 0.5)', lineWidth: 1 },
        ticks: {
          color: '#6a7a96',
          font: { family: "'JetBrains Mono', monospace", size: 11 },
          padding: 8,
        },
        border: { color: 'transparent' },
      },
    },
  };

  useEffect(() => {
    const fetchMetrics = async () => {
      try {
        const res = await fetch('http://localhost:6080/api/v1/metrics/connections');
        const json: { active_uplinks: number } = await res.json();

        const currentUplinks = json.active_uplinks || 0;
        setUplinks(currentUplinks);
        setPeak((prev) => Math.max(prev, currentUplinks));

        const now = new Date().toLocaleTimeString();
        setChartData((prev) => {
          const newLabels = [...(prev.labels as string[]), now].slice(-60);
          const newData = [...prev.datasets[0].data, currentUplinks].slice(-60);

          return {
            labels: newLabels,
            datasets: [{ ...prev.datasets[0], data: newData }],
          };
        });
      } catch {
        // Silent catch for polling
      }
    };

    const interval = setInterval(fetchMetrics, 1000);
    return () => clearInterval(interval);
  }, []);

  const getStatusColor = (value: number) => {
    if (value > 40000) return { color: 'var(--accent-coral)', label: 'CRITICAL' };
    if (value > 20000) return { color: 'var(--accent-amber)', label: 'HIGH LOAD' };
    return { color: 'var(--accent-cyan)', label: 'NOMINAL' };
  };

  const status = getStatusColor(uplinks);

  return (
    <div
      id="analytics-page"
      className="animate-fade-in"
      style={{
        padding: 32,
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        gap: 24,
        position: 'relative',
        zIndex: 1,
        overflow: 'auto',
      }}
    >
      {/* ── Page Header ── */}
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
          Real-Time Analytics
        </h1>
        <p
          style={{
            margin: '4px 0 0',
            fontSize: 13,
            color: 'var(--text-secondary)',
          }}
        >
          Gateway ingestion telemetry — 1s polling interval
        </p>
      </div>

      {/* ── Stat Cards Row ── */}
      <div
        id="analytics-stat-cards"
        className="stagger-children"
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(3, 1fr)',
          gap: 16,
        }}
      >
        {/* Active Connections */}
        <div
          id="stat-active-connections"
          style={{
            background: 'var(--bg-surface)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 16,
            padding: '24px 28px',
            position: 'relative',
            overflow: 'hidden',
          }}
        >
          <div
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              right: 0,
              height: 2,
              background: `linear-gradient(90deg, transparent, ${status.color}, transparent)`,
              opacity: 0.6,
            }}
          />
          <div
            style={{
              fontSize: 11,
              fontWeight: 600,
              color: 'var(--text-secondary)',
              letterSpacing: '1.5px',
              textTransform: 'uppercase',
              marginBottom: 8,
            }}
          >
            Active WebSockets
          </div>
          <div
            style={{
              fontSize: 40,
              fontWeight: 900,
              fontFamily: 'var(--font-mono)',
              color: status.color,
              lineHeight: 1,
              transition: 'color 0.3s',
            }}
          >
            {uplinks.toLocaleString()}
          </div>
        </div>

        {/* Peak Load */}
        <div
          id="stat-peak-load"
          style={{
            background: 'var(--bg-surface)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 16,
            padding: '24px 28px',
            position: 'relative',
            overflow: 'hidden',
          }}
        >
          <div
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              right: 0,
              height: 2,
              background: 'linear-gradient(90deg, transparent, var(--accent-violet), transparent)',
              opacity: 0.4,
            }}
          />
          <div
            style={{
              fontSize: 11,
              fontWeight: 600,
              color: 'var(--text-secondary)',
              letterSpacing: '1.5px',
              textTransform: 'uppercase',
              marginBottom: 8,
            }}
          >
            Peak Load
          </div>
          <div
            style={{
              fontSize: 40,
              fontWeight: 900,
              fontFamily: 'var(--font-mono)',
              color: 'var(--accent-violet)',
              lineHeight: 1,
            }}
          >
            {peak.toLocaleString()}
          </div>
        </div>

        {/* System Status */}
        <div
          id="stat-system-status"
          style={{
            background: 'var(--bg-surface)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 16,
            padding: '24px 28px',
            position: 'relative',
            overflow: 'hidden',
          }}
        >
          <div
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              right: 0,
              height: 2,
              background: `linear-gradient(90deg, transparent, ${status.color}, transparent)`,
              opacity: 0.4,
            }}
          />
          <div
            style={{
              fontSize: 11,
              fontWeight: 600,
              color: 'var(--text-secondary)',
              letterSpacing: '1.5px',
              textTransform: 'uppercase',
              marginBottom: 8,
            }}
          >
            System Status
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <div
              style={{
                width: 10,
                height: 10,
                borderRadius: '50%',
                background: status.color,
                boxShadow: `0 0 8px ${status.color}50`,
              }}
            />
            <span
              style={{
                fontSize: 18,
                fontWeight: 700,
                color: status.color,
                letterSpacing: '1px',
                transition: 'color 0.3s',
              }}
            >
              {status.label}
            </span>
          </div>
        </div>
      </div>

      {/* ── Chart Area ── */}
      <div
        id="analytics-chart-container"
        className="animate-fade-in-up"
        style={{
          flex: 1,
          minHeight: 350,
          background: 'var(--bg-surface)',
          border: '1px solid var(--border-subtle)',
          borderRadius: 16,
          padding: '24px 28px',
          position: 'relative',
          overflow: 'hidden',
        }}
      >
        <div
          style={{
            position: 'absolute',
            top: 0,
            left: 0,
            right: 0,
            height: 1,
            background: 'linear-gradient(90deg, transparent, var(--border-default), transparent)',
          }}
        />
        <Line data={chartData} options={chartOptions} />
      </div>
    </div>
  );
}