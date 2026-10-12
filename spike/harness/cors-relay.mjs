// Spike harness (#37): relays the reference client's browser requests to
// mm-mcp's Streamable HTTP endpoint without the Origin headers mm-mcp refuses,
// and answers CORS so the browser lets the reference client read the replies.
import http from "node:http";
const [listen, target] = [Number(process.argv[2] || 3001), Number(process.argv[3] || 3101)];
http.createServer((req, res) => {
  const cors = {
    "access-control-allow-origin": "*",
    "access-control-allow-headers": "*",
    "access-control-allow-methods": "GET, POST, DELETE, OPTIONS",
    "access-control-expose-headers": "mcp-session-id, mcp-protocol-version",
  };
  if (req.method === "OPTIONS") { res.writeHead(204, cors); res.end(); return; }
  const headers = { ...req.headers };
  for (const name of ["origin", "referer", "sec-fetch-site", "sec-fetch-mode", "sec-fetch-dest"]) delete headers[name];
  headers.host = "127.0.0.1:" + target;
  const upstream = http.request({ host: "127.0.0.1", port: target, method: req.method, path: req.url, headers }, (reply) => {
    res.writeHead(reply.statusCode, { ...reply.headers, ...cors });
    reply.pipe(res);
  });
  upstream.on("error", (e) => { res.writeHead(502, cors); res.end(String(e)); });
  req.pipe(upstream);
}).listen(listen, "127.0.0.1", () => console.log("relay " + listen + " -> " + target));
