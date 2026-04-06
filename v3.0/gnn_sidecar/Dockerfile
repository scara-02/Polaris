# gnn_sidecar/Dockerfile
FROM python:3.11-slim

WORKDIR /app

# Install PyTorch CPU wheel first (smaller image, fine for inference)
RUN pip install --no-cache-dir \
    torch==2.3.0 --index-url https://download.pytorch.org/whl/cpu

# Install PyG and the rest
RUN pip install --no-cache-dir \
    torch_geometric==2.5.3 \
    flask==3.0.3 flask-cors==4.0.1 \
    redis==5.0.7 requests==2.32.3

COPY polaris_demand_gnn.py .
COPY checkpoints/ ./checkpoints/

EXPOSE 5050
CMD ["python", "polaris_demand_gnn.py", "--serve"]