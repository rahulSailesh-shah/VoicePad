import { Hono } from "hono";
import { cors } from "hono/cors";
import { auth } from "./lib/auth"; // path to your auth file

const app = new Hono();
app.use(
  "*",
  cors({
    origin: [
      "http://localhost:5173",
      "http://127.0.0.1:5173",
      "http://localhost:9000",
      "http://127.0.0.1:9000",
    ],
    credentials: true,
    allowMethods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"],
    allowHeaders: ["Content-Type", "Authorization"],
    exposeHeaders: [],
    maxAge: 86400,
  })
);

app
  .on(["POST", "GET"], "/api/auth/*", (c) => auth.handler(c.req.raw))
  .get("/", (c) => {
    return c.text("Auth Service is running");
  });

export default app;
