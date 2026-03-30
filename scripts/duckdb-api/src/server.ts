import duckdb from "duckdb";

const DB_PATH = process.env.DUCKDB_PATH || "/data/claude_hooks.db";
const PORT = parseInt(process.env.PORT || "9999", 10);

// Open DuckDB in read-only mode (host writes, API reads only)
let db: duckdb.Database;
try {
  db = new duckdb.Database(DB_PATH, { access_mode: "READ_ONLY" });
  console.log(`Connected to DuckDB at ${DB_PATH} (read-only)`);
} catch (err) {
  console.error(`Failed to open DuckDB at ${DB_PATH}:`, err);
  console.log("Starting with no database connection. /health will report unhealthy.");
}

function executeQuery(sql: string): Promise<Record<string, unknown>[]> {
  return new Promise((resolve, reject) => {
    if (!db) {
      reject(new Error("Database not connected"));
      return;
    }
    db.all(sql, (err: Error | null, rows: Record<string, unknown>[]) => {
      if (err) {
        reject(err);
      } else {
        resolve(rows || []);
      }
    });
  });
}

function corsHeaders(): Record<string, string> {
  return {
    "Access-Control-Allow-Origin": "*",
    "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
    "Access-Control-Allow-Headers": "Content-Type",
    "Content-Type": "application/json",
  };
}

const server = Bun.serve({
  port: PORT,
  async fetch(req: Request): Promise<Response> {
    const url = new URL(req.url);
    const headers = corsHeaders();

    // CORS preflight
    if (req.method === "OPTIONS") {
      return new Response(null, { status: 204, headers });
    }

    // Health check
    if (url.pathname === "/health") {
      try {
        if (db) {
          await executeQuery("SELECT 1 AS ok");
          return new Response(
            JSON.stringify({ status: "ok", database: DB_PATH }),
            { headers }
          );
        }
        return new Response(
          JSON.stringify({ status: "no_database", database: DB_PATH }),
          { status: 503, headers }
        );
      } catch (err) {
        return new Response(
          JSON.stringify({
            status: "error",
            error: err instanceof Error ? err.message : String(err),
          }),
          { status: 503, headers }
        );
      }
    }

    // SQL query endpoint
    if (url.pathname === "/query") {
      let sql = "";

      if (req.method === "GET") {
        sql = url.searchParams.get("sql") || "";
      } else if (req.method === "POST") {
        try {
          const body = await req.json();
          sql = body.sql || "";
        } catch {
          return new Response(
            JSON.stringify({ error: "Invalid JSON body" }),
            { status: 400, headers }
          );
        }
      } else {
        return new Response(
          JSON.stringify({ error: "Method not allowed" }),
          { status: 405, headers }
        );
      }

      if (!sql.trim()) {
        return new Response(
          JSON.stringify({ error: "No SQL query provided. Use ?sql=SELECT..." }),
          { status: 400, headers }
        );
      }

      // Security: block write operations (DB is read-only but belt-and-suspenders)
      const sqlUpper = sql.trim().toUpperCase();
      const blocked = ["INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "CREATE", "TRUNCATE"];
      if (blocked.some((keyword) => sqlUpper.startsWith(keyword))) {
        return new Response(
          JSON.stringify({ error: "Write operations are not allowed" }),
          { status: 403, headers }
        );
      }

      try {
        const rows = await executeQuery(sql);
        return new Response(JSON.stringify(rows), { headers });
      } catch (err) {
        return new Response(
          JSON.stringify({
            error: err instanceof Error ? err.message : String(err),
          }),
          { status: 500, headers }
        );
      }
    }

    // 404 for everything else
    return new Response(
      JSON.stringify({
        error: "Not found",
        endpoints: {
          "/health": "Health check",
          "/query?sql=SELECT...": "Execute SQL query (GET or POST)",
        },
      }),
      { status: 404, headers }
    );
  },
});

console.log(`DuckDB API server running on http://localhost:${server.port}`);
