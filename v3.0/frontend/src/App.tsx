import { BrowserRouter as Router, Routes, Route, Link, useLocation } from 'react-router-dom';
import MapDashboard from './pages/MapDashboard';
import SwarmTester from './pages/SwarmTester';
import Analytics from './pages/Analytics';
import './App.css';

function Sidebar() {
  const location = useLocation();

  const navItems = [
    { path: '/', icon: '🗺️', label: 'Spatial Command' },
    { path: '/analytics', icon: '📊', label: 'Analytics' },
    { path: '/swarm', icon: '🚁', label: 'Swarm Tester' },
  ];

  return (
    <aside className="polaris-sidebar" id="polaris-sidebar">
      {/* Logo */}
      <div className="sidebar-logo">
        <div className="sidebar-logo-text">
          <span className="sidebar-logo-icon">⚡</span>
          POLARIS
        </div>
        <div className="sidebar-version">COMMAND CENTER v3.0</div>
      </div>

      <div className="sidebar-divider" />

      {/* Navigation */}
      <nav className="sidebar-nav stagger-children" id="main-navigation">
        <div className="sidebar-section-title">Operations</div>
        {navItems.map((item) => (
          <Link
            key={item.path}
            to={item.path}
            id={`nav-${item.label.toLowerCase().replace(/\s+/g, '-')}`}
            className={`sidebar-nav-link ${location.pathname === item.path ? 'active' : ''}`}
          >
            <span className="sidebar-nav-icon">{item.icon}</span>
            <span className="sidebar-nav-label">{item.label}</span>
          </Link>
        ))}
      </nav>

      {/* System Status */}
      <div className="sidebar-status animate-fade-in-up" id="system-status">
        <div className="sidebar-section-title" style={{ padding: '0 0 10px' }}>
          System Status
        </div>
        <div className="sidebar-status-row">
          <span className="status-dot online" />
          <span>Spatial Engine</span>
        </div>
        <div className="sidebar-status-row">
          <span className="status-dot online" />
          <span>IoT Gateway</span>
        </div>
        <div className="sidebar-status-row">
          <span className="status-dot standby" />
          <span>GNN Sidecar</span>
        </div>
      </div>

      <div className="sidebar-footer">
        Microservices Architecture
      </div>
    </aside>
  );
}

export default function App() {
  return (
    <Router>
      <div style={{ display: 'flex', width: '100%', height: '100vh', overflow: 'hidden' }}>
        <Sidebar />
        <main className="polaris-content">
          <Routes>
            <Route path="/" element={<MapDashboard />} />
            <Route path="/analytics" element={<Analytics />} />
            <Route path="/swarm" element={<SwarmTester />} />
          </Routes>
        </main>
      </div>
    </Router>
  );
}