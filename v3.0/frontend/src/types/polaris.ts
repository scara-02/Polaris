export interface MatchResult {
  node_id: string;
  asset_class: number;
  lat: number;
  lon: number;
  distance_km: number;
  eta_seconds: number;
  route_type: string;
}

export interface ZonePrediction {
  ID: string;
  Lat: number;
  Lon: number;
  RadiusKm: number;
  RequiredAssets: number;
  TargetClass: number;
  TenantID: string;
}

export interface LogEntry {
  time: string;
  msg: string;
  type: 'info' | 'success' | 'warning' | 'danger';
}

export interface TelemetryPayload {
  tenant_id: string;
  node_id: string;
  asset_class: number;
  lat: number;
  lon: number;
  status: string;
  battery: number;
}