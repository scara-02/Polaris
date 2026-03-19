import { BrowserRouter as Router, Routes, Route, Link } from 'react-router-dom';
import MapDashboard from './pages/MapDashboard';
import SwarmTester from './pages/SwarmTester';
import Analytics from './pages/Analytics';

export default function App() {
  return (
    <Router>
      <div className="flex h-screen bg-slate-900 text-slate-200 font-sans overflow-hidden">
        {/* Shared Sidebar */}
        <div className="w-64 bg-slate-800 p-6 flex flex-col shadow-xl z-50">
          <h1 className="text-2xl font-bold text-blue-500 mb-1">🌌 POLARIS</h1>
          <p className="text-xs text-slate-400 mb-8">v3.0 Command Center</p>
          
          <nav className="flex flex-col gap-4">
            <Link to="/" className="hover:text-blue-400 transition">📍 Spatial Map</Link>
            <Link to="/analytics" className="hover:text-blue-400 transition">📈 Analytics</Link>
            <Link to="/swarm" className="hover:text-blue-400 transition">🚁 Swarm Tester</Link>
          </nav>
          
          <div className="mt-auto text-[10px] text-slate-500 text-center">
            Microservices Architecture Active
          </div>
        </div>

        {/* Dynamic Content Area */}
        <div className="flex-1 relative">
          <Routes>
            <Route path="/" element={<MapDashboard />} />
            <Route path="/analytics" element={<Analytics />} />
            <Route path="/swarm" element={<SwarmTester />} />
          </Routes>
        </div>
      </div>
    </Router>
  );
}