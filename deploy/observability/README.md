# ONR Observability (Promtail + Loki + Grafana)

## Start

```bash
cd deploy/observability
docker compose up -d
```

## Access

- Grafana: http://localhost:3001 (admin/admin)
- Loki API: http://localhost:3100

## Auto Provisioning

- Grafana datasource `Loki` (uid: `loki`) is auto-created at startup.
- Dashboard `ONR Access Log Overview` is auto-imported to folder `ONR` at startup.
- No manual datasource/dashboard import is required.

## Stop

```bash
cd deploy/observability
docker compose down
```

## Notes

- This setup reads access logs from `logs/access.log`.
- Keep label cardinality low; avoid adding high-cardinality fields as labels.
