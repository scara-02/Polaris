"""
generate_multinode_traffic.py
─────────────────────────────
Reads the single-node Traffic.csv and expands it into a 15-node dataset.

Each node represents a real Chennai zone, labelled by its lat/lon centroid.
Per-zone variation is created via:
  • A unique demand multiplier (commercial zones ~1.3×, suburban ~0.7×)
  • Random jitter ±15 % on each vehicle count
  • The Traffic Situation label is re-computed from the new Total

Output: checkpoints/Traffic_15nodes.csv
"""

import csv
import math
import os
import random

random.seed(42)

# ── 15 real Chennai zones with (lat, lon, name, demand_multiplier) ────
ZONES = [
    (13.0418, 80.2341, "T_Nagar",        1.30),
    (13.0850, 80.2101, "Anna_Nagar",      1.15),
    (13.0012, 80.2565, "Adyar",           1.05),
    (12.9610, 80.2425, "OMR_Thoraipakkam",1.20),
    (13.0067, 80.2206, "Velachery",       1.10),
    (13.0368, 80.2676, "Mylapore",        0.95),
    (13.0067, 80.2206, "Guindy",          1.25),
    (13.0569, 80.2425, "Nungambakkam",    1.00),
    (13.0732, 80.2609, "Egmore",          1.10),
    (13.1070, 80.2320, "Perambur",        0.80),
    (13.0500, 80.2600, "Royapettah",      0.90),
    (12.9830, 80.2594, "Thiruvanmiyur",   0.85),
    (13.0382, 80.1574, "Porur",           0.75),
    (12.9516, 80.1462, "Chromepet",       0.70),
    (12.9249, 80.1378, "Tambaram",        0.65),
]

# ── Traffic situation thresholds (same as original dataset convention) ──
def classify_traffic(total: int) -> str:
    if total <= 60:
        return "low"
    elif total <= 130:
        return "normal"
    elif total <= 170:
        return "high"
    else:
        return "heavy"


def jitter(value: float, pct: float = 0.15) -> int:
    """Apply random jitter of ±pct to value, floored at 0."""
    factor = 1.0 + random.uniform(-pct, pct)
    return max(0, round(value * factor))


def main():
    src_path = os.path.join(os.path.dirname(__file__), "checkpoints", "Traffic.csv")
    dst_path = os.path.join(os.path.dirname(__file__), "checkpoints", "Traffic_15nodes.csv")

    # Read source single-node data
    with open(src_path, "r", newline="", encoding="utf-8-sig") as f:
        reader = csv.DictReader(f)
        base_rows = list(reader)

    print(f"[gen] Read {len(base_rows)} rows from base Traffic.csv")

    # Write multi-node CSV
    fieldnames = [
        "zone_id", "zone_name", "lat", "lon",
        "Time", "Date", "Day of the week",
        "CarCount", "BikeCount", "BusCount", "TruckCount",
        "Total", "Traffic Situation",
    ]

    with open(dst_path, "w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=fieldnames)
        writer.writeheader()

        total_written = 0
        for zone_idx, (lat, lon, name, multiplier) in enumerate(ZONES):
            for row in base_rows:
                car   = jitter(int(row["CarCount"])   * multiplier)
                bike  = jitter(int(row["BikeCount"])  * multiplier)
                bus   = jitter(int(row["BusCount"])   * multiplier)
                truck = jitter(int(row["TruckCount"]) * multiplier)
                total = car + bike + bus + truck

                writer.writerow({
                    "zone_id":            zone_idx,
                    "zone_name":          name,
                    "lat":                f"{lat:.4f}",
                    "lon":                f"{lon:.4f}",
                    "Time":               row["Time"],
                    "Date":               row["Date"],
                    "Day of the week":    row["Day of the week"],
                    "CarCount":           car,
                    "BikeCount":          bike,
                    "BusCount":           bus,
                    "TruckCount":         truck,
                    "Total":              total,
                    "Traffic Situation":  classify_traffic(total),
                })
                total_written += 1

    expected = len(ZONES) * len(base_rows)
    print(f"[gen] Wrote {total_written} rows (expected {expected}) → {dst_path}")
    assert total_written == expected, "Row count mismatch!"
    print("[gen] ✅ Done")


if __name__ == "__main__":
    main()
