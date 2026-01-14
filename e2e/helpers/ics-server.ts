import express from "express";

export interface ICSServerOptions {
  requireAuth?: boolean;
  username?: string;
  password?: string;
  bearerToken?: string;
  returnStatus?: number;
  etag?: string;
  lastModified?: string;
}

export function startICSServer(port: number, icsContent: string, options: ICSServerOptions = {}) {
  const app = express();
  
  let currentContent = icsContent;
  let requestCount = 0;

  app.get("/calendar.ics", (req, res) => {
    requestCount++;

    // Check authentication if required
    if (options.requireAuth) {
      const authHeader = req.headers.authorization;
      
      if (options.bearerToken) {
        if (!authHeader || authHeader !== `Bearer ${options.bearerToken}`) {
          res.status(401).send("Unauthorized");
          return;
        }
      } else if (options.username && options.password) {
        const expectedAuth = `Basic ${Buffer.from(`${options.username}:${options.password}`).toString('base64')}`;
        if (!authHeader || authHeader !== expectedAuth) {
          res.status(401).send("Unauthorized");
          return;
        }
      }
    }

    // Return custom status if specified
    if (options.returnStatus && options.returnStatus !== 200) {
      res.status(options.returnStatus).send("Error");
      return;
    }

    // Handle If-Modified-Since
    const ifModifiedSince = req.headers['if-modified-since'];
    if (ifModifiedSince && options.lastModified && ifModifiedSince === options.lastModified) {
      res.status(304).send();
      return;
    }

    // Set headers
    res.setHeader("Content-Type", "text/calendar; charset=utf-8");
    if (options.etag) {
      res.setHeader("ETag", options.etag);
    }
    if (options.lastModified) {
      res.setHeader("Last-Modified", options.lastModified);
    }

    res.send(currentContent);
  });

  const server = app.listen(port);

  return {
    getRequestCount: () => requestCount,
    updateContent: (newContent: string) => {
      currentContent = newContent;
    },
    close: () => new Promise<void>((resolve) => {
      server.close(() => resolve());
    }),
  };
}
