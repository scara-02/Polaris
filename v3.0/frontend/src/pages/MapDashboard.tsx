import { useEffect, useRef, useState } from 'react';
import L from 'leaflet';
import type { MatchResult } from '../types/polaris';

interface MapState {
  map: L.Map;
  markersLayer: L.LayerGroup;
  hotspotsLayer: L.LayerGroup;
}

export default function MapDashboard() {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const mapRef = useRef<MapState | null>(null);

  const [activeNodes, setActiveNodes] = useState(0);
  const [showPredicted, setShowPredicted] = useState(true);
  const [hotZoneCount, setHotZoneCount] = useState(0);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    // Leaflet adds a _leaflet_id to the container when initialized.
    // This is the canonical way to prevent double-init in StrictMode.
    if ((container as any)._leaflet_id) {
      // Map already exists on this element, re-bind ref
      return;
    }

    const map = L.map(container, {
      zoomControl: true,
      attributionControl: true,
    }).setView([13.04, 80.24], 11);

    L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
      subdomains: 'abcd',
      maxZoom: 20,
      detectRetina: false,
    }).addTo(map);

    const markersLayer = L.layerGroup().addTo(map);
    const hotspotsLayer = L.layerGroup().addTo(map);
    mapRef.current = { map, markersLayer, hotspotsLayer };

    // Force size recalculation after React renders
    const invalidateSize = () => map.invalidateSize({ animate: false });
    requestAnimationFrame(() => {
      invalidateSize();
      setTimeout(invalidateSize, 150);
      setTimeout(invalidateSize, 400);
    });

    // ResizeObserver for dynamic layout
    const ro = new ResizeObserver(invalidateSize);
    ro.observe(container);

    const droneIcon = L.divIcon({
      html: '<div style="font-size: 24px;">🚁</div>',
      className: 'drone-marker',
      iconSize: [30, 30],
    });

    const fetchSpatialData = async () => {
      try {
        const res = await fetch(
          'http://localhost:6081/api/v1/nodes/match?lat=13.04&lon=80.24&radius_km=50&class=16&tenant_id=alpha_logistics'
        );
        const json = await res.json();
        const data: MatchResult[] = Array.isArray(json.data) ? json.data : [];

        if (mapRef.current) {
          mapRef.current.markersLayer.clearLayers();
          data.forEach((node) => {
            L.marker([node.lat, node.lon], { icon: droneIcon })
              .bindPopup(
                `<div style="text-align:center">
                  <strong style="color:#a78bfa;font-size:14px">${node.node_id}</strong><br/>
                  <span style="color:#a0aec8;font-size:12px">Distance: ${node.distance_km.toFixed(2)} km</span>
                </div>`
              )
              .addTo(mapRef.current!.markersLayer);
          });
          setActiveNodes(data.length);
        }
      } catch (e) {
        console.error('Drone fetch error', e);
      }
    };

    const fetchPredictedZones = async () => {
      try {
        const res = await fetch('http://localhost:6081/api/v1/zones/predicted');
        const json = await res.json();

        if (json.status === 'success' && mapRef.current) {
          const { hotspotsLayer } = mapRef.current;
          hotspotsLayer.clearLayers();

          let count = 0;
          json.data.forEach((zone: { lat: number; lon: number; radius_km: number; id: string; status: string }) => {
            count++;
            L.circle([zone.lat, zone.lon], {
              color: '#ff7b7b',
              weight: 2,
              fillColor: '#ff7b7b',
              fillOpacity: 0.3,
              radius: zone.radius_km * 1000,
            })
              .bindPopup(
                `<div style="text-align:center">
                  <strong style="color:#ff7b7b;font-size:14px">🤖 AI Forecast</strong><br/>
                  <span style="color:#a0aec8">Zone: ${zone.id}</span><br/>
                  <span style="color:#ff7b7b;font-weight:600">${zone.status}</span>
                </div>`
              )
              .addTo(hotspotsLayer);
          });
          setHotZoneCount(count);
          console.log('Rebalancing Data:', json.meta);
        }
      } catch (err) {
        console.error('AI fetch failed', err);
      }
    };

    const i1 = setInterval(fetchSpatialData, 2000);
    const i2 = setInterval(fetchPredictedZones, 5000);
    fetchPredictedZones();
    fetchSpatialData();

    return () => {
      ro.disconnect();
      clearInterval(i1);
      clearInterval(i2);
      if (mapRef.current) {
        mapRef.current.map.remove();
        mapRef.current = null;
      }
    };
  }, []);

  useEffect(() => {
    if (!mapRef.current) return;
    if (showPredicted) mapRef.current.map.addLayer(mapRef.current.hotspotsLayer);
    else mapRef.current.map.removeLayer(mapRef.current.hotspotsLayer);
  }, [showPredicted]);

  return (
    <div
      id="map-dashboard"
      style={{
        position: 'absolute',
        top: 0,
        left: 0,
        width: '100%',
        height: '100%',
        background: 'var(--bg-base)',
        zIndex: 1,
      }}
    >
      {/* ── The Map Container ── */}
      <div
        ref={containerRef}
        id="leaflet-map-container"
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          width: '100%',
          height: '100%',
        }}
      />

      {/* ── Floating Stats Panel ── */}
      <div
        id="map-overlay-controls"
        style={{
          position: 'fixed',
          top: 24,
          left: 284,
          zIndex: 9999,
          display: 'flex',
          flexDirection: 'column',
          gap: 12,
          pointerEvents: 'none',
        }}
        className="stagger-children"
      >
        <div
          id="stat-active-drones"
          style={{
            padding: '20px 24px',
            borderRadius: 16,
            pointerEvents: 'auto',
            minWidth: 180,
            background: '#111827',
            border: '1px solid #2d3a56',
            boxShadow: '0 8px 32px rgba(0,0,0,0.6), 0 0 0 1px rgba(167,139,250,0.08)',
          }}
        >
          <div
            style={{
              fontSize: 42,
              fontWeight: 900,
              fontFamily: 'var(--font-mono)',
              background: 'linear-gradient(135deg, #22d3a8, #34d399)',
              WebkitBackgroundClip: 'text',
              WebkitTextFillColor: 'transparent',
              lineHeight: 1,
            }}
          >
            {activeNodes}
          </div>
          <div
            style={{
              fontSize: 10,
              fontWeight: 700,
              color: 'var(--text-secondary)',
              letterSpacing: '2.5px',
              textTransform: 'uppercase' as const,
              marginTop: 8,
            }}
          >
            Active Drones
          </div>
        </div>

        <div
          id="stat-hot-zones"
          style={{
            padding: '16px 24px',
            borderRadius: 16,
            pointerEvents: 'auto',
            minWidth: 180,
            background: '#111827',
            border: '1px solid #2d3a56',
            boxShadow: '0 8px 32px rgba(0,0,0,0.6), 0 0 0 1px rgba(167,139,250,0.08)',
          }}
        >
          <div
            style={{
              fontSize: 28,
              fontWeight: 800,
              fontFamily: 'var(--font-mono)',
              color: showPredicted ? 'var(--accent-coral)' : 'var(--text-tertiary)',
              lineHeight: 1,
              transition: 'color 0.3s',
            }}
          >
            {hotZoneCount}
          </div>
          <div
            style={{
              fontSize: 10,
              fontWeight: 700,
              color: 'var(--text-secondary)',
              letterSpacing: '2.5px',
              textTransform: 'uppercase' as const,
              marginTop: 6,
            }}
          >
            AI Hot Zones
          </div>
        </div>

        <button
          id="toggle-ai-forecast"
          onClick={() => setShowPredicted((p) => !p)}
          style={{
            pointerEvents: 'auto',
            padding: '14px 20px',
            borderRadius: 12,
            border: showPredicted
              ? '1px solid rgba(255, 123, 123, 0.4)'
              : '1px solid #2d3a56',
            background: showPredicted ? '#1f1520' : '#111827',
            color: showPredicted ? '#ff7b7b' : 'var(--text-secondary)',
            fontSize: 11,
            fontWeight: 700,
            fontFamily: 'var(--font-sans)',
            letterSpacing: '1.5px',
            cursor: 'pointer',
            transition: 'all 0.25s cubic-bezier(0.16, 1, 0.3, 1)',
            textTransform: 'uppercase' as const,
          }}
        >
          {showPredicted ? '● HIDE AI FORECAST' : '○ SHOW AI FORECAST'}
        </button>
      </div>
    </div>
  );
}