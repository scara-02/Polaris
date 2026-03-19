import { useEffect, useState } from 'react';
import { Line } from 'react-chartjs-2';
import type { ChartData, ChartOptions } from 'chart.js';
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Title, Tooltip, Filler } from 'chart.js';

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Title, Tooltip, Filler);

export default function Analytics() {
  const [uplinks, setUplinks] = useState<number>(0);
  
  // Strongly type the ChartJS data structure
  const [chartData, setChartData] = useState<ChartData<'line'>>({
    labels: [],
    datasets: [{
      label: 'Concurrent IoT Uplinks',
      data: [],
      borderColor: '#10b981',
      backgroundColor: 'rgba(16, 185, 129, 0.1)',
      fill: true,
      tension: 0.4,
      pointRadius: 0
    }]
  });

  const chartOptions: ChartOptions<'line'> = {
    responsive: true,
    maintainAspectRatio: false,
    animation: false,
    scales: {
      x: { display: false },
      y: { beginAtZero: true, grid: { color: '#334155' } }
    }
  };

  useEffect(() => {
    const fetchMetrics = async () => {
      try {
        const res = await fetch('http://localhost:6080/api/v1/metrics/connections');
        const json: { active_uplinks: number } = await res.json();
        
        const currentUplinks = json.active_uplinks || 0;
        setUplinks(currentUplinks);
        
        const now = new Date().toLocaleTimeString();
        setChartData((prev) => {
          const newLabels = [...(prev.labels as string[]), now].slice(-60);
          const newData = [...prev.datasets[0].data, currentUplinks].slice(-60);
          
          return {
            labels: newLabels,
            datasets: [{
              ...prev.datasets[0],
              data: newData
            }]
          };
        });
      } catch (err) {
        // Silent catch for polling to prevent console spam
      }
    };

    const interval = setInterval(fetchMetrics, 1000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="p-8 h-full flex flex-col">
      <div className="mb-8 text-center bg-slate-800 p-6 rounded-xl border border-slate-700 max-w-sm mx-auto">
        <div className={`text-6xl font-bold ${uplinks > 20000 ? 'text-red-500' : 'text-emerald-500'}`}>
          {uplinks.toLocaleString()}
        </div>
        <div className="text-slate-400 mt-2 uppercase tracking-widest text-sm">Active WebSockets</div>
      </div>

      <div className="flex-1 min-h-[400px] bg-slate-800 p-6 rounded-xl border border-slate-700">
        <Line data={chartData} options={chartOptions} />
      </div>
    </div>
  );
}