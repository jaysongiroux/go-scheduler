// test/webhook-server.ts
import express from "express";

export function startWebhookServer(port: number) {
  const app = express();
  app.use(express.json());

  let lastEvent: any = null;

  app.post("/webhook", (req, res) => {
    lastEvent = req.body;
    res.sendStatus(200);
  });

  const server = app.listen(port);

  return {
    getLastEvent: () => lastEvent,
    close: () => server.close(),
  };
}
