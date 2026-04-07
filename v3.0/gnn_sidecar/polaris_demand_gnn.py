"""
polaris_demand_gnn.py
─────────────────────
Spatio-Temporal Graph Neural Network for demand prediction in the Polaris platform.

Architecture
───────────
  Input  : zone graph with node features over T historical timesteps
  Spatial: GATConv (Graph Attention) – learns which neighbouring zones matter most
  Temporal: GRU  – captures demand trends over time
  Fusion : concat spatial + temporal embeddings, project to hidden dim
  Output : predicted demand per zone for the next H timesteps

Install
───────
  pip install torch torch_geometric

Usage
─────
  See __main__ block at the bottom for a quick synthetic demo.
"""
import os
import csv
import math
import redis
import torch
import torch.nn as nn
import torch.nn.functional as F
from torch_geometric.nn import GATConv
from torch_geometric.data import Data, Batch
import redis as redis_lib
import numpy as np
from flask import Flask, jsonify
from datetime import datetime
# connects to the same Redis your Go services use
_redis = redis_lib.Redis(host=os.environ.get("REDIS_HOST", "localhost"), port=6379, db=0)

NUM_ZONES     = 15   # 15 real Chennai zone nodes
NODE_FEATURES = 7    # Total, CarCount, BikeCount, BusCount, TruckCount, hour_sin, hour_cos
T_WINDOW      = 12

def _latest_window_stub():
    """
    Reads the last T_WINDOW * NUM_ZONES entries from the Polaris
    telemetry Redis Stream that the Spatial Engine already writes.
    Stream key: polaris:telemetry  (check your spatial-engine source)
    Each entry has fields: zone_id, demand, supply, lat, lon, hour_sin, hour_cos
    """
    entries = _redis.xrevrange("polaris:telemetry", count=T_WINDOW * NUM_ZONES)
    if not entries or len(entries) < NUM_ZONES:
        # not enough data yet — return zeros so server doesn't crash on startup
        return torch.zeros(NUM_ZONES, T_WINDOW, NODE_FEATURES)

    # reshape raw Redis entries → (NUM_ZONES, T_WINDOW, NODE_FEATURES) tensor
    # entries is newest-first, so reverse it
    entries = list(reversed(entries))
    zone_data = {}
    for _, fields in entries:
        zid = int(fields.get(b"zone_id", 0))
        row = [
            float(fields.get(b"demand",   0)),
            float(fields.get(b"supply",   0)),
            float(fields.get(b"hour_sin", 0)),
            float(fields.get(b"hour_cos", 1)),
            float(fields.get(b"day",      0)),
            float(fields.get(b"lat",      0)),
            float(fields.get(b"lon",      0)),
        ]
        zone_data.setdefault(zid, []).append(row)

    tensor = torch.zeros(NUM_ZONES, T_WINDOW, NODE_FEATURES)
    for zid, rows in zone_data.items():
        if zid < NUM_ZONES:
            rows = rows[-T_WINDOW:]  # keep most recent T_WINDOW
            tensor[zid, :len(rows), :] = torch.tensor(rows, dtype=torch.float)
    return tensor


def load_traffic_csv(path: str, num_zones: int = NUM_ZONES,
                     num_features: int = NODE_FEATURES) -> torch.Tensor:
    """
    Load the multi-node Traffic CSV into a tensor.

    The CSV has columns:
      zone_id, zone_name, lat, lon, Time, Date, Day of the week,
      CarCount, BikeCount, BusCount, TruckCount, Total, Traffic Situation

    We map to 7 node features:
      [Total, CarCount, BikeCount, BusCount, TruckCount, hour_sin, hour_cos]

    Returns
    ───────
    (num_zones, total_timesteps, num_features) tensor
    Also returns zone_coords list of (lat, lon) tuples.
    """
    # Read all rows grouped by zone_id
    zone_rows: dict[int, list] = {}
    zone_coords: dict[int, tuple[float, float]] = {}

    with open(path, "r", newline="", encoding="utf-8-sig") as f:
        reader = csv.DictReader(f)
        for row in reader:
            zid = int(row["zone_id"])
            if zid >= num_zones:
                continue

            # Parse time for cyclical encoding
            time_str = row["Time"].strip()
            try:
                t = datetime.strptime(time_str, "%I:%M:%S %p")
            except ValueError:
                t = datetime.strptime(time_str, "%H:%M:%S")
            hour_frac = t.hour + t.minute / 60.0
            hour_sin = math.sin(2 * math.pi * hour_frac / 24.0)
            hour_cos = math.cos(2 * math.pi * hour_frac / 24.0)

            features = [
                float(row["Total"]),
                float(row["CarCount"]),
                float(row["BikeCount"]),
                float(row["BusCount"]),
                float(row["TruckCount"]),
                hour_sin,
                hour_cos,
            ]
            zone_rows.setdefault(zid, []).append(features)
            if zid not in zone_coords:
                zone_coords[zid] = (float(row["lat"]), float(row["lon"]))

    # Find the minimum timestep count across zones
    min_steps = min(len(rows) for rows in zone_rows.values())
    print(f"[CSV] Loaded {len(zone_rows)} zones, {min_steps} timesteps each")

    tensor = torch.zeros(num_zones, min_steps, num_features)
    for zid, rows in zone_rows.items():
        rows = rows[:min_steps]  # truncate to uniform length
        tensor[zid] = torch.tensor(rows, dtype=torch.float)

    coords = [(zone_coords.get(i, (0.0, 0.0))) for i in range(num_zones)]
    return tensor, coords


# ──────────────────────────────────────────────
# 1.  SPATIAL ENCODER  (Graph Attention Network)
# ──────────────────────────────────────────────

class SpatialEncoder(nn.Module):
    """
    Two-layer Graph Attention Network.

    Each zone attends to its neighbours and learns how much weight to give
    each one.  For example, a residential zone at 8 AM should attend heavily
    to nearby commercial zones that are about to see a demand spike.

    Args
    ────
    in_channels  : number of node features  (e.g. 7: demand, supply, hour …)
    hidden_dim   : output embedding size per zone
    heads        : number of independent attention heads (multi-head attention)
    dropout      : dropout probability applied to attention coefficients
    """

    def __init__(self, in_channels: int, hidden_dim: int, heads: int = 4, dropout: float = 0.1):
        super().__init__()

        # Layer 1 – projects raw features into heads × (hidden_dim // heads)
        self.gat1 = GATConv(
            in_channels=in_channels,
            out_channels=hidden_dim // heads,
            heads=heads,
            dropout=dropout,
            concat=True,        # concatenate head outputs → hidden_dim
        )

        # Layer 2 – refines embeddings; output is hidden_dim (single head)
        self.gat2 = GATConv(
            in_channels=hidden_dim,
            out_channels=hidden_dim,
            heads=1,
            dropout=dropout,
            concat=False,       # average head outputs → hidden_dim
        )

        self.norm1 = nn.LayerNorm(hidden_dim)
        self.norm2 = nn.LayerNorm(hidden_dim)
        self.dropout = nn.Dropout(dropout)

    def forward(self, x: torch.Tensor, edge_index: torch.Tensor,
                edge_attr: torch.Tensor | None = None) -> torch.Tensor:
        """
        Args
        ────
        x          : (num_zones, in_channels)  – node feature matrix
        edge_index : (2, num_edges)            – COO edge list
        edge_attr  : (num_edges, edge_features) or None

        Returns
        ───────
        (num_zones, hidden_dim) spatial embedding per zone
        """
        # Layer 1 + residual-style norm
        h = self.gat1(x, edge_index, edge_attr)
        h = self.norm1(F.elu(h))
        h = self.dropout(h)

        # Layer 2 + residual-style norm
        h = self.gat2(h, edge_index, edge_attr)
        h = self.norm2(F.elu(h))

        return h   # (num_zones, hidden_dim)


# ──────────────────────────────────────────────
# 2.  TEMPORAL ENCODER  (GRU over time)
# ──────────────────────────────────────────────

class TemporalEncoder(nn.Module):
    """
    GRU that processes the historical demand sequence for every zone in parallel.

    The GRU sees T consecutive snapshots of zone features and produces a
    single summary vector that captures recent trends (rising demand, daily
    periodicity, etc.).

    Args
    ────
    in_channels : same as SpatialEncoder.in_channels
    hidden_dim  : GRU hidden state size
    num_layers  : GRU depth
    dropout     : inter-layer dropout (only applies when num_layers > 1)
    """

    def __init__(self, in_channels: int, hidden_dim: int,
                 num_layers: int = 2, dropout: float = 0.1):
        super().__init__()
        self.gru = nn.GRU(
            input_size=in_channels,
            hidden_size=hidden_dim,
            num_layers=num_layers,
            batch_first=True,   # input shape: (batch, T, features)
            dropout=dropout if num_layers > 1 else 0.0,
        )
        self.norm = nn.LayerNorm(hidden_dim)

    def forward(self, x_seq: torch.Tensor) -> torch.Tensor:
        """
        Args
        ────
        x_seq : (num_zones, T, in_channels)  – feature sequence per zone

        Returns
        ───────
        (num_zones, hidden_dim) – last GRU hidden state per zone
        """
        _, h_n = self.gru(x_seq)   # h_n: (num_layers, num_zones, hidden_dim)
        out = h_n[-1]              # take the last layer's hidden state
        return self.norm(out)      # (num_zones, hidden_dim)


# ──────────────────────────────────────────────
# 3.  FULL SPATIO-TEMPORAL MODEL
# ──────────────────────────────────────────────

class PolarisDemandGNN(nn.Module):
    """
    Spatio-Temporal GNN for zone-level demand prediction.

    Combines:
      • SpatialEncoder  – what are my neighbours doing RIGHT NOW?
      • TemporalEncoder – what has been happening OVER TIME in each zone?
      • Fusion + MLP    – project joint embedding to H-step demand forecast

    Args
    ────
    node_features    : number of features per zone per timestep
    hidden_dim       : shared embedding size (must be divisible by gat_heads)
    gat_heads        : attention heads in the spatial encoder
    gru_layers       : GRU depth in the temporal encoder
    horizon          : number of future timesteps to predict
    dropout          : global dropout rate
    """

    def __init__(
        self,
        node_features: int = 7,
        hidden_dim: int = 64,
        gat_heads: int = 4,
        gru_layers: int = 2,
        horizon: int = 6,
        dropout: float = 0.1,
    ):
        super().__init__()
        self.horizon = horizon

        # Spatial branch: processes the LATEST snapshot of the graph
        self.spatial_enc = SpatialEncoder(
            in_channels=node_features,
            hidden_dim=hidden_dim,
            heads=gat_heads,
            dropout=dropout,
        )

        # Temporal branch: processes the full T-step history per zone
        self.temporal_enc = TemporalEncoder(
            in_channels=node_features,
            hidden_dim=hidden_dim,
            num_layers=gru_layers,
            dropout=dropout,
        )

        # Fusion: concat [spatial ‖ temporal] → hidden_dim
        self.fusion = nn.Sequential(
            nn.Linear(hidden_dim * 2, hidden_dim),
            nn.LayerNorm(hidden_dim),
            nn.ELU(),
            nn.Dropout(dropout),
        )

        # Output head: hidden_dim → predicted demand for H future steps
        self.head = nn.Sequential(
            nn.Linear(hidden_dim, hidden_dim // 2),
            nn.ELU(),
            nn.Linear(hidden_dim // 2, horizon),
        )

    def forward(
        self,
        x_seq: torch.Tensor,
        edge_index: torch.Tensor,
        edge_attr: torch.Tensor | None = None,
    ) -> torch.Tensor:
        """
        Args
        ────
        x_seq      : (num_zones, T, node_features)
                     Historical feature sequences for every zone.

        edge_index : (2, num_edges)
                     Zone adjacency in COO format (built once from the QuadTree).

        edge_attr  : (num_edges, edge_features) or None
                     Optional edge weights, e.g. inverse distance between zones.

        Returns
        ───────
        pred : (num_zones, horizon)
               Predicted demand for each zone for the next H timesteps.
        """
        # Extract the most recent snapshot for spatial encoding
        x_now = x_seq[:, -1, :]   # (num_zones, node_features)

        # Spatial branch – attends to the current neighbourhood state
        spatial_emb = self.spatial_enc(x_now, edge_index, edge_attr)

        # Temporal branch – summarises historical trends
        temporal_emb = self.temporal_enc(x_seq)

        # Fuse and predict
        combined = torch.cat([spatial_emb, temporal_emb], dim=-1)
        fused = self.fusion(combined)
        pred = self.head(fused)    # (num_zones, horizon)

        return pred


# ──────────────────────────────────────────────
# 4.  ZONE GRAPH BUILDER
#     Converts Polaris zone metadata into a PyG graph
# ──────────────────────────────────────────────

def build_zone_graph(
    zone_coords: list[tuple[float, float]],
    radius_km: float = 2.0,
) -> tuple[torch.Tensor, torch.Tensor]:
    """
    Build a zone adjacency graph from lat/lon coordinates.

    Two zones are connected if their Euclidean distance (approximate,
    using 1 deg ≈ 111 km) is within radius_km.

    In production, replace this with actual QuadTree neighbour queries
    from the Polaris spatial engine.

    Args
    ────
    zone_coords : list of (lat, lon) tuples for each zone centroid
    radius_km   : connect zones within this distance

    Returns
    ───────
    edge_index : (2, num_edges) LongTensor
    edge_attr  : (num_edges, 1) FloatTensor – normalised inverse distance
    """
    import math

    n = len(zone_coords)
    src_list, dst_list, weight_list = [], [], []
    threshold = radius_km / 111.0   # rough degrees

    for i in range(n):
        for j in range(n):
            if i == j:
                continue
            dlat = zone_coords[i][0] - zone_coords[j][0]
            dlon = zone_coords[i][1] - zone_coords[j][1]
            dist = math.sqrt(dlat ** 2 + dlon ** 2)
            if dist < threshold:
                src_list.append(i)
                dst_list.append(j)
                weight_list.append(1.0 / (dist + 1e-6))

    edge_index = torch.tensor([src_list, dst_list], dtype=torch.long)

    # Normalise weights to [0, 1]
    weights = torch.tensor(weight_list, dtype=torch.float).unsqueeze(1)
    if weights.numel() > 0:
        weights = weights / weights.max()

    return edge_index, weights


# ──────────────────────────────────────────────
# 5.  TRAINING UTILITIES
# ──────────────────────────────────────────────

class DemandDataset(torch.utils.data.Dataset):
    """
    Sliding-window dataset over zone telemetry history.

    Each sample is:
      x  : (num_zones, T, node_features)  – historical window
      y  : (num_zones, horizon)           – future demand labels

    Args
    ────
    telemetry    : (num_zones, total_timesteps, node_features) tensor
    window       : number of historical steps fed to the model (T)
    horizon      : number of future steps to predict (H)
    demand_idx   : index of the demand feature in the last dimension
    """

    def __init__(
        self,
        telemetry: torch.Tensor,
        window: int = 12,
        horizon: int = 6,
        demand_idx: int = 0,
    ):
        self.telemetry = telemetry
        self.window = window
        self.horizon = horizon
        self.demand_idx = demand_idx
        self.length = telemetry.shape[1] - window - horizon + 1

    def __len__(self) -> int:
        return self.length

    def __getitem__(self, idx: int) -> tuple[torch.Tensor, torch.Tensor]:
        x = self.telemetry[:, idx : idx + self.window, :]
        y = self.telemetry[:, idx + self.window : idx + self.window + self.horizon, self.demand_idx]
        return x, y


def train_one_epoch(
    model: PolarisDemandGNN,
    loader: torch.utils.data.DataLoader,
    edge_index: torch.Tensor,
    edge_attr: torch.Tensor,
    optimiser: torch.optim.Optimizer,
    device: torch.device,
) -> float:
    """Run one full training epoch. Returns mean MAE loss."""
    model.train()
    total_loss = 0.0

    for x_batch, y_batch in loader:
        # x_batch : (batch, num_zones, T, features)
        # y_batch : (batch, num_zones, horizon)
        x_batch = x_batch.to(device)
        y_batch = y_batch.to(device)
        ei = edge_index.to(device)
        ea = edge_attr.to(device) if edge_attr is not None else None

        optimiser.zero_grad()

        # Process each sample in the batch independently
        # (PyG graphs share the same topology across the batch)
        preds = []
        for i in range(x_batch.size(0)):
            pred = model(x_batch[i], ei, ea)   # (num_zones, horizon)
            preds.append(pred)

        pred_batch = torch.stack(preds)          # (batch, num_zones, horizon)
        loss = F.l1_loss(pred_batch, y_batch)    # MAE
        loss.backward()
        torch.nn.utils.clip_grad_norm_(model.parameters(), max_norm=1.0)
        optimiser.step()
        total_loss += loss.item()

    return total_loss / len(loader)


@torch.no_grad()
def evaluate(
    model: PolarisDemandGNN,
    loader: torch.utils.data.DataLoader,
    edge_index: torch.Tensor,
    edge_attr: torch.Tensor,
    device: torch.device,
) -> dict[str, float]:
    """Return MAE and RMSE on the given loader."""
    model.eval()
    all_preds, all_targets = [], []

    for x_batch, y_batch in loader:
        x_batch = x_batch.to(device)
        ei = edge_index.to(device)
        ea = edge_attr.to(device) if edge_attr is not None else None

        for i in range(x_batch.size(0)):
            pred = model(x_batch[i], ei, ea)
            all_preds.append(pred.cpu())
            all_targets.append(y_batch[i])

    preds = torch.stack(all_preds)
    targets = torch.stack(all_targets)
    mae = F.l1_loss(preds, targets).item()
    rmse = torch.sqrt(F.mse_loss(preds, targets)).item()
    return {"mae": mae, "rmse": rmse}


# ──────────────────────────────────────────────
# 6.  POLARIS INFERENCE HOOK
#     Call this from the Go backend via a Python sidecar
# ──────────────────────────────────────────────

class PolarisPredictor:
    """
    Thin wrapper around a trained PolarisDemandGNN for serving predictions.

    The Go Coordination Engine calls predict() after each telemetry batch
    to get the demand forecast, which then feeds:
      • the Rebalancing Engine  (4.1)
      • the Spatial Alerting System (3.3)
      • the live heatmap (3.1)

    Args
    ────
    model_path   : path to a saved model checkpoint
    edge_index   : pre-built zone adjacency (torch.Tensor)
    edge_attr    : edge weights (torch.Tensor)
    device       : 'cuda' or 'cpu'
    """

    def __init__(
        self,
        model: PolarisDemandGNN,
        edge_index: torch.Tensor,
        edge_attr: torch.Tensor | None = None,
        device: str = "cpu",
    ):
        self.device = torch.device(device)
        self.model = model.to(self.device).eval()
        self.edge_index = edge_index.to(self.device)
        self.edge_attr = edge_attr.to(self.device) if edge_attr is not None else None

    @classmethod
    def from_checkpoint(
        cls,
        checkpoint_path: str,
        model_kwargs: dict,
        edge_index: torch.Tensor,
        edge_attr: torch.Tensor | None = None,
        device: str = "cpu",
    ) -> "PolarisPredictor":
        model = PolarisDemandGNN(**model_kwargs)
        state = torch.load(checkpoint_path, map_location=device)
        model.load_state_dict(state)
        return cls(model, edge_index, edge_attr, device)

    @torch.no_grad()
    def predict(self, x_seq: torch.Tensor) -> dict:
        """
        Args
        ────
        x_seq : (num_zones, T, node_features) – latest telemetry window

        Returns
        ───────
        dict with keys:
          'demand'     : (num_zones, horizon) numpy array – predicted demand
          'hot_zones'  : list of zone indices predicted to be busiest
          'rebalance'  : list of (from_zone, to_zone) suggestions
        """
        x_seq = x_seq.to(self.device)
        pred = self.model(x_seq, self.edge_index, self.edge_attr)
        pred_np = pred.cpu().numpy()

        # Average demand over the forecast horizon for ranking
        avg_demand = pred_np.mean(axis=1)
        sorted_zones = avg_demand.argsort()[::-1].tolist()
        hot_zones = sorted_zones[:5]

        # Simple rebalancing: suggest moving from bottom 5 to top 5 zones
        cold_zones = sorted_zones[-5:]
        rebalance = list(zip(cold_zones, hot_zones))

        return {
            "demand": pred_np,
            "hot_zones": hot_zones,
            "rebalance": rebalance,
        }


# ──────────────────────────────────────────────
# 7.  FLASK API SERVER  (served by Docker via --serve flag)
# ──────────────────────────────────────────────
import sys

# ── Load real traffic data from CSV ───────────────────────────────────
_CSV_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "checkpoints", "Traffic_15nodes.csv")

if os.path.exists(_CSV_PATH):
    _traffic_tensor, _SERVE_ZONE_COORDS = load_traffic_csv(_CSV_PATH, num_zones=NUM_ZONES)
    print(f"[GNN] Loaded traffic data: {_traffic_tensor.shape}")
else:
    print(f"[GNN] WARNING – no CSV at {_CSV_PATH}, using zero tensor")
    _traffic_tensor = torch.zeros(NUM_ZONES, 96, NODE_FEATURES)  # ~1 day fallback
    _SERVE_ZONE_COORDS = [
        (13.0418, 80.2341), (13.0850, 80.2101), (13.0012, 80.2565),
        (12.9610, 80.2425), (13.0067, 80.2206), (13.0368, 80.2676),
        (13.0067, 80.2206), (13.0569, 80.2425), (13.0732, 80.2609),
        (13.1070, 80.2320), (13.0500, 80.2600), (12.9830, 80.2594),
        (13.0382, 80.1574), (12.9516, 80.1462), (12.9249, 80.1378),
    ]

_serve_edge_index, _serve_edge_attr = build_zone_graph(_SERVE_ZONE_COORDS, radius_km=3.0)

# ── Model – initialised at module level so Flask routes can use it ─────
_serve_model = PolarisDemandGNN(
    node_features=NODE_FEATURES,
    hidden_dim=64,
    horizon=6,
)

# Checkpoint path
_CKPT_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "checkpoints", "demand_gnn_best.pt")
if os.path.exists(_CKPT_PATH):
    _serve_model.load_state_dict(torch.load(_CKPT_PATH, map_location="cpu"))
    print(f"[GNN] Loaded checkpoint: {_CKPT_PATH}")
else:
    print(f"[GNN] WARNING – no checkpoint at {_CKPT_PATH}, using random weights")

predictor = PolarisPredictor(_serve_model, _serve_edge_index, _serve_edge_attr, device="cpu")

# ── Flask app ─────────────────────────────────────────────────────────
from flask_cors import CORS
app = Flask(__name__)
CORS(app)


@app.route("/health", methods=["GET"])
def health():
    return jsonify({"model": "PolarisDemandGNN", "status": "ok", "zones": NUM_ZONES})


@app.route("/demand/forecast", methods=["GET"])
def forecast():
    now = datetime.now()
    total_steps = _traffic_tensor.shape[1]

    # Map current time to an index in the traffic data
    # Each row is a 15-min interval; 96 intervals per day
    intervals_per_day = 96
    current_interval = (now.hour * 4) + (now.minute // 15)
    # Use day-of-week offset to pick from the multi-day dataset
    day_offset = (now.weekday() * intervals_per_day) % total_steps
    start_idx = (day_offset + current_interval) % (total_steps - T_WINDOW)

    # Extract the real traffic window from CSV data
    traffic_input = _traffic_tensor[:, start_idx : start_idx + T_WINDOW, :]

    try:
        result_dict = predictor.predict(traffic_input)
        hot_zones   = result_dict["hot_zones"]

        # Build rebalance suggestions from cold → hot zones
        rebalance_data = [
            {
                "from_zone": result_dict["rebalance"][i][0] if i < len(result_dict["rebalance"]) else 0,
                "to_zone":   z,
                "priority":  "HIGH" if i < 2 else "NORMAL",
            }
            for i, z in enumerate(hot_zones)
        ]

        # Include zone coordinates in the response for the frontend map
        zone_info = [
            {"zone_id": i, "lat": _SERVE_ZONE_COORDS[i][0], "lon": _SERVE_ZONE_COORDS[i][1]}
            for i in range(NUM_ZONES)
        ]

        print(f"✅ GNN | {now.strftime('%H:%M:%S')} | window=[{start_idx}:{start_idx+T_WINDOW}] | hot_zones={hot_zones}")
        return jsonify({
            "status": "success",
            "data":   {
                "hot_zones":  hot_zones,
                "rebalance":  rebalance_data,
                "zones":      zone_info,
            },
        })

    except Exception as exc:
        print(f"❌ GNN error: {exc}")
        return jsonify({"status": "error", "message": str(exc)}), 500


# ──────────────────────────────────────────────
# 8.  ENTRYPOINT
# ──────────────────────────────────────────────

if __name__ == "__main__":
    print("Starting Polaris GNN API server on :5050 ...")
    app.run(host="0.0.0.0", port=5050, debug=False)