// import { useEffect, useRef, useState } from 'react';
// import L from 'leaflet';
// // @ts-ignore - leaflet.heat lacks official TS types
// import 'leaflet.heat';
// import 'leaflet/dist/leaflet.css';
// import type { MatchResult, ZonePrediction } from '../types/polaris';

// interface MapState {
//   map: L.Map;
//   markersLayer: L.LayerGroup;
//   heatLayer: any; 
//   hotspotsLayer: L.LayerGroup;
// }

// export default function MapDashboard() {
//   const mapRef = useRef<MapState | null>(null);
//   const [activeNodes, setActiveNodes] = useState<number>(0);

//   useEffect(() => {
//     if (mapRef.current) return;

//     const map = L.map('map-container', { zoomControl: false }).setView([13.04, 80.24], 12);

//     L.control.zoom({ position: 'topright' }).addTo(map);

//     L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
//       attribution: '&copy; CARTO',
//       subdomains: 'abcd',
//       maxZoom: 19
//     }).addTo(map);

//     const heatLayer = (L as any).heatLayer([], {
//       radius: 25,
//       blur: 15,
//       maxZoom: 14
//     }).addTo(map);

//     const markersLayer = L.layerGroup().addTo(map);
//     const hotspotsLayer = L.layerGroup().addTo(map);

//     mapRef.current = {
//       map,
//       markersLayer,
//       heatLayer,
//       hotspotsLayer
//     };

//     setTimeout(() => {
//       map.invalidateSize();
//     }, 0);

//     const droneIcon = L.divIcon({
//       html: '🚁',
//       className: 'custom-div-icon',
//       iconSize: [24, 24]
//     });

//     const fetchSpatialData = async () => {
//       try {
//         const res = await fetch('http://localhost:6081/api/v1/nodes/match?lat=13.04&lon=80.24&radius_km=50&class=16&tenant_id=alpha_logistics');
//         const json: { status: string; data: MatchResult[] } = await res.json();

//         if (json.status === "success") {
//           setActiveNodes(json.data.length);

//           markersLayer.clearLayers();
//           const heatData: [number, number, number][] = [];

//           json.data.forEach((node) => {
//             heatData.push([node.lat, node.lon, 1.0]);

//             L.marker([node.lat, node.lon], { icon: droneIcon })
//               .bindPopup(`<strong>${node.node_id}</strong><br>Distance: ${node.distance_km.toFixed(2)} km`)
//               .addTo(markersLayer);
//           });

//           if (heatData.length > 0) {
//             heatLayer.setLatLngs(heatData);
//           }
//         }
//       } catch (err) {
//         console.error("Engine fetch failed", err);
//       }
//     };

//     const fetchPredictedZones = async () => {
//       try {
//         const res = await fetch('http://localhost:6081/api/v1/zones/predicted');
//         const json: { status: string; data: ZonePrediction[] } = await res.json();

//         if (json.status === "success") {
//           hotspotsLayer.clearLayers();

//           json.data.forEach((zone) => {
//             L.circle([zone.Lat, zone.Lon], {
//               color: '#ef4444',
//               fillColor: '#ef4444',
//               fillOpacity: 0.2,
//               radius: zone.RadiusKm * 1000
//             })
//               .bindPopup(`<b>🤖 AI Prediction</b><br>Zone: ${zone.ID}`)
//               .addTo(hotspotsLayer);
//           });
//         }
//       } catch (err) {
//         console.error("Failed to fetch ML zones", err);
//       }
//     };

//     const spatialInterval = setInterval(fetchSpatialData, 1000);
//     const predictionInterval = setInterval(fetchPredictedZones, 10000);

//     fetchPredictedZones();

//     return () => {
//       clearInterval(spatialInterval);
//       clearInterval(predictionInterval);

//       map.remove();
//       mapRef.current = null;
//     };
//   }, []);


//   return (
//     <div className="relative w-full h-full">
//       <div className="absolute top-4 left-4 z-[1000] bg-slate-800/90 p-4 rounded-lg border border-slate-700 shadow-lg">
//         <div className="text-3xl font-bold text-emerald-500">{activeNodes}</div>
//         <div className="text-xs text-slate-400 uppercase tracking-widest">Active Nodes</div>
//       </div>
//       <div id="map-container" className="w-full h-full" />
//     </div>
//   );
// }

import { useEffect, useRef, useState } from 'react';
import L from 'leaflet';
// @ts-ignore
import 'leaflet.heat';
import 'leaflet/dist/leaflet.css';
import type { MatchResult, ZonePrediction } from '../types/polaris';

interface MapState {
  map: L.Map;
  markersLayer: L.LayerGroup;
  heatLayer: any;
  hotspotsLayer: L.LayerGroup;
}

export default function MapDashboard() {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const mapRef = useRef<MapState | null>(null);
  const [activeNodes, setActiveNodes] = useState<number>(0);

  useEffect(() => {
    // 🚫 Prevent double init + ensure container exists
    if (!containerRef.current || mapRef.current) return;

    const map = L.map(containerRef.current, {
      zoomControl: false
    }).setView([13.04, 80.24], 12);

    L.control.zoom({ position: 'topright' }).addTo(map);

    L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
      attribution: '&copy; CARTO',
      subdomains: 'abcd',
      maxZoom: 19
    }).addTo(map);

    const markersLayer = L.layerGroup().addTo(map);
    const hotspotsLayer = L.layerGroup().addTo(map);

    mapRef.current = {
      map,
      markersLayer,
      heatLayer: null,
      hotspotsLayer
    };

    // ✅ Initialize heatmap ONLY after map is ready
    map.whenReady(() => {
      // 🔥 Wait until container has real size
      const waitForSize = () => {
        const size = map.getSize();

        if (size.x === 0 || size.y === 0) {
          requestAnimationFrame(waitForSize);
          return;
        }

        // ✅ Now safe to create heatmap
        const heatLayer = (L as any).heatLayer([], {
          radius: 25,
          blur: 15,
          maxZoom: 14
        }).addTo(map);

        if (mapRef.current) {
          mapRef.current.heatLayer = heatLayer;
        }

        map.invalidateSize();
      };

      waitForSize();
    });


    const droneIcon = L.divIcon({
      html: '🚁',
      className: 'custom-div-icon',
      iconSize: [24, 24]
    });

    // 🚀 Fetch nodes
    const fetchSpatialData = async () => {
      try {
        const res = await fetch(
          'http://localhost:6081/api/v1/nodes/match?lat=13.04&lon=80.24&radius_km=50&class=16&tenant_id=alpha_logistics'
        );
        const json: { status: string; data: MatchResult[] } = await res.json();

        if (json.status === 'success' && mapRef.current) {
          const { markersLayer, heatLayer } = mapRef.current;

          setActiveNodes(json.data.length);
          markersLayer.clearLayers();

          const heatData: [number, number, number][] = [];

          json.data.forEach((node) => {
            heatData.push([node.lat, node.lon, 1.0]);

            L.marker([node.lat, node.lon], { icon: droneIcon })
              .bindPopup(
                `<strong>${node.node_id}</strong><br>Distance: ${node.distance_km.toFixed(2)} km`
              )
              .addTo(markersLayer);
          });

          // ✅ Guard heat layer usage
          if (heatLayer && heatData.length > 0) {
            heatLayer.setLatLngs(heatData);
          }
        }
      } catch (err) {
        console.error('Engine fetch failed', err);
      }
    };

    // 🤖 Fetch predicted zones
    const fetchPredictedZones = async () => {
      try {
        const res = await fetch('http://localhost:6081/api/v1/zones/predicted');
        const json: { status: string; data: ZonePrediction[] } = await res.json();

        if (json.status === 'success' && mapRef.current) {
          const { hotspotsLayer } = mapRef.current;

          hotspotsLayer.clearLayers();

          json.data.forEach((zone) => {
            L.circle([zone.Lat, zone.Lon], {
              color: '#ef4444',
              fillColor: '#ef4444',
              fillOpacity: 0.2,
              radius: zone.RadiusKm * 1000
            })
              .bindPopup(`<b>🤖 AI Prediction</b><br>Zone: ${zone.ID}`)
              .addTo(hotspotsLayer);
          });
        }
      } catch (err) {
        console.error('Failed to fetch ML zones', err);
      }
    };

    const spatialInterval = setInterval(fetchSpatialData, 1000);
    const predictionInterval = setInterval(fetchPredictedZones, 10000);

    fetchPredictedZones();

    // 🧹 Cleanup (critical)
    return () => {
      clearInterval(spatialInterval);
      clearInterval(predictionInterval);

      if (mapRef.current) {
        mapRef.current.map.remove();
        mapRef.current = null;
      }
    };
  }, []);

  return (
    <div className="relative w-full h-full">
      <div className="absolute top-4 left-4 z-[1000] bg-slate-800/90 p-4 rounded-lg border border-slate-700 shadow-lg">
        <div className="text-3xl font-bold text-emerald-500">{activeNodes}</div>
        <div className="text-xs text-slate-400 uppercase tracking-widest">
          Active Nodes
        </div>
      </div>

      {/* ✅ Use ref instead of id */}
      <div ref={containerRef} className="w-full h-full" />
    </div>
  );
}
