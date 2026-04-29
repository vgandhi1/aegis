import { useEffect, useState } from "react";

type Status = { service: string; work_orders: number };
type WorkOrder = {
  id: number;
  station_id: string;
  vin: string;
  firmware_version: string;
  status: string;
  created_at: string;
};

function fmtTime(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

export default function App() {
  const [status, setStatus] = useState<Status | null>(null);
  const [orders, setOrders] = useState<WorkOrder[] | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const [s, o] = await Promise.all([
          fetch("/api/v1/status").then((r) => r.json()),
          fetch("/api/v1/work-orders").then((r) => r.json()),
        ]);
        if (!cancelled) {
          setStatus(s);
          setOrders(o);
          setErr(null);
          setLastUpdated(new Date());
        }
      } catch (e) {
        if (!cancelled) setErr(String(e));
      }
    }
    load();
    const id = setInterval(load, 10_000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, []);

  return (
    <div style={{ maxWidth: 960, margin: "0 auto", padding: "2rem" }}>
      <header style={{ marginBottom: "2rem" }}>
        <div style={{ display: "flex", alignItems: "baseline", gap: "1rem" }}>
          <h1 style={{ margin: 0, fontWeight: 600, letterSpacing: "-0.02em" }}>
            Aegis
          </h1>
          {lastUpdated && (
            <span style={{ fontSize: "0.75rem", color: "#5a6a7e" }}>
              updated {lastUpdated.toLocaleTimeString()} · refreshes every 10s
            </span>
          )}
        </div>
        <p style={{ margin: "0.5rem 0 0", color: "#9aa7b8" }}>
          Unified manufacturing correlation — MES snapshot.
        </p>
      </header>

      {err && (
        <p style={{ color: "#ff8b7a" }}>
          Could not reach MES API. Start infra + mes-service and refresh. ({err})
        </p>
      )}

      <section style={{ marginBottom: "2rem" }}>
        <h2 style={{ fontSize: "1rem", color: "#9aa7b8" }}>MES status</h2>
        {status ? (
          <pre
            style={{
              background: "#141b24",
              padding: "1rem",
              borderRadius: 8,
              overflow: "auto",
            }}
          >
            {JSON.stringify(status, null, 2)}
          </pre>
        ) : (
          !err && <p>Loading…</p>
        )}
      </section>

      <section>
        <h2 style={{ fontSize: "1rem", color: "#9aa7b8" }}>Recent work orders</h2>
        {orders && orders.length === 0 && <p>No rows yet.</p>}
        {orders && orders.length > 0 && (
          <table
            style={{
              width: "100%",
              borderCollapse: "collapse",
              fontSize: "0.9rem",
            }}
          >
            <thead>
              <tr style={{ textAlign: "left", color: "#9aa7b8" }}>
                <th style={{ padding: "0.5rem 0" }}>VIN</th>
                <th>Station</th>
                <th>Firmware</th>
                <th>Status</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {orders.map((w) => (
                <tr key={w.id} style={{ borderTop: "1px solid #1e2733" }}>
                  <td style={{ padding: "0.5rem 0", fontFamily: "monospace" }}>{w.vin}</td>
                  <td>{w.station_id}</td>
                  <td>{w.firmware_version}</td>
                  <td>{w.status}</td>
                  <td style={{ color: "#5a6a7e", whiteSpace: "nowrap" }}>
                    {fmtTime(w.created_at)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </div>
  );
}
